package installer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

const (
	scrapePageTimeoutSeconds = 30
	scrapeDefaultExecTimeout = 300
)

// ScrapeAndInstall discovers the latest installer URL by scraping a download
// page, downloads it, and runs it silently. It handles its own dry-run and
// already-installed checks so it can be branched to before RunCommand's
// dry-run block.
func ScrapeAndInstall(cmd config.Command, dryRun bool) reporter.Result {
	res := reporter.Result{
		ID:   cmd.ID,
		Name: cmd.Name,
		Tier: "command",
	}

	// 1. Dry-run guard — no network or exec calls.
	if dryRun {
		fmt.Printf("    [dry-run] scrape: %s\n", cmd.ScrapeURL)
		fmt.Printf("    [dry-run] pattern: %s\n", cmd.URLPattern)
		if cmd.InstallArgs != "" {
			fmt.Printf("    [dry-run] install args: %s\n", cmd.InstallArgs)
		}
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("would scrape %s", cmd.ScrapeURL)
		return res
	}

	// 2. Already-installed check — short-circuits before any network call.
	if isAlreadyInstalled(cmd.Check) {
		res.Status = reporter.StatusAlready
		res.Detail = fmt.Sprintf("check passed: %s", cmd.Check)
		return res
	}

	// 3. Fetch the download page.
	pageBody, err := fetchPage(cmd.ScrapeURL)
	if err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("fetch page %s: %v", cmd.ScrapeURL, err)
		return res
	}

	// 4. Extract the download URL using the regex pattern.
	re, err := regexp.Compile(cmd.URLPattern)
	if err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("compile url_pattern: %v", err)
		return res
	}
	downloadURL := re.FindString(pageBody)
	if downloadURL == "" {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("no download URL found matching pattern %q in %s", cmd.URLPattern, cmd.ScrapeURL)
		return res
	}
	fmt.Printf("    found: %s\n", downloadURL)

	// 5. Download the installer to a temp file. defer ensures cleanup.
	tempPath := filepath.Join(os.TempDir(), cmd.ID+"-setup.exe")
	defer os.Remove(tempPath)

	if err := downloadFile(downloadURL, tempPath); err != nil {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("download %s: %v", downloadURL, err)
		return res
	}
	fmt.Printf("    downloaded to: %s\n", tempPath)

	// 6. Execute the installer.
	timeoutSecs := cmd.TimeoutSeconds
	if timeoutSecs <= 0 {
		timeoutSecs = scrapeDefaultExecTimeout
	}
	exitCode, err := runInstaller(tempPath, cmd.InstallArgs, timeoutSecs)
	if exitCode == 0 && err == nil {
		res.Status = reporter.StatusInstalled
	} else {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("installer exit code %d", exitCode)
		if err != nil {
			res.Detail += fmt.Sprintf(": %v", err)
		}
	}
	return res
}

// fetchPage GETs the given URL and returns the response body as a string.
// Returns an error on network failure or non-200 status.
func fetchPage(url string) (string, error) {
	client := &http.Client{Timeout: scrapePageTimeoutSeconds * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

// downloadFile streams the response from url into destPath, truncating any
// existing file (os.Create semantics).
func downloadFile(url, destPath string) error {
	resp, err := http.Get(url) //nolint:gosec // URL comes from config, not user input
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// runInstaller runs the downloaded .exe with the given space-separated args
// and a timeout. Returns the exit code and any execution error.
func runInstaller(exePath, installArgs string, timeoutSeconds int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	args := strings.Fields(installArgs)
	cmd := exec.CommandContext(ctx, exePath, args...)
	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("    │ %s\n", line)
			}
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		return -1, fmt.Errorf("timed out after %ds", timeoutSeconds)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}
