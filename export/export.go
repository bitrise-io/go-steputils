package export

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/fileutil"
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
	cmdFactory  command.Factory
	fileManager fileutil.FileManager
}

// NewExporter ...
func NewExporter(cmdFactory command.Factory) Exporter {
	return Exporter{
		cmdFactory:  cmdFactory,
		fileManager: fileutil.NewFileManager(),
	}
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

// ExportOutputDir is a convenience method for copying sourceDir to destinationDir and then exporting the
// absolute destination dir with ExportOutput()
func (e *Exporter) ExportOutputDir(sourceDir, destinationDir, envKey string) error {
	absSourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}

	absDestinationDir, err := filepath.Abs(destinationDir)
	if err != nil {
		return err
	}

	if absSourceDir != absDestinationDir {
		if err := copyDir(absSourceDir, absDestinationDir); err != nil {
			return err
		}
	}

	return e.ExportOutput(envKey, absDestinationDir)
}

// ExportOutputFileContent ...
func (e *Exporter) ExportOutputFileContent(content, dst, envKey string) error {
	if err := e.fileManager.WriteBytes(dst, []byte(content)); err != nil {
		return err
	}

	return e.ExportOutputFile(envKey, dst, dst)
}

// ExportOutputFileContentAndReturnLastNLines ...
func (e *Exporter) ExportOutputFileContentAndReturnLastNLines(content, dst, envKey string, lines int) (string, error) {
	if err := e.ExportOutputFileContent(content, dst, envKey); err != nil {
		return "", err
	}

	return lastNLines(content, lines), nil
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

func runExport(cmd command.Command) error {
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("exporting output with envman failed: %s, output: %s", err, out)
	}
	return nil
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

func copyFilePreservingMode(src, dst string, info os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck

	// create destination file with same mode
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			log.Fatalf("Failed to close output file: %s", err)
		}
	}(out)

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return nil
}

func copySymlink(src, dst string) error {
	linkTarget, err := os.Readlink(src)
	if err != nil {
		return err
	}
	// create symlink at dst pointing to same target
	return os.Symlink(linkTarget, dst)
}

func copyDir(srcDir, dstDir string) error {
	srcDir = filepath.Clean(srcDir)
	dstDir = filepath.Clean(dstDir)

	info, err := os.Lstat(srcDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcDir)
	}

	// create destination root
	if err := os.MkdirAll(dstDir, info.Mode()); err != nil {
		return err
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstDir, rel)

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			// symlink: reproduce it
			if err := copySymlink(path, targetPath); err != nil {
				return err
			}
		case info.IsDir():
			// create directory with same permissions
			if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
				return err
			}
		default:
			// regular file
			if err := copyFilePreservingMode(path, targetPath, info); err != nil {
				return err
			}
		}
		return nil
	})
}

func lastNLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	// normalize CRLF to LF if needed
	if strings.Contains(s, "\r\n") {
		s = strings.ReplaceAll(s, "\r\n", "\n")
	}

	// skip trailing newlines so we don't count empty trailing lines
	i := len(s) - 1
	for i >= 0 && s[i] == '\n' {
		i--
	}
	if i < 0 {
		return "" // string was all newlines
	}

	// scan backward counting '\n' occurrences
	count := 0
	for ; i >= 0; i-- {
		if s[i] == '\n' {
			count++
			if count == n {
				// substring after this newline is the last n lines
				start := i + 1
				res := s[start:]
				// trim trailing whitespace (spaces, tabs, newlines, etc.)
				return strings.TrimRightFunc(res, func(r rune) bool {
					return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
				})
			}
		}
	}

	// fewer than n newlines => return whole string (trim trailing whitespace)
	return strings.TrimRightFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
	})
}
