package main

import "golang.org/x/sys/windows"

// isAdmin returns true if the current process is running with elevated privileges.
func isAdmin() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}
