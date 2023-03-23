//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/cache/network"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-retryablehttp"
)

func TestUpload(t *testing.T) {
	// Given
	cacheKey := "integration-test"
	baseURL := os.Getenv("BITRISEIO_ABCS_API_URL")
	token := os.Getenv("BITRISEIO_BITRISE_SERVICES_ACCESS_TOKEN")
	testFile := "testdata/single-item.tzst"
	params := network.UploadParams{
		APIBaseURL:  baseURL,
		Token:       token,
		ArchivePath: testFile,
		ArchiveSize: 468,
		CacheKey:    cacheKey,
	}

	logger.EnableDebugLog(true)

	// When
	err := network.Upload(params, logger)

	// Then
	assert.NoError(t, err)

	bytes, err := ioutil.ReadFile(testFile)
	if err != nil {
		t.Errorf(err.Error())
	}
	expectedChecksum := checksumOf(bytes)
	checksum, err := downloadArchive(cacheKey, baseURL, token)
	if err != nil {
		t.Errorf(err.Error())
	}
	assert.Equal(t, expectedChecksum, checksum)
}

// downloadArchive downloads the archive from the API based on cacheKey and returns its SHA256 checksum
func downloadArchive(cacheKey string, baseURL string, token string) (string, error) {
	client := retryablehttp.NewClient()

	// Obtain pre-signed download URL
	url := fmt.Sprintf("%s/restore?cache_keys=%s", baseURL, cacheKey)
	req, err := retryablehttp.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	var parsedResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&parsedResp)
	if err != nil {
		return "", err
	}
	downloadURL := parsedResp["url"].(string)

	// Download archive using pre-signed URL
	req2, err := retryablehttp.NewRequest(http.MethodGet, downloadURL, nil)
	req2.Header.Set("Content-Type", "application/octet-stream")
	resp2, err := client.Do(req2)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		logger.Errorf("HTTP status code: %d", resp2.StatusCode)
		errorResp, err := ioutil.ReadAll(resp2.Body)
		if err != nil {
			return "", err
		}
		logger.Errorf("Error response: %s", errorResp)
		return "", err
	}

	bytes, err := ioutil.ReadAll(resp2.Body)
	if err != nil {
		return "", err
	}

	return checksumOf(bytes), nil
}
