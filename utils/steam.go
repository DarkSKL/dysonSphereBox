package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func ReadSteamLibraryFolders(steamRoot string) ([]string, error) {
	if steamRoot == "" {
		return nil, nil
	}

	content, err := os.ReadFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"))
	if os.IsNotExist(err) {
		content, err = os.ReadFile(filepath.Join(steamRoot, "config", "libraryfolders.vdf"))
	}
	if err != nil {
		return nil, err
	}

	return ParseSteamLibraryFolders(string(content)), nil
}

func ParseSteamLibraryFolders(content string) []string {
	paths := make([]string, 0)
	seen := make(map[string]struct{})

	for _, expression := range []*regexp.Regexp{
		regexp.MustCompile(`"path"\s+"([^"]+)"`),
		regexp.MustCompile(`"\d+"\s+"([^"]+)"`),
	} {
		for _, match := range expression.FindAllStringSubmatch(content, -1) {
			if len(match) < 2 {
				continue
			}

			AppendUniquePath(&paths, seen, strings.ReplaceAll(match[1], `\\`, `\`))
		}
	}

	return paths
}
