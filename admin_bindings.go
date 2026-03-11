//go:build bindings

package main

// isAdmin is overridden during Wails binding generation (wails build -tags bindings)
// to skip the administrator check so the build toolchain can introspect bindings.
func isAdmin() bool {
	return true
}
