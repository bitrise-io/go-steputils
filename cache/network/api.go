package network

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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
	ChunkSizeMB        int    `json:"chunk_size_mb,omitempty"` // optional chunk size in MB, default 32MB if not set
}

type prepareMultipartUploadResponse struct {
	ID                 string                      `json:"id"`
	ChunkSizeBytes     int64                       `json:"chunk_size_bytes"`
	ChunkCount         int64                       `json:"chunk_count"`
	LastChunkSizeBytes int64                       `json:"last_chunk_size_bytes"`
	URLs               []prepareMultipartUploadURL `json:"urls"`
}

type prepareMultipartUploadURL struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type completeMultipartUploadRequest struct {
	Successful bool     `json:"successful"`
	Etags      []string `json:"etags,omitempty"`
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

func (c apiClient) prepareMultipartUpload(requestBody prepareUploadRequest) (prepareMultipartUploadResponse, error) {
	url := fmt.Sprintf("%s/multipart-upload", c.baseURL)

	body, err := json.Marshal(requestBody)
	if err != nil {
		return prepareMultipartUploadResponse{}, err
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return prepareMultipartUploadResponse{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return prepareMultipartUploadResponse{}, err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			c.logger.Printf(err.Error())
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return prepareMultipartUploadResponse{}, unwrapError(resp)
	}

	var response prepareMultipartUploadResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return prepareMultipartUploadResponse{}, err
	}

	return response, nil
}

func (c apiClient) completeMultipartUpload(uploadID string, etags []string) (acknowledgeResponse, error) {
	resp, err := c.acknowledgeMultipartUpload(uploadID, true, etags)
	if err != nil {
		return acknowledgeResponse{}, fmt.Errorf("complete multipart upload: %w", err)
	}
	return resp, nil
}

func (c apiClient) abortMultipartUpload(uploadID string) error {
	_, err := c.acknowledgeMultipartUpload(uploadID, false, nil)
	if err != nil {
		return fmt.Errorf("abort multipart upload: %w", err)
	}
	return nil
}

func (c apiClient) acknowledgeMultipartUpload(uploadID string, successful bool, etags []string) (acknowledgeResponse, error) {
	url := fmt.Sprintf("%s/multipart-upload/%s/acknowledge", c.baseURL, uploadID)

	requestBody := completeMultipartUploadRequest{
		Successful: successful,
		Etags:      etags,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return acknowledgeResponse{}, err
	}

	req, err := retryablehttp.NewRequest(http.MethodPatch, url, body)
	if err != nil {
		return acknowledgeResponse{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	req.Header.Set("Content-type", "application/json")

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
