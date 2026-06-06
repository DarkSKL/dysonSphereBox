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

	for _, path := range ParseSteamVdfValues(content, "path") {
		AppendUniquePath(&paths, seen, path)
	}

	legacyLibraryExpression := regexp.MustCompile(`"\d+"\s+"([^"]+)"`)
	for _, match := range legacyLibraryExpression.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}

		AppendUniquePath(&paths, seen, DecodeSteamVdfPath(match[1]))
	}

	return paths
}

func ReadSteamAppInstallDir(libraryRoot string, appID string) (string, error) {
	if libraryRoot == "" || appID == "" {
		return "", nil
	}

	content, err := os.ReadFile(filepath.Join(libraryRoot, "steamapps", "appmanifest_"+appID+".acf"))
	if err != nil {
		return "", err
	}

	values := ParseSteamVdfValues(string(content), "installdir")
	if len(values) == 0 {
		return "", nil
	}

	return filepath.Join(libraryRoot, "steamapps", "common", values[0]), nil
}

func ParseSteamVdfValues(content string, key string) []string {
	values := make([]string, 0)
	if key == "" {
		return values
	}

	expression := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s+"([^"]+)"`)
	for _, match := range expression.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}

		values = append(values, DecodeSteamVdfPath(match[1]))
	}

	return values
}

func DecodeSteamVdfPath(path string) string {
	return strings.ReplaceAll(path, `\\`, `\`)
}
