package export

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-utils/ziputil"
)

const (
	filesType              = "files"
	foldersType            = "folders"
	mixedFileAndFolderType = "mixed"
)

// Exporter ...
type Exporter struct {
	cmdFactory command.Factory
}

// NewExporter ...
func NewExporter(cmdFactory command.Factory) Exporter {
	return Exporter{cmdFactory: cmdFactory}
}

// ExportOutput is used for exposing values for other steps.
// Regular env vars are isolated between steps, so instead of calling `os.Setenv()`, use this to explicitly expose
// a value for subsequent steps.
func (e *Exporter) ExportOutput(key, value string) error {
	cmd := e.cmdFactory.Create("envman", []string{"add", "--key", key, "--value", value}, nil)
	return runExport(cmd)
}

// ExportOutputNoExpand works like ExportOutput but does not expand environment variables in the value.
// This can be used when the value is unstrusted or is beyond the control of the step.
func (e *Exporter) ExportOutputNoExpand(key, value string) error {
	cmd := e.cmdFactory.Create("envman", []string{"add", "--key", key, "--value", value, "--no-expand"}, nil)
	return runExport(cmd)
}

// ExportSecretOutput is used for exposing secret values for other steps.
// Regular env vars are isolated between steps, so instead of calling `os.Setenv()`, use this to explicitly expose
// a secret value for subsequent steps.
func (e *Exporter) ExportSecretOutput(key, value string) error {
	cmd := e.cmdFactory.Create("envman", []string{"add", "--key", key, "--value", value, "--sensitive"}, nil)
	return runExport(cmd)
}

// ExportOutputFile is a convenience method for copying sourcePath to destinationPath and then exporting the
// absolute destination path with ExportOutput()
func (e *Exporter) ExportOutputFile(key, sourcePath, destinationPath string) error {
	pathModifier := pathutil.NewPathModifier()
	absSourcePath, err := pathModifier.AbsPath(sourcePath)
	if err != nil {
		return err
	}
	absDestinationPath, err := pathModifier.AbsPath(destinationPath)
	if err != nil {
		return err
	}

	if absSourcePath != absDestinationPath {
		if err = copyFile(absSourcePath, absDestinationPath); err != nil {
			return err
		}
	}

	return e.ExportOutput(key, absDestinationPath)
}

// ExportOutputDir is a convenience method for copying sourceDir to destinationDir and then exporting the
// absolute destination path with ExportOutput()
// Note: Symlinks are followed when copying the directory (instead of copying the symlink itself)
func (e *Exporter) ExportOutputDir(key, sourceDir, destinationDir string) error {
	pathModifier := pathutil.NewPathModifier()
	absSourceDir, err := pathModifier.AbsPath(sourceDir)
	if err != nil {
		return fmt.Errorf("resolve source directory path: %w", err)
	}
	absDestinationDir, err := pathModifier.AbsPath(destinationDir)
	if err != nil {
		return fmt.Errorf("resolve destination directory path: %w", err)
	}

	if absSourceDir != absDestinationDir {
		if err = copyDir(absSourceDir, absDestinationDir); err != nil {
			return err
		}
	}

	return e.ExportOutput(key, absDestinationDir)
}

// ExportOutputFilesZip is a convenience method for creating a ZIP archive from sourcePaths at zipPath and then
// exporting the absolute path of the ZIP with ExportOutput()
func (e *Exporter) ExportOutputFilesZip(key string, sourcePaths []string, zipPath string) error {
	tempZipPath, err := zipFilePath()
	if err != nil {
		return err
	}

	// We have separate zip functions for files and folders and that is the main reason we cannot have mixed
	// paths (files and also folders) in the input. It has to be either folders or files. Everything
	// else leads to an error.
	inputType, err := getInputType(sourcePaths)
	if err != nil {
		return err
	}
	switch inputType {
	case filesType:
		err = ziputil.ZipFiles(sourcePaths, tempZipPath)
	case foldersType:
		err = ziputil.ZipDirs(sourcePaths, tempZipPath)
	case mixedFileAndFolderType:
		return fmt.Errorf("source path list (%s) contains a mix of files and folders", sourcePaths)
	default:
		return fmt.Errorf("source path list (%s) is empty", sourcePaths)
	}

	if err != nil {
		return err
	}

	return e.ExportOutputFile(key, tempZipPath, zipPath)
}

func zipFilePath() (string, error) {
	tmpDir, err := pathutil.NewPathProvider().CreateTempDir("__export_tmp_dir__")
	if err != nil {
		return "", err
	}

	return filepath.Join(tmpDir, "temp-zip-file.zip"), nil
}

func getInputType(sourcePths []string) (string, error) {
	var folderCount, fileCount int
	pathChecker := pathutil.NewPathChecker()

	for _, path := range sourcePths {
		exist, err := pathChecker.IsDirExists(path)
		if err != nil {
			return "", err
		}

		if exist {
			folderCount++
			continue
		}

		exist, err = pathChecker.IsPathExists(path)
		if err != nil {
			return "", err
		}

		if exist {
			fileCount++
		}
	}

	if fileCount == len(sourcePths) {
		return filesType, nil
	} else if folderCount == len(sourcePths) {
		return foldersType, nil
	} else if 0 < folderCount && 0 < fileCount {
		return mixedFileAndFolderType, nil
	}

	return "", nil
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			log.Fatalf("Failed to close output file: %s", err)
		}
	}(out)

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return nil
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source directory: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("calculate relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		// Follow symlinks by using os.Stat instead of the lstat-based info from Walk
		if info.Mode()&os.ModeSymlink != 0 {
			info, err = os.Stat(path)
			if err != nil {
				return fmt.Errorf("follow symlink %s: %w", path, err)
			}
		}

		if info.IsDir() {
			if err := os.MkdirAll(dstPath, info.Mode()); err != nil {
				return fmt.Errorf("create directory %s: %w", dstPath, err)
			}
		} else if info.Mode().IsRegular() {
			if err := copyFile(path, dstPath); err != nil {
				return fmt.Errorf("copy file %s: %w", path, err)
			}
			if err := os.Chmod(dstPath, info.Mode()); err != nil {
				return fmt.Errorf("set permissions on %s: %w", dstPath, err)
			}
		} else {
			return fmt.Errorf("unsupported file type for %s (mode: %s)", path, info.Mode())
		}

		return nil
	})
}

func runExport(cmd command.Command) error {
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("exporting output with envman failed: %s, output: %s", err, out)
	}
	return nil
}
