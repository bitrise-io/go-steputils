package commandhelper

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/bitrise-io/go-steputils/output"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
)

// RawOutputCouldNotBeExporterError ...
type RawOutputCouldNotBeExporterError struct {
	wrappedError error
}

func (e RawOutputCouldNotBeExporterError) Error() string {
	return fmt.Sprintf("could not export output: %s", e.wrappedError)
}

// RunAndExportOutputWithReturningLastNLines ...
func RunAndExportOutputWithReturningLastNLines(cmd *command.Model, destinationPath, envKey string, lines int) (string, error) {
	var outBuffer bytes.Buffer
	preStdout := cmd.GetCmd().Stdout
	preStderr := cmd.GetCmd().Stderr
	cmd.SetStdout(&outBuffer)
	cmd.SetStderr(&outBuffer)

	defer func() {
		cmd.SetStdout(preStdout)
		cmd.SetStderr(preStderr)
	}()

	cmdError := cmd.Run()
	rawOutput := outBuffer.String()

	lastLines, err := output.ExportOutputFileContentAndReturnLastNLines(rawOutput, destinationPath, envKey, lines)
	if err != nil {
		return "", RawOutputCouldNotBeExporterError{err}
	}

	return lastLines, cmdError
}

// RunAndExportOutput ...
func RunAndExportOutput(cmd *command.Model, destinationPath, envKey string, lines int) error {
	outputLines, cmdErr := RunAndExportOutputWithReturningLastNLines(cmd, destinationPath, envKey, lines)

	var exportErr *RawOutputCouldNotBeExporterError
	if errors.As(cmdErr, &exportErr) {
		log.Warnf("Failed to export %s, error: %s", envKey, exportErr)
		cmdErr = nil
	}

	if lines > 0 {
		lastLines := "You can find the last couple of lines of output below.:"
		if cmdErr != nil {
			log.Errorf(lastLines)
		} else {
			log.Infof(lastLines)
		}

		log.Printf(outputLines)

		if cmdErr != nil {
			log.Warnf("If you can't find the reason of the error in the log, please check the %s.", destinationPath)
		}
	}

	log.Infof(colorstring.Magenta(fmt.Sprintf(`The log file is stored in %s, and its full path is available in the $%s environment variable.`, destinationPath, envKey)))

	return cmdErr
}
