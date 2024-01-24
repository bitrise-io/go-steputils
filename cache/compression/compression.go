package compression

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/klauspost/compress/zstd"
)

// ArchiveDependencyChecker ...
//
//go:generate moq -stub -out archive_dependency_checker_mock.go . ArchiveDependencyChecker
type ArchiveDependencyChecker interface {
	CheckDependencies() bool
}

// DependencyChecker ...
type DependencyChecker struct {
	logger  log.Logger
	envRepo env.Repository
}

// NewDependencyChecker ...
func NewDependencyChecker(logger log.Logger, envRepo env.Repository) *DependencyChecker {
	return &DependencyChecker{
		logger:  logger,
		envRepo: envRepo,
	}
}

// CheckDependencies ...
func (dc *DependencyChecker) CheckDependencies() bool {
	return dc.checkDepdendency("tar") && dc.checkDepdendency("zstd")
}

func (dc *DependencyChecker) checkDepdendency(binaryName string) bool {
	cmdFactory := command.NewFactory(dc.envRepo)
	cmd := cmdFactory.Create("which", []string{binaryName}, nil)
	dc.logger.Debugf("$ %s", cmd.PrintableCommandArgs())

	_, err := cmd.RunAndReturnTrimmedCombinedOutput()
	return err == nil
}

// Archiver ...
type Archiver struct {
	logger                   log.Logger
	envRepo                  env.Repository
	archiveDependencyChecker ArchiveDependencyChecker
}

// NewArchiver ...
func NewArchiver(logger log.Logger, envRepo env.Repository, archiveDependencyChecker ArchiveDependencyChecker) *Archiver {
	return &Archiver{
		logger:                   logger,
		envRepo:                  envRepo,
		archiveDependencyChecker: archiveDependencyChecker,
	}
}

// Compress creates a compressed archive from the provided files and folders using absolute paths.
func (a *Archiver) Compress(archivePath string, includePaths []string) error {
	haveZstdAndTar := a.archiveDependencyChecker.CheckDependencies()

	if !haveZstdAndTar {
		a.logger.Infof("Falling back to native implementation of zstd.")
		if err := a.compressWithGoLib(archivePath, includePaths); err != nil {
			return fmt.Errorf("compress files: %w", err)
		}
		return nil
	}

	a.logger.Infof("Using installed zstd binary")
	if err := a.compressWithBinary(archivePath, includePaths); err != nil {
		return fmt.Errorf("compress files: %w", err)
	}
	return nil
}

// Decompress takes an archive path and extracts files. This assumes an archive created with absolute file paths.
func (a *Archiver) Decompress(archivePath string, destinationDirectory string) error {
	haveZstdAndTar := a.archiveDependencyChecker.CheckDependencies()
	if !haveZstdAndTar {
		a.logger.Infof("Falling back to native implementation of zstd.")
		if err := a.decompressWithGolib(archivePath, destinationDirectory); err != nil {
			return fmt.Errorf("decompress files: %w", err)
		}
		return nil
	}

	a.logger.Infof("Using installed zstd binary")
	if err := a.decompressWithBinary(archivePath, destinationDirectory); err != nil {
		return fmt.Errorf("decompress files: %w", err)
	}
	return nil
}

func (a *Archiver) compressWithGoLib(archivePath string, includePaths []string) error {
	var buf bytes.Buffer

	for _, p := range includePaths {
		zstdWriter, err := zstd.NewWriter(&buf)
		if err != nil {
			return fmt.Errorf("create zstd writer: %w", err)
		}
		tw := tar.NewWriter(zstdWriter)

		path := filepath.Clean(p)
		// walk through every file in the folder
		if err := filepath.Walk(path, func(file string, fi os.FileInfo, e error) error {
			// generate tar header
			header, err := tar.FileInfoHeader(fi, file)
			if err != nil {
				return fmt.Errorf("create file info header: %w", err)
			}

			path := filepath.Clean(file)
			header.Name = path

			var link string
			if fi.Mode()&os.ModeSymlink != 0 {
				if link, err = os.Readlink(file); err != nil {
					return fmt.Errorf("read symlink: %w", err)
				}
			}
			if link != "" {
				header.Typeflag = tar.TypeSymlink
				header.Linkname = link
			}

			// write header
			if err := tw.WriteHeader(header); err != nil {
				return fmt.Errorf("write tar file header: %w", err)
			}

			// nothing more to do for non-regular files or directories
			if !fi.Mode().IsRegular() || fi.IsDir() {
				return nil
			}

			data, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("open file: %w", err)
			}
			if _, err := io.Copy(tw, data); err != nil {
				return fmt.Errorf("copy to file: %w", err)
			}
			if err := data.Close(); err != nil {
				return fmt.Errorf("close file: %w", err)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("iterate on files: %w", err)
		}

		// produce tar
		if err := tw.Close(); err != nil {
			return fmt.Errorf("close tar writer: %w", err)
		}
		// produce zstd
		if err := zstdWriter.Close(); err != nil {
			return fmt.Errorf("close zstd writer: %w", err)
		}
	}

	// write the archive file
	fileToWrite, err := os.OpenFile(archivePath, os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}
	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		return fmt.Errorf("write arhive file: %w", err)
	}
	if err := fileToWrite.Close(); err != nil {
		return fmt.Errorf("close archive file: %w", err)
	}

	return nil
}

func (a *Archiver) compressWithBinary(archivePath string, includePaths []string) error {
	cmdFactory := command.NewFactory(a.envRepo)

	/*
		tar arguments:
		--use-compress-program: Pipe the output to zstd instead of using the built-in gzip compression
		-P: Alias for --absolute-paths in BSD tar and --absolute-names in GNU tar (step runs on both Linux and macOS)
			Storing absolute paths in the archive allows paths outside the current directory (such as ~/.gradle)
		-c: Create archive
		-f: Output file
	*/
	tarArgs := []string{
		"--use-compress-program", "zstd --threads=0", // Use CPU count threads
		"-P",
		"-c",
		"-f", archivePath,
	}
	tarArgs = append(tarArgs, includePaths...)

	cmd := cmdFactory.Create("tar", tarArgs, nil)

	a.logger.Debugf("$ %s", cmd.PrintableCommandArgs())

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("command failed with exit status %d (%s):\n%w", exitErr.ExitCode(), cmd.PrintableCommandArgs(), errors.New(out))
		}
		return fmt.Errorf("executing command failed (%s): %w", cmd.PrintableCommandArgs(), err)
	}

	return nil
}

func (a *Archiver) decompressWithGolib(archivePath string, destinationDirectory string) error {
	compressedFile, err := os.OpenFile(archivePath, os.O_RDWR, 0777)
	if err != nil {
		return fmt.Errorf("read file %s: %w", archivePath, err)
	}

	zr, err := zstd.NewReader(compressedFile)
	if err != nil {
		return fmt.Errorf("create zstd reader: %w", err)
	}

	tr := tar.NewReader(zr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar file: %w", err)
		}

		target := filepath.ToSlash(header.Name)

		if destinationDirectory != "" {
			target = filepath.Join(destinationDirectory, target)
		}

		switch header.Typeflag {
		// if its a dir and it doesn't exist create it (with 0755 permission)
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return fmt.Errorf("create target directories: %w", err)
				}
			}
		// if it's a file create it (with same permission)
		case tar.TypeReg:
			fileToWrite, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			// copy over contents
			if _, err := io.Copy(fileToWrite, tr); err != nil {
				return fmt.Errorf("copy content to file: %w", err)
			}
			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			if err := fileToWrite.Close(); err != nil {
				return fmt.Errorf("write file: %w", err)
			}
		case tar.TypeSymlink:
			err = os.Symlink(header.Linkname, target)
			if err != nil {
				return fmt.Errorf("symlink file: %w", err)
			}

		}
	}
	return nil
}

func (a *Archiver) decompressWithBinary(archivePath string, destinationDirectory string) error {
	commandFactory := command.NewFactory(a.envRepo)

	/*
		tar arguments:
		--use-compress-program: Pipe the input to zstd instead of using the built-in gzip compression
		-P: Alias for --absolute-paths in BSD tar and --absolute-names in GNU tar (step runs on both Linux and macOS)
			Storing absolute paths in the archive allows paths outside the current directory (such as ~/.gradle)
		-x: Extract archive
		-f: Output file
	*/
	decompressTarArgs := []string{
		"--use-compress-program", "zstd -d",
		"-x",
		"-f", archivePath,
		"-P",
	}

	if destinationDirectory != "" {
		decompressTarArgs = append(decompressTarArgs, "--directory", destinationDirectory)
	}

	cmd := commandFactory.Create("tar", decompressTarArgs, nil)
	a.logger.Debugf("$ %s", cmd.PrintableCommandArgs())

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("command failed with exit status %d (%s):\n%w", exitErr.ExitCode(), cmd.PrintableCommandArgs(), errors.New(out))
		}
		return fmt.Errorf("executing command failed (%s): %w", cmd.PrintableCommandArgs(), err)
	}

	return nil
}

// AreAllPathsEmpty checks if the provided paths are all nonexistent files or empty directories
func AreAllPathsEmpty(includePaths []string) bool {
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
