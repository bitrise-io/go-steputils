package network

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/hashicorp/go-retryablehttp"
)

const maxKeyLength = 512
const maxKeyCount = 8

type prepareUploadRequest struct {
	CacheKey           string `json:"cache_key"`
	ArchiveFileName    string `json:"archive_filename"`
	ArchiveContentType string `json:"archive_content_type"`
	ArchiveSizeInBytes int64  `json:"archive_size_in_bytes"`
}

type prepareUploadResponse struct {
	ID            string            `json:"id"`
	UploadMethod  string            `json:"method"`
	UploadURL     string            `json:"url"`
	UploadHeaders map[string]string `json:"headers"`
}

type acknowledgeResponse struct {
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type restoreResponse struct {
	URL        string `json:"url"`
	MatchedKey string `json:"matched_cache_key"`
}

type apiClient struct {
	httpClient  *retryablehttp.Client
	baseURL     string
	accessToken string
	logger      log.Logger
}

func newAPIClient(client *retryablehttp.Client, baseURL string, accessToken string, logger log.Logger) apiClient {
	return apiClient{
		httpClient:  client,
		baseURL:     baseURL,
		accessToken: accessToken,
		logger:      logger,
	}
}

func (c apiClient) prepareUpload(requestBody prepareUploadRequest) (prepareUploadResponse, error) {
	url := fmt.Sprintf("%s/upload", c.baseURL)

	body, err := json.Marshal(requestBody)
	if err != nil {
		return prepareUploadResponse{}, err
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return prepareUploadResponse{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return prepareUploadResponse{}, err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			c.logger.Printf(err.Error())
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return prepareUploadResponse{}, unwrapError(resp)
	}

	var response prepareUploadResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return prepareUploadResponse{}, err
	}

	return response, nil
}

func (c apiClient) uploadArchive(archivePath, uploadMethod, uploadURL string, headers map[string]string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}

	req, err := retryablehttp.NewRequest(uploadMethod, uploadURL, file)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Add Content-Length header manually because retryablehttp doesn't do it automatically
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	req.ContentLength = fileInfo.Size()

	dump, err := httputil.DumpRequest(req.Request, false)
	if err != nil {
		c.logger.Warnf("error while dumping request: %s", err)
	}
	c.logger.Debugf("Request dump: %s", string(dump))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			c.logger.Printf(err.Error())
		}
	}(resp.Body)

	dump, err = httputil.DumpResponse(resp, true)
	if err != nil {
		c.logger.Warnf("error while dumping response: %s", err)
	}
	c.logger.Debugf("Response dump: %s", string(dump))

	if resp.StatusCode != http.StatusOK {
		return unwrapError(resp)
	}

	return nil
}

func (c apiClient) acknowledgeUpload(uploadID string) (acknowledgeResponse, error) {
	url := fmt.Sprintf("%s/upload/%s/acknowledge", c.baseURL, uploadID)

	req, err := retryablehttp.NewRequest(http.MethodPatch, url, nil)
	if err != nil {
		return acknowledgeResponse{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return acknowledgeResponse{}, err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			c.logger.Printf(err.Error())
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return acknowledgeResponse{}, unwrapError(resp)
	}

	var response acknowledgeResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return acknowledgeResponse{}, err
	}
	return response, nil
}

func (c apiClient) restore(cacheKeys []string) (restoreResponse, error) {
	keysInQuery, err := validateKeys(cacheKeys)
	if err != nil {
		return restoreResponse{}, err
	}
	apiURL := fmt.Sprintf("%s/restore?cache_keys=%s", c.baseURL, keysInQuery)

	req, err := retryablehttp.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return restoreResponse{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return restoreResponse{}, err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			c.logger.Printf(err.Error())
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return restoreResponse{}, ErrCacheNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return restoreResponse{}, unwrapError(resp)
	}

	var response restoreResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return restoreResponse{}, err
	}

	return response, nil
}

func unwrapError(resp *http.Response) error {
	errorResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errorResp)
}

func validateKeys(keys []string) (string, error) {
	if len(keys) > maxKeyCount {
		return "", fmt.Errorf("maximum number of keys is %d, %d provided", maxKeyCount, len(keys))
	}
	truncatedKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.Contains(key, ",") {
			return "", fmt.Errorf("commas are not allowed in keys (invalid key: %s)", key)
		}
		if len(key) > maxKeyLength {
			truncatedKeys = append(truncatedKeys, key[:maxKeyLength])
		} else {
			truncatedKeys = append(truncatedKeys, key)
		}
	}

	return url.QueryEscape(strings.Join(truncatedKeys, ",")), nil
}
