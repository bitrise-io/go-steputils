package output

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-steputils/tools"
)

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

// Zip ...
func Zip(sourcePth, destinationZipPth string) error {
	parentDir := filepath.Dir(sourcePth)
	dirName := filepath.Base(sourcePth)
	cmd := command.New("/usr/bin/zip", "-rTy", destinationZipPth, dirName)
	cmd.SetDir(parentDir)
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to zip: %s, output: %s, error: %s", sourcePth, out, err)
	}
	return nil
}

// ZipAndExportOutput ...
func ZipAndExportOutput(sourcePth, destinationZipPth, envKey string) error {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("__export_tmp_dir__")
	if err != nil {
		return err
	}

	base := filepath.Base(sourcePth)
	tmpZipFilePth := filepath.Join(tmpDir, base+".zip")

	if err := Zip(sourcePth, tmpZipFilePth); err != nil {
		return err
	}

	return ExportOutputFile(tmpZipFilePth, destinationZipPth, envKey)
}
