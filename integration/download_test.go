//go:build integration
// +build integration

package integration

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/cache/network"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/assert"
)

func TestSuccessfulDownload(t *testing.T) {
	// Given
	cacheKeys := []string{
		"cache-key-v2",
		"cache-key",
		"key:with&strange?characters<>",
		strings.Repeat("cache-key-", 60),
	}
	baseURL := os.Getenv("BITRISEIO_ABCS_API_URL")
	token := os.Getenv("BITRISEIO_BITRISE_SERVICES_ACCESS_TOKEN")
	testFile := "testdata/single-item.tzst"

	err := uploadArchive(cacheKeys[0], testFile, baseURL, token)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// When
	downloadPath := filepath.Join(t.TempDir(), "cache-test.tzst")
	params := network.DownloadParams{
		APIBaseURL:   baseURL,
		Token:        token,
		CacheKeys:    cacheKeys,
		DownloadPath: downloadPath,
	}
	logger.EnableDebugLog(true)
	matchedKey, err := network.Download(params, logger)

	// Then
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, cacheKeys[0], matchedKey)

	testFileBytes, err := ioutil.ReadFile(testFile)
	if err != nil {
		t.Fatalf(err.Error())
	}
	downloadedFileBytes, err := ioutil.ReadFile(downloadPath)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expectedChecksum := checksumOf(testFileBytes)
	checksum := checksumOf(downloadedFileBytes)
	assert.Equal(t, expectedChecksum, checksum)
}

func TestNotFoundDownload(t *testing.T) {
	// Given
	cacheKeys := []string{
		fmt.Sprintf("no-cache-for-this-%d", rand.Int()),
	}
	baseURL := os.Getenv("BITRISEIO_ABCS_API_URL")
	token := os.Getenv("BITRISEIO_BITRISE_SERVICES_ACCESS_TOKEN")

	// When
	downloadPath := filepath.Join(t.TempDir(), "cache-test.tzst")
	params := network.DownloadParams{
		APIBaseURL:   baseURL,
		Token:        token,
		CacheKeys:    cacheKeys,
		DownloadPath: downloadPath,
	}
	logger.EnableDebugLog(true)
	matchedKey, err := network.Download(params, logger)

	// Then
	assert.Equal(t, "", matchedKey)
	assert.ErrorIs(t, err, network.ErrCacheNotFound)
}

func uploadArchive(cacheKey, path, baseURL, token string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	params := network.UploadParams{
		APIBaseURL:  baseURL,
		Token:       token,
		ArchivePath: path,
		ArchiveSize: fileInfo.Size(),
		CacheKey:    cacheKey,
	}
	logger := log.NewLogger()
	logger.EnableDebugLog(true)
	err = network.Upload(params, logger)
	if err != nil {
		return err
	}

	return nil
}
