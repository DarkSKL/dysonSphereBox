package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	DysonSphereExeName    = "DSPGAME.exe"
	DysonSphereDirName    = "Dyson Sphere Program"
	DysonSphereSteamAppID = "1366540"
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
	return FindDysonSphereDir(DysonSphereSearchOptions{
		SteamRoots:       SteamRootCandidates(),
		ShortcutRoots:    ShortcutRootCandidates(),
		ShortcutResolver: ResolveWindowsShortcutTarget,
	})
}

type DysonSphereSearchOptions struct {
	SteamRoots       []string
	ShortcutRoots    []string
	ShortcutResolver func(string) (string, error)
}

func FindDysonSphereDir(options DysonSphereSearchOptions) (string, error) {
	libraryRoots := make([]string, 0, len(options.SteamRoots))
	seen := make(map[string]struct{})

	for _, steamRoot := range options.SteamRoots {
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
		manifestDirPath, err := ReadSteamAppInstallDir(libraryRoot, DysonSphereSteamAppID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if dirPath := NormalizeDysonSphereDir(manifestDirPath); dirPath != "" {
			return dirPath, nil
		}

		dirPath := filepath.Join(libraryRoot, "steamapps", "common", DysonSphereDirName)
		if NormalizeDysonSphereDir(dirPath) != "" {
			return dirPath, nil
		}
	}

	if dirPath, err := FindDysonSphereDirFromShortcuts(options.ShortcutRoots, options.ShortcutResolver); err != nil || dirPath != "" {
		return dirPath, err
	}

	return "", nil
}

func FindDysonSphereDirFromShortcuts(shortcutRoots []string, shortcutResolver func(string) (string, error)) (string, error) {
	if shortcutResolver == nil {
		return "", nil
	}

	for _, shortcutRoot := range shortcutRoots {
		shortcutRoot = CleanPath(shortcutRoot)
		if shortcutRoot == "" {
			continue
		}

		for _, shortcutName := range DysonSphereShortcutNames() {
			targetPath, err := shortcutResolver(filepath.Join(shortcutRoot, shortcutName+".lnk"))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return "", err
			}
			if dirPath := NormalizeDysonSphereDir(targetPath); dirPath != "" {
				return dirPath, nil
			}
		}
	}

	return "", nil
}

func DysonSphereShortcutNames() []string {
	return []string{
		"Dyson Sphere Program",
		"戴森球计划",
	}
}

func SteamRootCandidates() []string {
	paths := make([]string, 0, 6)
	seen := make(map[string]struct{})

	for _, envName := range []string{"STEAM_PATH", "STEAM_DIR"} {
		AppendUniquePath(&paths, seen, os.Getenv(envName))
	}

	for _, registryPath := range []struct {
		key   registry.Key
		path  string
		value string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamExe"},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `Software\Valve\Steam`, "InstallPath"},
	} {
		AppendUniquePath(&paths, seen, ReadRegistrySteamPath(registryPath.key, registryPath.path, registryPath.value))
	}

	AppendUniquePath(&paths, seen, filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam"))
	AppendUniquePath(&paths, seen, filepath.Join(os.Getenv("ProgramFiles"), "Steam"))

	if homeDir, err := os.UserHomeDir(); err == nil {
		AppendUniquePath(&paths, seen, filepath.Join(homeDir, ".steam", "steam"))
		AppendUniquePath(&paths, seen, filepath.Join(homeDir, "Library", "Application Support", "Steam"))
	}

	return paths
}

func ShortcutRootCandidates() []string {
	paths := make([]string, 0, 8)
	seen := make(map[string]struct{})

	if publicDir := os.Getenv("PUBLIC"); publicDir != "" {
		AppendUniquePath(&paths, seen, filepath.Join(publicDir, "Desktop"))
	}

	if programDataDir := os.Getenv("PROGRAMDATA"); programDataDir != "" {
		AppendUniquePath(&paths, seen, filepath.Join(programDataDir, "Microsoft", "Windows", "Start Menu", "Programs"))
		AppendUniquePath(&paths, seen, filepath.Join(programDataDir, "Microsoft", "Windows", "Start Menu", "Programs", "Steam"))
	}

	if appDataDir := os.Getenv("APPDATA"); appDataDir != "" {
		AppendUniquePath(&paths, seen, filepath.Join(appDataDir, "Microsoft", "Windows", "Start Menu", "Programs"))
		AppendUniquePath(&paths, seen, filepath.Join(appDataDir, "Microsoft", "Windows", "Start Menu", "Programs", "Steam"))
	}

	if userProfileDir := os.Getenv("USERPROFILE"); userProfileDir != "" {
		AppendUniquePath(&paths, seen, filepath.Join(userProfileDir, "Desktop"))
	}

	return paths
}

func ReadRegistrySteamPath(root registry.Key, path string, value string) string {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	steamPath, _, err := key.GetStringValue(value)
	if err != nil {
		return ""
	}

	if strings.EqualFold(filepath.Base(steamPath), "steam.exe") {
		return filepath.Dir(steamPath)
	}

	return steamPath
}
