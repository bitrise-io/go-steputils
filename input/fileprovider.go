package input

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/pathutil"
)

const (
	fileSchema = "file://"
)

// FileDownloader ..
type FileDownloader interface {
	Get(destination, source string) error
}

// FileProvider supports retrieving the local path to a file either provided
// as a local path using `file://` scheme
// or downloading the file to a temporary location and return the path to it.
type FileProvider struct {
	path           string
	filedownloader FileDownloader
}

// NewFileProvider ...
func NewFileProvider(path string, filedownloader FileDownloader) FileProvider {
	return FileProvider{
		path:           path,
		filedownloader: filedownloader,
	}
}

// LocalPath ...
func (fileProvider FileProvider) LocalPath() (string, error) {

	var localPath string
	if strings.HasPrefix(fileProvider.path, fileSchema) {
		trimmedPath, err := fileProvider.trimmedFilePath()
		if err != nil {
			return "", err
		}
		localPath = trimmedPath
	} else {
		downloadedPath, err := fileProvider.downloadFile()
		if err != nil {
			return "", err
		}
		localPath = downloadedPath
	}

	return localPath, nil
}

// Removes file:// from the begining of the path
func (fileProvider FileProvider) trimmedFilePath() (string, error) {
	pth := strings.TrimPrefix(fileProvider.path, fileSchema)
	return pathutil.AbsPath(pth)
}

func (fileProvider FileProvider) downloadFile() (string, error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("FileProviderprovider")
	if err != nil {
		return "", err
	}

	fileName, err := fileProvider.fileNameFromPathURL()
	if err != nil {
		return "", err
	}
	localPath := path.Join(tmpDir, fileName)
	if err := fileProvider.filedownloader.Get(localPath, fileProvider.path); err != nil {
		return "", err
	}

	return localPath, nil
}

// Returns the file's name from a URL that starts with
// `http://` or `https://`
func (fileProvider FileProvider) fileNameFromPathURL() (string, error) {
	url, err := url.Parse(fileProvider.path)
	if err != nil {
		return "", err
	}

	return filepath.Base(url.Path), nil
}
