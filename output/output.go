package output

import (
	"fmt"
	"path/filepath"

	v1_command "github.com/bitrise-io/go-utils/command"
	v1_fileutil "github.com/bitrise-io/go-utils/fileutil"
	v1_pathutil "github.com/bitrise-io/go-utils/pathutil"
	v1_stringutil "github.com/bitrise-io/go-utils/stringutil"
	v1_ziputil "github.com/bitrise-io/go-utils/ziputil"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	//"github.com/bitrise-io/go-utils/v2/stringutil"
	//"github.com/bitrise-io/go-utils/v2/ziputil"
)

func temp() {
	_ = v1_command.New("echo", "hello")
	_, _ = v1_pathutil.AbsPath("")
	_ = v1_fileutil.WriteStringToFile("", "")
	_ = v1_stringutil.LastNLines("", 3)
	_ = v1_ziputil.ZipFiles([]string{}, "")
}

// ExportOutputDir ...
func ExportOutputDir(sourceDir, destinationDir, envKey string) error {
	absSourceDir, err := pathutil.AbsPath(sourceDir)
	if err != nil {
		return err
	}

	absDestinationDir, err := pathutil.AbsPath(destinationDir)
	if err != nil {
		return err
	}

	if absSourceDir != absDestinationDir {
		if err := command.CopyDir(absSourceDir, absDestinationDir, true); err != nil {
			return err
		}
	}
	return tools.ExportEnvironmentWithEnvman(envKey, absDestinationDir)
}

// ExportOutputFile ...
func ExportOutputFile(sourcePth, destinationPth, envKey string) error {
	absSourcePth, err := pathutil.AbsPath(sourcePth)
	if err != nil {
		return err
	}

	absDestinationPth, err := pathutil.AbsPath(destinationPth)
	if err != nil {
		return err
	}

	if absSourcePth != absDestinationPth {
		if err := command.CopyFile(absSourcePth, absDestinationPth); err != nil {
			return err
		}
	}
	return tools.ExportEnvironmentWithEnvman(envKey, absDestinationPth)
}

// ExportOutputFileContent ...
func ExportOutputFileContent(content, destinationPth, envKey string) error {
	if err := fileutil.WriteStringToFile(destinationPth, content); err != nil {
		return err
	}

	return ExportOutputFile(destinationPth, destinationPth, envKey)
}

// ExportOutputFileContentAndReturnLastNLines ...
func ExportOutputFileContentAndReturnLastNLines(content, destinationPath, envKey string, lines int) (string, error) {
	if err := fileutil.WriteStringToFile(destinationPath, content); err != nil {
		return "", err
	}

	if err := ExportOutputFile(destinationPath, destinationPath, envKey); err != nil {
		return "", err
	}

	return v1_stringutil.LastNLines(content, lines), nil
}

// ZipAndExportOutput ...
func ZipAndExportOutput(sourcePths []string, destinationZipPth, envKey string) error {
	tmpZipFilePth, err := zipFilePath()
	if err != nil {
		return err
	}

	inputType, err := getInputType(sourcePths)
	if err != nil {
		return err
	}

	// We have separate zip functions for files and folders and that is the main reason we cannot have mixed
	// paths (files and also folders) in the input. It has to be either folders or files. Everything
	// else leads to an error.
	switch inputType {
	case filesType:
		err = v1_ziputil.ZipFiles(sourcePths, tmpZipFilePth)
	case foldersType:
		err = v1_ziputil.ZipDirs(sourcePths, tmpZipFilePth)
	case mixedFileAndFolderType:
		return fmt.Errorf("source path list (%s) contains a mix of files and folders", sourcePths)
	default:
		return fmt.Errorf("source path list (%s) is empty", sourcePths)
	}

	if err != nil {
		return err
	}

	return ExportOutputFile(tmpZipFilePth, destinationZipPth, envKey)
}

func zipFilePath() (string, error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("__export_tmp_dir__")
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
		exist, err := pathutil.IsDirExists(path)
		if err != nil {
			return "", err
		}

		if exist {
			folderCount++
			continue
		}

		exist, err = pathutil.IsPathExists(path)
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
