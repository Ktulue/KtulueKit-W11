package installer

import "os"

// setenv wraps os.Setenv for platform-specific PATH injection.
func setenv(key, value string) error {
	return os.Setenv(key, value)
}
