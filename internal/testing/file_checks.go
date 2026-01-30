package testing

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

// FileChecker allows chaining multiple checks on a file path.
type FileChecker struct {
	Path   string
	Checks []func(string) error
}

// NewFileChecker creates a FileChecker for the given path.
func NewFileChecker(path string) *FileChecker {
	return &FileChecker{Path: path, Checks: []func(string) error{}}
}

// Check runs all checks on the FileChecker's path, returning the first error encountered.
func (fc *FileChecker) Check() error {
	errors := MultiError{}
	for _, check := range fc.Checks {
		if err := check(fc.Path); err != nil {
			AppendErr(&errors, err)
		}
	}

	if len(errors) == 0 {
		return nil
	}

	return errors
}

// IsDir adds a check that the path is a directory.
func (fc *FileChecker) IsDir() *FileChecker {
	fc.Checks = append(fc.Checks, func(path string) error {
		info, err := getInfo(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("expected directory but not a directory: %s", path)
		}
		return nil
	})
	return fc
}

// IsFile adds a check that the path is a regular file.
func (fc *FileChecker) IsFile() *FileChecker {
	fc.Checks = append(fc.Checks, func(path string) error {
		info, err := getInfo(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("expected file but is a directory: %s", path)
		}
		return nil
	})
	return fc
}

// IsSymlink adds a check that the path is a symlink.
func (fc *FileChecker) IsSymlink() *FileChecker {
	fc.Checks = append(fc.Checks, func(path string) error {
		info, err := getInfo(path)
		if err != nil {
			return err
		}
		if (info.Mode() & os.ModeSymlink) == 0 {
			return fmt.Errorf("expected %s to be a symlink, but it's not", path)
		}
		return nil
	})
	return fc
}

// ModeEquals adds a check that the path has the specified permission bits.
func (fc *FileChecker) ModeEquals(perm os.FileMode) *FileChecker {
	modeEqualsFunc := func(path string, wantPerm os.FileMode) error {
		info, err := getInfo(path)
		if err != nil {
			return err
		}
		got := info.Mode().Perm()
		if got != wantPerm.Perm() {
			return fmt.Errorf("mode mismatch for %s: want %o got %o", path, wantPerm.Perm(), got)
		}
		return nil
	}
	fc.Checks = append(fc.Checks, func(path string) error {
		return modeEqualsFunc(path, perm)
	})
	return fc
}

// Owner adds a check that the path has the specified uid and gid.
func (fc *FileChecker) Owner(uid, gid int) *FileChecker {
	ownerFunc := func(path string, wantUID, wantGID int) error {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("owner check not supported on Windows in this helper")
		}
		info, err := getInfo(path)
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("failed to get underlying Stat_t for %s", path)
		}
		gotUID := int(stat.Uid)
		gotGID := int(stat.Gid)
		if gotUID != wantUID || gotGID != wantGID {
			return fmt.Errorf("owner mismatch for %s: want uid/gid %d/%d got %d/%d", path, wantUID, wantGID, gotUID, gotGID)
		}
		return nil
	}

	fc.Checks = append(fc.Checks, func(path string) error {
		return ownerFunc(path, uid, gid)
	})
	return fc
}

// Content adds a check that the file at the path has the specified content.
func (fc *FileChecker) Content(content string) *FileChecker {
	checkContentFunc := func(path string, want string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got := string(b)
		if got != want {
			return fmt.Errorf("file %s content mismatch\nwant:\n%q\n\ngot:\n%q", path, want, got)
		}
		return nil
	}

	fc.Checks = append(fc.Checks, func(path string) error {
		return checkContentFunc(path, content)
	})
	return fc
}

func getInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path does not exist: %s", path)
		}
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return info, nil
}
