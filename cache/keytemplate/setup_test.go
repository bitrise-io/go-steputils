package keytemplate

import (
	"os"
	"path/filepath"
	"testing"
)

func createTempFileAndGetChecksum(t *testing.T, dir string) (string, []byte, error) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	tmpFile, err := os.Create(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	_, err = tmpFile.WriteString("test")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	err = tmpFile.Close()
	if err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	checksum, err := checksumOfFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to calculate checksum: %v", err)
	}

	return tmpFile.Name(), checksum, nil
}
