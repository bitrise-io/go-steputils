package keytemplate

import (
	"fmt"
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
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	_, err = tmpFile.WriteString("test")
	if err != nil {
		return "", nil, fmt.Errorf("failed to write to temp file: %w", err)
	}
	err = tmpFile.Close()
	if err != nil {
		return "", nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	checksum, err := checksumOfFile(tmpFile.Name())
	if err != nil {
		return "", nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return tmpFile.Name(), checksum, nil
}
