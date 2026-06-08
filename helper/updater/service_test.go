package updater

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

func TestArchiveExecutableNameCanBeDecompressed(t *testing.T) {
	const payload = "updated executable"

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create(archiveExecutableName())
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := file.Write([]byte(payload)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	reader, err := selfupdate.DecompressCommand(
		bytes.NewReader(archive.Bytes()),
		"DysonSphereBox_windows_"+runtime.GOARCH+".zip",
		archiveExecutableName(),
		runtime.GOOS,
		runtime.GOARCH,
	)
	if err != nil {
		t.Fatalf("decompress archive executable: %v", err)
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read decompressed executable: %v", err)
	}
	if string(content) != payload {
		t.Fatalf("unexpected payload: %q", string(content))
	}
}

func TestEnsureWindowsExecutableExtension(t *testing.T) {
	dir := t.TempDir()
	executablePath := filepath.Join(dir, "DysonSphereBox")
	if err := os.WriteFile(executablePath, []byte("updated executable"), 0755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	if err := ensureWindowsExecutableExtension(executablePath); err != nil {
		t.Fatalf("ensure executable extension: %v", err)
	}

	if runtime.GOOS == "windows" {
		if _, err := os.Stat(executablePath + ".exe"); err != nil {
			t.Fatalf("expected .exe file: %v", err)
		}
		return
	}

	if _, err := os.Stat(executablePath); err != nil {
		t.Fatalf("expected original executable path on non-windows: %v", err)
	}
}
