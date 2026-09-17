// Package toolroot locates the Shiploom core tree (stdlib only).
//
// Resolution order: SHIPLOOM_CORE_DIR when set, else the current
// directory when it holds core/VERSION (repo-root runs), else an error
// telling the operator to set SHIPLOOM_CORE_DIR. Installed layouts
// (P5 distribution) will seed the env var or an exe-adjacent tree.
package toolroot

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root returns the core tree directory or an error.
func Root() (string, error) {
	if dir := os.Getenv("SHIPLOOM_CORE_DIR"); dir != "" {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("set SHIPLOOM_CORE_DIR to the Shiploom core tree: %s", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "core", "VERSION")); err == nil {
		return cwd, nil
	}
	return "", fmt.Errorf("set SHIPLOOM_CORE_DIR to the Shiploom core tree (no core/VERSION under %s)", cwd)
}

// Join resolves path segments under the core tree.
func Join(elem ...string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{root}, elem...)...), nil
}
