//go:build !bindings

package main

import "os"

// isAdmin returns true if the current process has administrator privileges.
func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}
