package utils

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	DysonSphereExeName = "DSPGAME.exe"
	DysonSphereDirName = "Dyson Sphere Program"
)

func NormalizeDysonSphereDir(path string) string {
	path = CleanPath(path)
	if path == "" {
		return ""
	}

	if strings.EqualFold(filepath.Base(path), DysonSphereExeName) {
		if FileExists(path) {
			return filepath.Dir(path)
		}
		return ""
	}

	if FileExists(filepath.Join(path, DysonSphereExeName)) {
		return path
	}

	return ""
}

func FindDysonSphereDirFromSteam() (string, error) {
	steamRoots := SteamRootCandidates()
	libraryRoots := make([]string, 0, len(steamRoots))
	seen := make(map[string]struct{})

	for _, steamRoot := range steamRoots {
		AppendUniquePath(&libraryRoots, seen, steamRoot)

		paths, err := ReadSteamLibraryFolders(steamRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}

		for _, path := range paths {
			AppendUniquePath(&libraryRoots, seen, path)
		}
	}

	for _, libraryRoot := range libraryRoots {
		dirPath := filepath.Join(libraryRoot, "steamapps", "common", DysonSphereDirName)
		if NormalizeDysonSphereDir(dirPath) != "" {
			return dirPath, nil
		}
	}

	return "", nil
}

func SteamRootCandidates() []string {
	paths := make([]string, 0, 6)
	seen := make(map[string]struct{})

	for _, envName := range []string{"STEAM_PATH", "STEAM_DIR"} {
		AppendUniquePath(&paths, seen, os.Getenv(envName))
	}

	AppendUniquePath(&paths, seen, filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam"))
	AppendUniquePath(&paths, seen, filepath.Join(os.Getenv("ProgramFiles"), "Steam"))

	if homeDir, err := os.UserHomeDir(); err == nil {
		AppendUniquePath(&paths, seen, filepath.Join(homeDir, ".steam", "steam"))
		AppendUniquePath(&paths, seen, filepath.Join(homeDir, "Library", "Application Support", "Steam"))
	}

	return paths
}
