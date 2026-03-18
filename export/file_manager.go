package export

import "github.com/bitrise-io/go-utils/v2/fileutil"

// FileManager defines file management operations.
type FileManager = fileutil.FileManager

// SysStat holds file system stat information.
type SysStat = fileutil.SysStat

// NewFileManager creates a new FileManager instance.
func NewFileManager() FileManager {
	return fileutil.NewFileManager()
}
