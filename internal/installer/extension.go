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
var browserPolicyPaths = map[string]string{
	"brave":  `Software\Policies\BraveSoftware\Brave-Browser\ExtensionInstallForcelist`,
	"chrome": `Software\Policies\Google\Chrome\ExtensionInstallForcelist`,
}

// Chrome Web Store base URL for url mode.
const chromeStoreURL = "https://chrome.google.com/webstore/detail/"

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

	// Value written: "<extension_id>;https://clients2.google.com/service/update2/crx"
	value := fmt.Sprintf("%s;https://clients2.google.com/service/update2/crx", ext.ExtensionID)

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

// urlInstall opens the Chrome Web Store listing for manual one-click install.
func urlInstall(ext config.Extension, dryRun bool, res reporter.Result) reporter.Result {
	url := chromeStoreURL + ext.ExtensionID

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
	res.Detail = "Chrome Web Store page opened — click 'Add to Brave' to complete"
	return res
}
