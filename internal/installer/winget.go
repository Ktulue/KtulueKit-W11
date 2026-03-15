package installer

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

// CheckConnectivity verifies that the machine can reach the internet via DNS.
// Returns (true, "") on success. On failure returns (false, reason) where reason
// includes a LAN-mode hint when a non-loopback network interface is active —
// this catches the common case of being on a LAN without internet access.
func CheckConnectivity() (ok bool, reason string) {
	_, err := net.LookupHost("dns.msftncsi.com")
	if err == nil {
		return true, ""
	}

	// DNS failed — check if there's an active non-loopback interface.
	// If so, the machine is on a LAN but has no internet route.
	ifaces, ifErr := net.Interfaces()
	if ifErr == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
					return false, fmt.Sprintf(
						"LAN connection detected (%s on %s) but no internet access — winget requires internet to download packages",
						ip, iface.Name,
					)
				}
			}
		}
	}

	return false, "no network connectivity detected — winget requires internet to download packages"
}

// CheckWingetAvailable verifies that winget is on PATH and functional.
// Returns an error if winget is missing or does not respond within 5 seconds.
func CheckWingetAvailable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "winget", "--version").Run()
}

// UpdateSources runs "winget source update" to refresh the package database.
// Output is streamed to the console. Returns an error if the command fails.
func UpdateSources() error {
	cmd := exec.Command("winget", "source", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// InstallPackage runs a single winget install for a Tier 1 package.
// If upgradeIfInstalled is true and the pre-check passes, runs winget upgrade instead of skipping.
// Returns a Result reflecting what happened.
func InstallPackage(pkg config.Package, dryRun bool, retryCount int, upgradeIfInstalled bool) reporter.Result {
	res := reporter.Result{
		ID:   pkg.ID,
		Name: pkg.Name,
		Tier: "winget",
	}

	if pkg.Notes != "" && dryRun {
		fmt.Printf("    note: %s\n", pkg.Notes)
	}

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("winget install -e --id %s --scope %s", pkg.ID, pkg.Scope)
		return res
	}

	// Pre-check: if a check command is provided and passes, either upgrade or skip.
	if pkg.Check != "" && isAlreadyInstalled(pkg.Check) {
		if upgradeIfInstalled {
			fmt.Printf("    already installed — checking for updates...\n")
			return runWingetUpgrade(pkg, retryCount)
		}
		res.Status = reporter.StatusAlready
		res.Detail = fmt.Sprintf("pre-check passed: %s", pkg.Check)
		return res
	}

	args := buildWingetArgs(pkg)

	var exitCode int
	var err error

	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			fmt.Printf("    retrying %s (attempt %d)...\n", pkg.Name, attempt+1)
			time.Sleep(3 * time.Second)
		}

		exitCode, err = runWithTimeout(args, pkg.TimeoutSeconds)
		if err == nil || exitCode == 0 {
			break
		}
	}

	res.ExitCode = exitCode
	res.Status, res.Detail = classifyWingetExit(exitCode, err)
	return res
}

// buildWingetArgs constructs the winget install argument slice.
func buildWingetArgs(pkg config.Package) []string {
	args := []string{
		"install",
		"-e",
		"--id", pkg.ID,
		"--scope", pkg.Scope,
		"--accept-package-agreements",
		"--accept-source-agreements",
		"--disable-interactivity",
	}
	if pkg.Version != "" {
		args = append(args, "--version", pkg.Version)
	}
	return args
}

// classifyWingetExit maps winget exit codes to Result statuses.
//
// Windows HRESULTs are 32-bit signed values. Go's exec.ExitCode() returns an
// int (64-bit on amd64), so a code like 0x8A15002B arrives as 2316632107 (large
// positive) rather than -1978335189. Casting to int32 before the switch restores
// the expected sign so the cases match correctly.
//
// Reference: https://github.com/microsoft/winget-cli/blob/master/src/AppInstallerSharedLib/Public/AppInstallerErrors.h
func classifyWingetExit(code int, runErr error) (status, detail string) {
	switch int32(code) {
	case 0:
		return reporter.StatusInstalled, ""

	// ── Already up to date / nothing to do ────────────────────────────────
	case -1978335189: // 0x8A15002B — UPDATE_NOT_APPLICABLE
		return reporter.StatusAlready, "winget: no update applicable (already up to date)"
	case -1978335212: // 0x8A150014 — NO_APPLICATIONS_FOUND (no upgrade available)
		return reporter.StatusAlready, "winget: no applicable upgrade found"
	case -1978335216: // 0x8A150010 — NO_APPLICABLE_INSTALLER (already installed via other method)
		return reporter.StatusAlready, "winget: no applicable installer (package may already be installed)"

	// ── Reboot / environment ───────────────────────────────────────────────
	case -1978335215: // 0x8A150011 — install blocked / reboot pending
		return reporter.StatusReboot, "winget: reboot may be required"

	// ── Real failures ──────────────────────────────────────────────────────
	case -1978335146: // 0x8A150056 — INSTALLER_PROHIBITS_ELEVATION
		return reporter.StatusFailed, "winget: installer prohibits elevation (try --scope user or run without admin)"
	case -1978334968: // 0x8A150108 — INSTALL_CONTACT_SUPPORT
		return reporter.StatusFailed, "winget: installer error (contact support)"
	case -1978334957: // 0x8A150113 — INSTALL_SYSTEM_NOT_SUPPORTED
		return reporter.StatusFailed, "winget: system does not meet package requirements"

	default:
		detail := fmt.Sprintf("exit code %d (0x%08X)", code, uint32(code))
		if runErr != nil {
			detail += fmt.Sprintf(": %s", runErr.Error())
		}
		return reporter.StatusFailed, detail
	}
}

// UninstallPackage runs winget uninstall for a Tier 1 package.
// If pkg.Check exits non-zero (not installed), returns StatusSkipped.
// If pkg.Check is empty, runs unconditionally (winget handles "not found").
func UninstallPackage(pkg config.Package, dryRun bool) reporter.Result {
	res := reporter.Result{ID: pkg.ID, Name: pkg.Name, Tier: "winget"}

	if dryRun {
		res.Status = reporter.StatusDryRun
		res.Detail = fmt.Sprintf("winget uninstall -e --id %s", pkg.ID)
		return res
	}

	if pkg.Check != "" && !isAlreadyInstalled(pkg.Check) {
		res.Status = reporter.StatusSkipped
		res.Detail = "not detected as installed — skipping"
		return res
	}

	args := []string{
		"uninstall", "-e", "--id", pkg.ID,
		"--accept-source-agreements", "--disable-interactivity",
	}

	exitCode, err := runWithTimeout(args, pkg.TimeoutSeconds)
	res.ExitCode = exitCode
	if exitCode == 0 && err == nil {
		res.Status = reporter.StatusInstalled
	} else {
		res.Status = reporter.StatusFailed
		res.Detail = fmt.Sprintf("exit code %d", exitCode)
		if err != nil {
			res.Detail += fmt.Sprintf(": %s", err.Error())
		}
	}
	return res
}

// runWingetUpgrade attempts to upgrade an already-installed package.
// Exit 0 → StatusUpgraded; "no update applicable" codes → StatusAlready; other failures → StatusFailed.
func runWingetUpgrade(pkg config.Package, retryCount int) reporter.Result {
	res := reporter.Result{
		ID:   pkg.ID,
		Name: pkg.Name,
		Tier: "winget",
	}

	args := []string{
		"upgrade",
		"-e",
		"--id", pkg.ID,
		"--scope", pkg.Scope,
		"--accept-package-agreements",
		"--accept-source-agreements",
		"--disable-interactivity",
	}

	var exitCode int
	var err error

	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			fmt.Printf("    retrying upgrade %s (attempt %d)...\n", pkg.Name, attempt+1)
			time.Sleep(3 * time.Second)
		}
		exitCode, err = runWithTimeout(args, pkg.TimeoutSeconds)
		if err == nil || exitCode == 0 {
			break
		}
	}

	res.ExitCode = exitCode
	status, detail := classifyWingetExit(exitCode, err)

	// Translate a clean install exit into "upgraded" since we know it was already present.
	if status == reporter.StatusInstalled {
		status = reporter.StatusUpgraded
	}

	res.Status = status
	res.Detail = detail
	return res
}

// runWithTimeout executes winget with the given args and a timeout.
// Returns the exit code and any execution error.
func runWithTimeout(args []string, timeoutSeconds int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "winget", args...)

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		// Stream output lines prefixed so they're visually nested
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
			return exitErr.ExitCode(), nil // non-zero exit, but not an exec failure
		}
		return -1, err
	}

	return 0, nil
}
