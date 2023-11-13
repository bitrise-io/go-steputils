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

type acknowledgeRequest struct {
	Successful bool     `json:"successful"`
	Etags      []string `json:"etags"`
}

type uploadURL struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

type prepareUploadResponse struct {
	ID                       string      `json:"id"`
	UploadURLs               []uploadURL `json:"urls"`
	UploadChunkSizeBytes     int         `json:"chunk_size_bytes"`
	UploadChunkCount         int         `json:"chunk_count"`
	UploadLastChunkSizeBytes int         `json:"last_chunk_size_bytes"`
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
	url := fmt.Sprintf("%s/multipart-upload", c.baseURL)

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

// Data can be either byte[] or io.ReaderSeeker
func (c apiClient) uploadArchiveChunk(uploadURL uploadURL, data interface{}, size int64) (string, error) {

	switch body := data.(type) {
	case []byte, io.ReadSeeker:
		// do nothing
	default:
		return "", fmt.Errorf("invalid body type: %T", body)
	}

	req, err := retryablehttp.NewRequest(uploadURL.Method, uploadURL.URL, data)
	if err != nil {
		return "", err
	}
	for k, v := range uploadURL.Headers {
		req.Header.Set(k, v)
	}

	// Add Content-Length header manually because retryablehttp doesn't do it automatically
	req.Header.Set("Content-Length", fmt.Sprintf("%d", size))
	req.ContentLength = size

	dump, err := httputil.DumpRequest(req.Request, false)
	if err != nil {
		c.logger.Warnf("error while dumping request: %s", err)
	}
	c.logger.Debugf("Chunk request dump: %s", string(dump))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
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
	c.logger.Debugf("Chunk response dump: %s", string(dump))

	if resp.StatusCode != http.StatusOK {
		return "", unwrapError(resp)
	}

	etag := resp.Header.Get("ETag")

	return etag, nil
}

func (c apiClient) uploadArchive(archivePath string, chunkSize, chunkCount, lastChunkSize int, uploadURLs []uploadURL) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %s", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			c.logger.Errorf("failed to close file: %s", err)
		}
	}(file)

	etags := make([]string, 0, chunkCount)

	if chunkCount == 1 {
		fileInfo, err := os.Stat(archivePath)
		if err != nil {
			return nil, fmt.Errorf("stat file: %s", err)
		}
		c.logger.Debugf("Uploading single chunk (non multipart upload)", uploadURLs[0].URL)

		_, err = c.uploadArchiveChunk(uploadURLs[0], file, fileInfo.Size())
		if err != nil {
			return nil, fmt.Errorf("upload single chunk (non multipart): %s", err)
		}
		return nil, nil
	}

	c.logger.Debugf("Uploading %d chunks, %dB each", chunkCount, chunkSize)

	for i := 0; i < chunkCount; i++ {
		chunkData, err := io.ReadAll(io.NewSectionReader(file, int64(i)*int64(chunkSize), int64(chunkSize)))
		if err != nil {
			return nil, fmt.Errorf("read chunk: %s", err)
		}

		if i < chunkCount-1 && len(chunkData) != chunkSize {
			c.logger.Warnf("chunk size mismatch, expected %d, got %d", chunkSize, len(chunkData))
		}
		if i == chunkCount-1 && len(chunkData) != lastChunkSize {
			c.logger.Warnf("last chunk size mismatch, expected %d, got %d", lastChunkSize, len(chunkData))
		}

		c.logger.Debugf("Uploading chunk %d to %s", i, uploadURLs[i].URL)
		etag, err := c.uploadArchiveChunk(uploadURLs[i], chunkData, int64(len(chunkData)))
		if err != nil {
			return nil, fmt.Errorf("upload chunk part %d: %s", i, err)
		}
		etags = append(etags, etag)
	}

	return etags, nil
}

func (c apiClient) acknowledgeUpload(successful bool, uploadID string, partTags []string) (acknowledgeResponse, error) {
	url := fmt.Sprintf("%s/multipart-upload/%s/acknowledge", c.baseURL, uploadID)

	body, err := json.Marshal(acknowledgeRequest{
		Successful: successful,
		Etags:      partTags,
	})
	if err != nil {
		return acknowledgeResponse{}, err
	}

	req, err := retryablehttp.NewRequest(http.MethodPatch, url, body)
	if err != nil {
		return acknowledgeResponse{}, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))

	dump, err := httputil.DumpRequest(req.Request, true)
	if err != nil {
		c.logger.Warnf("error while dumping request: %s", err)
	}
	c.logger.Debugf("Acknowledge request dump: %s", string(dump))

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

	dump, err = httputil.DumpResponse(resp, true)
	if err != nil {
		c.logger.Warnf("error while dumping response: %s", err)
	}
	c.logger.Debugf("Acknowledge response dump: %s", string(dump))

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

func (c apiClient) downloadArchive(url string) (io.ReadCloser, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer func(body io.ReadCloser) {
			err := body.Close()
			if err != nil {
				c.logger.Printf(err.Error())
			}
		}(resp.Body)
		return nil, unwrapError(resp)
	}

	return resp.Body, nil
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
