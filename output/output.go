package output

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/stringutil"
	"github.com/bitrise-io/go-utils/ziputil"
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

// ZipAndExportOutput ...
func ZipAndExportOutput(sourcePth, destinationZipPth, envKey string) error {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("__export_tmp_dir__")
	if err != nil {
		return err
	}

	base := filepath.Base(sourcePth)
	tmpZipFilePth := filepath.Join(tmpDir, base+".zip")

	if exist, err := pathutil.IsDirExists(sourcePth); err != nil {
		return err
	} else if exist {
		if err := ziputil.ZipDir(sourcePth, tmpZipFilePth, false); err != nil {
			return err
		}
	} else if exist, err := pathutil.IsPathExists(sourcePth); err != nil {
		return err
	} else if exist {
		if err := ziputil.ZipFile(sourcePth, tmpZipFilePth); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("source path (%s) not exists", sourcePth)
	}

	return ExportOutputFile(tmpZipFilePth, destinationZipPth, envKey)
}

// RunAndExportOutput runs a command.Model and directs its ouput to a log file
// and keeps only the last N lines of the output.
func RunAndExportOutput(cmd *command.Model, outputPath, envkey string, outputLastLinesOfCode int) error {
	var outBuffer bytes.Buffer
	cmd.SetStdout(&outBuffer)
	cmd.SetStderr(&outBuffer)

	cmdError := cmd.Run()
	rawOutput := outBuffer.String()

	if err := ExportOutputFileContent(rawOutput, outputPath, envkey); err != nil {
		log.Warnf("Failed to export %s, error: %s", envkey, err)
		return cmdError
	}

	if outputLastLinesOfCode > 0 {
		log.Infof(colorstring.Magenta(fmt.Sprintf(`You can find the last couple of lines of output below.`)))

		lastLines := "Last lines of the output:"
		if cmdError != nil {
			log.Errorf(lastLines)
		} else {
			log.Infof(lastLines)
		}
		fmt.Println(stringutil.LastNLines(rawOutput, outputLastLinesOfCode))

		if cmdError != nil {
			log.Warnf("If you can't find the reason of the error in the log, please check the %s.", outputPath)
		}
	}

	log.Infof(colorstring.Magenta(fmt.Sprintf(`The log file is stored in %s, and its full path is available in the $%s environment variable.`, outputPath, envkey)))

	return cmdError
}
