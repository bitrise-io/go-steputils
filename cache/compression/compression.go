package compression

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
)

// Compress creates a compressed archive from the provided files and folders using absolute paths.
func Compress(archivePath string, includePaths []string, logger log.Logger, envRepo env.Repository) error {
	cmdFactory := command.NewFactory(envRepo)

	tarArgs := []string{
		"--use-compress-program",
		"zstd --threads=0", // Use CPU count threads
		"-P",               // Same as --absolute-paths in BSD tar, --absolute-names in GNU tar
		"-cf",
		archivePath,
		"--directory",
		envRepo.Get("BITRISE_SOURCE_DIR"),
	}
	tarArgs = append(tarArgs, includePaths...)

	cmd := cmdFactory.Create("tar", tarArgs, nil)

	logger.Debugf("$ %s", cmd.PrintableCommandArgs())

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		logger.Errorf("Compression command failed: %s", out)
		return err
	}

	return nil
}

// Decompress takes an archive path and extracts files. This assumes an archive created with absolute file paths.
func Decompress(archivePath string, logger log.Logger, envRepo env.Repository, additionalArgs ...string) error {
	commandFactory := command.NewFactory(envRepo)

	decompressTarArgs := []string{
		"--use-compress-program",
		"zstd -d",
		"-xf",
		archivePath,
		"-P", // Same as --absolute-paths in BSD tar, --absolute-names in GNU tar
	}

	if len(additionalArgs) > 0 {
		decompressTarArgs = append(decompressTarArgs, additionalArgs...)
	}

	cmd := commandFactory.Create("tar", decompressTarArgs, nil)
	logger.Debugf("$ %s", cmd.PrintableCommandArgs())

	output, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		logger.Errorf("Failed to decompress cache archive: %s", output)
		return err
	}

	return nil
}

func areAllPathsEmpty(includePaths []string) bool {
	allEmpty := true

	for _, path := range includePaths {
		// Check if file exists at path
		fileInfo, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			// File doesn't exist
			continue
		}

		// Check if it's a directory
		if !fileInfo.IsDir() {
			// Is a file and it exists
			allEmpty = false
			break
		}

		file, err := os.Open(path)
		if err != nil {
			continue
		}
		_, err = file.Readdirnames(1) // query only 1 child
		if errors.Is(err, io.EOF) {
			// Dir is empty
			continue
		}
		if err == nil {
			// Dir has files or dirs
			allEmpty = false
			break
		}
	}

	return allEmpty
}
