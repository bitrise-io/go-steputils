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
	"sync"
	"time"

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

	fmt.Printf("Uploading archive chunk: %s \n\r", uploadURL.URL)
	fmt.Printf("Chunk size: %d \n\r", size)

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
	fmt.Printf("Etag %s for %s of size %v \n\r", etag, uploadURL.URL, size)

	return etag, nil
}

func (c apiClient) uploadArchive(archivePath string, chunkSize, chunkCount, lastChunkSize int, uploadURLs []uploadURL) ([]string, error) {

	fmt.Printf("Uploading archive: %s \n\r", archivePath)
	fmt.Printf("Chunk size, count, last chunk size: %d, %d, %d \n\r", chunkSize, chunkCount, lastChunkSize)
	fmt.Printf("Upload urls: %+v \n\r", uploadURLs)

	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %s", err)
	}
	defer file.Close()

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

	type Job struct {
		ChunkNumber int
		ChunkStart  int64
		ChunkSize   int64
		UploadURL   uploadURL
	}

	jobs := make(chan Job, chunkCount)
	results := make(chan struct {
		ChunkNumber int
		ChunkSize   int64
		ETag        string
	}, chunkCount)
	errors := make(chan error, chunkCount)
	wg := sync.WaitGroup{}

	worker := func() {
		for job := range jobs {
			time.Sleep(30 * time.Second) // Simulated processing delay

			chunkData, err := io.ReadAll(io.NewSectionReader(file, job.ChunkStart, job.ChunkSize))
			if err != nil {
				errors <- fmt.Errorf("read chunk %d: %s", job.ChunkNumber, err)
				wg.Done()
				continue
			}

			fmt.Printf("Uploading chunk %d to %s, size: %v, chunk start: %v \n\r", job.ChunkNumber, job.UploadURL.URL, job.ChunkSize, job.ChunkStart)
			etag, err := c.uploadArchiveChunk(job.UploadURL, chunkData, int64(len(chunkData)))
			if err != nil {
				fmt.Printf("Error uploading chunk %d to %s \n\r", job.ChunkNumber, job.UploadURL.URL)
				errors <- fmt.Errorf("upload chunk part %d: %s", job.ChunkNumber, err)
				wg.Done()
				continue
			}

			results <- struct {
				ChunkNumber int
				ChunkSize   int64
				ETag        string
			}{job.ChunkNumber, job.ChunkSize, etag}
			wg.Done()
		}
	}

	workerCount := 10 // Number of concurrent workers
	for w := 0; w < workerCount; w++ {
		go worker()
	}

	for i := 0; i < chunkCount; i++ {
		chunkStart := int64(i) * int64(chunkSize)
		chunkEnd := int64(chunkSize)
		if i == chunkCount-1 {
			chunkEnd = int64(lastChunkSize)
		}

		wg.Add(1)
		jobs <- Job{
			ChunkNumber: i,
			ChunkStart:  chunkStart,
			ChunkSize:   chunkEnd,
			UploadURL:   uploadURLs[i],
		}
	}
	close(jobs)

	wg.Wait()
	close(results)
	close(errors)

	fmt.Printf("Results: %+v \n\r", results)

	etags := make([]string, chunkCount)
	for result := range results {
		etags[result.ChunkNumber] = result.ETag
	}
	c.logger.Debugf("Etags: %+v to  \n\r", etags)

	fmt.Printf("Etags: %+v \n\r", etags)

	for err := range errors {
		if err != nil {
			return nil, err
		}
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
		fmt.Printf("Error marshalling acknowledge request: %s \n\r", err)
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
		fmt.Printf("Error acknowledging upload: %s \n\r", unwrapError(resp))
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
