package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func CleanPath(path string) string {
	path = strings.Trim(strings.TrimSpace(os.ExpandEnv(path)), `"`)
	if path == "" {
		return ""
	}

	return filepath.Clean(path)
}

func AppendUniquePath(paths *[]string, seen map[string]struct{}, path string) {
	path = CleanPath(path)
	if path == "" {
		return
	}

	key := strings.ToLower(path)
	if _, ok := seen[key]; ok {
		return
	}

	seen[key] = struct{}{}
	*paths = append(*paths, path)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
