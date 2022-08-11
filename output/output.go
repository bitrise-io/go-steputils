package output

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-io/go-utils/ziputil"
)

type Exporter struct {
	cmdFactory command.Factory
}

func NewExporter(cmdFactory command.Factory) Exporter {
	return Exporter{cmdFactory: cmdFactory}
}

// ExportOutput ...
func (e *Exporter) ExportOutput(key, value string) error {
	opts := command.Opts{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  strings.NewReader(value),
	}
	cmd := e.cmdFactory.Create("envman", []string{"add", "--key", key}, &opts)
	return cmd.Run()
}

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

const (
	filesType              = "files"
	foldersType            = "folders"
	mixedFileAndFolderType = "mixed"
)

func getInputType(sourcePths []string) (string, error) {
	var folderCount, fileCount int

	for _, path := range sourcePths {
		exist, err := pathutil.NewPathChecker().IsPathExists(path)
		if err != nil {
			return "", err
		}

		if exist {
			folderCount++
			continue
		}

		exist, err = pathutil.NewPathChecker().IsPathExists(path)
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
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
