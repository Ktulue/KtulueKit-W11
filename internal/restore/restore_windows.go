package restore

import (
	"fmt"
	"os/exec"
	"time"
)

// CreateRestorePoint follows the Chris Titus WinUtil pattern:
//  1. Save and zero out SystemRestorePointCreationFrequency to bypass the 24-hour cooldown.
//  2. Enable System Restore on the system drive (no-op if already on).
//  3. Create the checkpoint via Checkpoint-Computer.
//  4. Restore the original frequency value (or remove the key if it didn't exist).
//
// On any failure it prints a warning and returns without blocking the run —
// System Restore may be disabled on the machine or via Group Policy.
func CreateRestorePoint(dryRun bool) {
	ts := time.Now().Format("2006-01-02 15:04")
	name := fmt.Sprintf("Pre-KtulueKit %s", ts)

	if dryRun {
		fmt.Printf("  [dry-run] Would create restore point: %q\n", name)
		return
	}

	fmt.Printf("  Creating restore point: %q ...\n", name)

	// Single PowerShell block with try/finally so the frequency key is always
	// reset even if Checkpoint-Computer throws.
	script := fmt.Sprintf(`
$regPath = "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\SystemRestore"
$regName = "SystemRestorePointCreationFrequency"

# Save original value (key may not exist)
$orig = $null
$origExists = $false
try {
    $orig = (Get-ItemProperty -Path $regPath -Name $regName -ErrorAction Stop).$regName
    $origExists = $true
} catch {}

try {
    # Bypass the 24-hour cooldown Windows enforces between restore points
    Set-ItemProperty -Path $regPath -Name $regName -Value 0 -Type DWord -Force | Out-Null

    # Enable System Restore on system drive in case it was turned off
    Enable-ComputerRestore -Drive "$env:SystemDrive" -ErrorAction SilentlyContinue

    Checkpoint-Computer -Description %q -RestorePointType APPLICATION_INSTALL -ErrorAction Stop
} finally {
    # Restore original frequency value
    if ($origExists) {
        Set-ItemProperty -Path $regPath -Name $regName -Value $orig -Type DWord -Force | Out-Null
    } else {
        Remove-ItemProperty -Path $regPath -Name $regName -ErrorAction SilentlyContinue
    }
}
`, name)

	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive",
		"-Command", script,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		output := string(out)
		fmt.Printf("  [warning] Restore point creation failed (System Restore may be disabled): %v\n", err)
		if output != "" {
			fmt.Printf("            %s\n", output)
		}
		return
	}

	fmt.Println("  Restore point created.")
}
