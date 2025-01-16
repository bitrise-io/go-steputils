package compression

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAreAllPathsEmpty(t *testing.T) {
	// Set up test dir structure
	basePath := t.TempDir()
	err := os.MkdirAll(filepath.Join(basePath, "empty_dir"), 0700)
	if err != nil {
		t.Fatalf("failed to create empty directory: %s", err)
	}
	err = os.MkdirAll(filepath.Join(basePath, "dir_with_dir_child", "nested_empty_dir"), 0700)
	if err != nil {
		t.Fatalf("failed to create directory with empty nested dir: %s", err)
	}
	err = os.MkdirAll(filepath.Join(basePath, "first_level", "second_level"), 0700)
	if err != nil {
		t.Fatalf("failed to create directory with a second level directory: %s", err)
	}
	err = ioutil.WriteFile(filepath.Join(basePath, "first_level", "second_level", "nested_file.txt"), []byte("hello"), 0700)
	if err != nil {
		t.Fatalf("failed to write file: %s", err)
	}

	tests := []struct {
		name         string
		includePaths []string
		want         bool
	}{
		{
			name: "single empty dir",
			includePaths: []string{
				filepath.Join(basePath, "empty_dir"),
			},
			want: true,
		},
		{
			name: "dir with files",
			includePaths: []string{
				filepath.Join(basePath, "first_level", "second_level", "nested_file.txt"),
			},
			want: false,
		},
		{
			name: "empty dir within dir",
			includePaths: []string{
				filepath.Join(basePath, "dir_with_dir_child"),
			},
			want: false,
		},
		{
			name: "empty and non-empty dirs",
			includePaths: []string{
				filepath.Join(basePath, "empty_dir"),
				filepath.Join(basePath, "first_level"),
			},
			want: false,
		},
		{
			name: "nonexistent dir",
			includePaths: []string{
				filepath.Join(basePath, "this doesn't exist"),
			},
			want: true,
		},
		{
			name: "file path",
			includePaths: []string{
				filepath.Join(basePath, "this doesn't exist"),
				filepath.Join(basePath, "empty_dir"),
				filepath.Join(basePath, "first_level", "second_level", "nested_file.txt"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AreAllPathsEmpty(tt.includePaths); got != tt.want {
				t.Errorf("areAllPathsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
