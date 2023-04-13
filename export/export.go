package export

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/bitrise-io/go-utils/v2/archive"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/pathutil"
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
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("exporting output with envman failed: %s, output: %s", err, out)
	}
	return nil
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
func (e *Exporter) ExportOutputFilesZip(key string, sourcePaths []string, zipPath string, flatten bool) error {
	for _, path := range sourcePaths {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("one of the provided file paths is a directory: %s", path)
		}
	}

	err := archive.ZipFromFiles(sourcePaths, zipPath, flatten)
	if err != nil {

	}

	return e.ExportOutput(key, zipPath)
}

func (e *Exporter) ExportOutputDirZip(key string, dirPath string, zipPath string) error {
	err := archive.ZipFromDir(dirPath, zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip archive from %s: %w", dirPath, err)
	}

	return e.ExportOutput(key, zipPath)
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
			log.Fatalf(err.Error())
		}
	}(out)

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return nil
}
