package keytemplate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// checksum returns a hex-encoded SHA-256 checksum of one or multiple files. Each file path can contain glob patterns,
// including "doublestar" patterns (such as `**/*.gradle`).
// The path list is sorted alphabetically to produce consistent output.
// Errors are logged as warnings and an empty string is returned in that case.
func (m Model) checksum(paths ...string) string {
	workingDir, err := os.Getwd()
	if err != nil {
		m.logger.Errorf(err.Error())
		return ""
	}

	evaluatedPaths := m.evaluateGlobPatterns(workingDir, paths)
	files := filterFilesOnly(evaluatedPaths)
	m.logger.Debugf("Files included in checksum:")
	for _, path := range files {
		m.logger.Debugf("- %s", path)
	}

	if len(files) == 0 {
		m.logger.Warnf("No files to include in the checksum")
		return ""
	} else if len(files) == 1 {
		checksum, err := checksumOfFile(files[0])
		if err != nil {
			m.logger.Warnf("Error while computing checksum %s: %s", files[0], err)
			return ""
		}
		return hex.EncodeToString(checksum)
	}

	finalChecksum := sha256.New()
	sort.Strings(files)
	for _, path := range files {
		checksum, err := checksumOfFile(path)
		if err != nil {
			m.logger.Warnf("Error while hashing %s: %s", path, err)
			continue
		}

		finalChecksum.Write(checksum)
	}

	return hex.EncodeToString(finalChecksum.Sum(nil))
}

func (m Model) evaluateGlobPatterns(workingDir string, paths []string) []string {
	var finalPaths []string

	for _, path := range paths {
		if strings.Contains(path, "*") {
			matches, err := doublestar.Glob(os.DirFS(workingDir), path)
			if matches == nil {
				m.logger.Warnf("No match for pattern: %s", path)
				continue
			}
			if err != nil {
				m.logger.Warnf("Error in pattern '%s': %s", path, err)
				continue
			}
			finalPaths = append(finalPaths, matches...)
		} else {
			finalPaths = append(finalPaths, path)
		}
	}

	return finalPaths
}

func checksumOfFile(path string) ([]byte, error) {
	hash := sha256.New()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hash.Write(b)
	return hash.Sum(nil), nil
}

func filterFilesOnly(paths []string) []string {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		files = append(files, path)
	}
	return files
}
