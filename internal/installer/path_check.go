package installer

import "os/exec"

// runtimeTools is the fixed list of tools checked after PATH refresh.
// These are the runtimes most likely to require a PATH update after winget install
// and to be depended on by Tier 2 commands.
var runtimeTools = []string{"git", "node", "python", "go", "rustup", "pwsh"}

// VerifyRuntimePaths checks whether each required runtime tool is findable on PATH.
// Returns a slice of tool names that are missing. An empty slice means all are present.
func VerifyRuntimePaths() []string {
	var missing []string
	for _, tool := range runtimeTools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	return missing
}
