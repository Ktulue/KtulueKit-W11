package installer

import (
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

// Browser registry policy paths for force-installing extensions.
// Chrome/Brave use ExtensionInstallForcelist with "<id>;<update_url>" values.
// Firefox uses Extensions\Install with XPI URL values (AMO extension IDs, not Chrome IDs).
var browserPolicyPaths = map[string]string{
	"brave":   `Software\Policies\BraveSoftware\Brave-Browser\ExtensionInstallForcelist`,
	"chrome":  `Software\Policies\Google\Chrome\ExtensionInstallForcelist`,
	"firefox": `Software\Policies\Mozilla\Firefox\Extensions\Install`,
}

// Web Store / AMO base URLs for url mode.
const (
	chromeStoreURL  = "https://chromewebstore.google.com/detail/"
	firefoxAMOURL   = "https://addons.mozilla.org/firefox/addon/"
)


// InstallExtension handles a Tier 3 browser extension.
func InstallExtension(ext config.Extension, dryRun bool) reporter.Result {
	res := reporter.Result{
		ID:   ext.ID,
		Name: ext.Name,
		Tier: "extension",
	}

	if ext.Notes != "" && dryRun {
		fmt.Printf("    note: %s\n", ext.Notes)
	}

	switch ext.Mode {
	case "force":
		return forceInstall(ext, dryRun, res)
	default: // "url"
		return urlInstall(ext, dryRun, res)
	}
}

// forceInstall writes the extension ID to the browser's enterprise policy registry key.
func forceInstall(ext config.Extension, dryRun bool, res reporter.Result) reporter.Result {
	path, ok := browserPolicyPaths[ext.Browser]
	if !ok {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("unsupported browser for force mode: %s", ext.Browser)
		return res
	}

	value := forceValue(ext.Browser, ext.ExtensionID)

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("HKLM\\%s  =>  %s", path, value)
		return res
	}

	// Check if already set
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		vals, _ := k.ReadValueNames(-1)
		for _, v := range vals {
			existing, _, _ := k.GetStringValue(v)
			if strings.HasPrefix(existing, ext.ExtensionID) {
				res.Status = reporter.StatusAlready
				res.Detail = "registry key already present"
				return res
			}
		}
		k.Close()
	}

	// Create or open the key and find the next available numeric index
	k, _, err = registry.CreateKey(registry.LOCAL_MACHINE, path, registry.ALL_ACCESS)
	if err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("cannot open registry key: %s", err)
		return res
	}
	defer k.Close()

	// Find next available index (1, 2, 3 ...)
	index := 1
	vals, _ := k.ReadValueNames(-1)
	for _, v := range vals {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n >= index {
			index = n + 1
		}
	}

	if err := k.SetStringValue(fmt.Sprintf("%d", index), value); err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("cannot write registry value: %s", err)
		return res
	}

	res.Status = reporter.StatusInstalled
	res.Detail = "registry policy written — browser restart required"
	return res
}

// forceValue returns the registry value string for the given browser and extension ID.
// Chrome/Brave use "<id>;<crx_update_url>". Firefox uses an AMO XPI URL.
func forceValue(browser, extensionID string) string {
	if browser == "firefox" {
		return fmt.Sprintf("https://addons.mozilla.org/firefox/downloads/latest/%s/addon-latest.xpi", extensionID)
	}
	return fmt.Sprintf("%s;https://clients2.google.com/service/update2/crx", extensionID)
}

// storeURL returns the browser-appropriate extension listing URL for url mode.
func storeURL(browser, extensionID string) string {
	if browser == "firefox" {
		return firefoxAMOURL + extensionID
	}
	return chromeStoreURL + extensionID
}

// storeLabel returns the human-readable "click to install" label for url mode.
func storeLabel(browser string) string {
	if browser == "firefox" {
		return "Firefox Add-ons page opened — click 'Add to Firefox' to complete"
	}
	return "Chrome Web Store page opened — click 'Add to Brave/Chrome' to complete"
}

// urlInstall opens the browser's extension store listing for manual one-click install.
func urlInstall(ext config.Extension, dryRun bool, res reporter.Result) reporter.Result {
	url := storeURL(ext.Browser, ext.ExtensionID)

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("would open: %s", url)
		return res
	}

	if err := exec.Command("cmd", "/C", "start", url).Run(); err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("could not open browser: %s", err)
		return res
	}

	res.Status = reporter.StatusInstalled
	res.Detail = storeLabel(ext.Browser)
	return res
}
