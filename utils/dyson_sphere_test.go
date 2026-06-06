package utils

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestFindDysonSphereDirFromRegistrySteamRoot(t *testing.T) {
	tempDir := t.TempDir()
	steamRoot := filepath.Join(tempDir, "Steam")
	libraryRoot := filepath.Join(tempDir, "Library")
	gameDir := filepath.Join(libraryRoot, "steamapps", "common", "CustomDSP")

	mustWriteText(t, filepath.Join(gameDir, DysonSphereExeName), "")
	mustWriteText(t, filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"), `"libraryfolders" { "1" { "path" "`+escapeVdfPath(libraryRoot)+`" } }`)
	mustWriteText(t, filepath.Join(libraryRoot, "steamapps", "appmanifest_"+DysonSphereSteamAppID+".acf"), `"AppState" { "appid" "`+DysonSphereSteamAppID+`" "installdir" "CustomDSP" }`)

	dirPath, err := FindDysonSphereDir(DysonSphereSearchOptions{
		SteamRoots: []string{steamRoot},
	})
	if err != nil {
		t.Fatalf("FindDysonSphereDir returned error: %v", err)
	}
	if dirPath != gameDir {
		t.Fatalf("FindDysonSphereDir = %q, want %q", dirPath, gameDir)
	}
}

func TestFindDysonSphereDirFromChineseShortcut(t *testing.T) {
	testFindDysonSphereDirFromShortcut(t, "戴森球计划")
}

func TestFindDysonSphereDirFromEnglishShortcut(t *testing.T) {
	testFindDysonSphereDirFromShortcut(t, "Dyson Sphere Program")
}

func TestParseWindowsShortcutTarget(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), DysonSphereExeName)
	shortcut := buildWindowsShortcutForTest(targetPath)

	parsedPath, err := ParseWindowsShortcutTarget(shortcut)
	if err != nil {
		t.Fatalf("ParseWindowsShortcutTarget returned error: %v", err)
	}
	if parsedPath != targetPath {
		t.Fatalf("ParseWindowsShortcutTarget = %q, want %q", parsedPath, targetPath)
	}
}

func testFindDysonSphereDirFromShortcut(t *testing.T, shortcutName string) {
	t.Helper()

	tempDir := t.TempDir()
	shortcutRoot := filepath.Join(tempDir, "Desktop")
	gameDir := filepath.Join(tempDir, "Dyson Sphere Program")
	exePath := filepath.Join(gameDir, DysonSphereExeName)

	mustWriteText(t, exePath, "")
	mustWriteText(t, filepath.Join(shortcutRoot, shortcutName+".lnk"), "shortcut placeholder")

	dirPath, err := FindDysonSphereDir(DysonSphereSearchOptions{
		ShortcutRoots: []string{shortcutRoot},
		ShortcutResolver: func(shortcutPath string) (string, error) {
			if filepath.Base(shortcutPath) != shortcutName+".lnk" {
				return "", os.ErrNotExist
			}

			return exePath, nil
		},
	})
	if err != nil {
		t.Fatalf("FindDysonSphereDir returned error: %v", err)
	}
	if dirPath != gameDir {
		t.Fatalf("FindDysonSphereDir = %q, want %q", dirPath, gameDir)
	}
}

func buildWindowsShortcutForTest(targetPath string) []byte {
	content := make([]byte, lnkHeaderSize)
	binary.LittleEndian.PutUint32(content[0:4], lnkHeaderSize)
	binary.LittleEndian.PutUint32(content[0x14:0x18], 0x02)

	linkInfo := make([]byte, 0x24)
	binary.LittleEndian.PutUint32(linkInfo[4:8], 0x24)
	binary.LittleEndian.PutUint32(linkInfo[16:20], 0x24)
	binary.LittleEndian.PutUint32(linkInfo[28:32], 0x24)

	for _, char := range utf16.Encode([]rune(targetPath)) {
		linkInfo = binary.LittleEndian.AppendUint16(linkInfo, char)
	}
	linkInfo = binary.LittleEndian.AppendUint16(linkInfo, 0)
	binary.LittleEndian.PutUint32(linkInfo[0:4], uint32(len(linkInfo)))

	return append(content, linkInfo...)
}

func mustWriteText(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
}

func escapeVdfPath(path string) string {
	return strings.ReplaceAll(path, `\`, `\\`)
}
