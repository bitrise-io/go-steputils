package network

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-steputils/v2/cache/kv"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
)

// UploadParams ...
type UploadParams struct {
	APIBaseURL     string
	Token          string
	ArchivePath    string
	ArchiveSize    int64
	CacheKey       string
	BuildCacheHost string
	InsecureGRPC   bool
	AppSlug        string
	Sha256Sum      string
}

// Upload a cache archive and associate it with the provided cache key
func Upload(params UploadParams, logger log.Logger) error {
	validatedKey, err := validateKey(params.CacheKey, logger)
	if err != nil {
		return err
	}

	client := newAPIClient(retryhttp.NewClient(logger), params.APIBaseURL, params.Token, logger)

	logger.Debugf("Get upload URL")
	prepareUploadRequest := prepareUploadRequest{
		CacheKey:           validatedKey,
		ArchiveFileName:    filepath.Base(params.ArchivePath),
		ArchiveContentType: "text/plain",
		ArchiveSizeInBytes: 1,
	}
	resp, err := client.prepareUpload(prepareUploadRequest)
	if err != nil {
		return fmt.Errorf("failed to get upload URL: %w", err)
	}
	logger.Debugf("Upload ID: %s", resp.ID)

	logger.Debugf("")
	logger.Debugf("Upload archive")
	uploadFlagFile, err := os.CreateTemp("", "uploadflag")
	if err != nil {
		return fmt.Errorf("create flag file: %w", err)
	}
	defer os.Remove(uploadFlagFile.Name())
	if _, err := uploadFlagFile.Write([]byte{42}); err != nil {
		return fmt.Errorf("write flag file: %w", err)
	}
	err = client.uploadArchive(uploadFlagFile.Name(), resp.UploadMethod, resp.UploadURL, resp.UploadHeaders)
	if err != nil {
		return fmt.Errorf("upload flag file: %w", err)
	}
	buildCacheKey, err := buildCacheKey(validatedKey)
	if err != nil {
		return fmt.Errorf("generate build cache key: %w", err)
	}

	const retries = 3
	err = retry.Times(retries).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		if attempt != 0 {
			logger.Debugf("Retrying archive upload... (attempt %d)", attempt+1)
		}

		ctx := context.Background()
		kvClient, err := kv.NewClient(ctx, kv.NewClientParams{
			UseInsecure: params.InsecureGRPC,
			Host:        params.BuildCacheHost,
			DialTimeout: 5 * time.Second,
			ClientName:  "kv",
			Token:       params.Token,
		})
		if err != nil {
			return fmt.Errorf("new kv client: %w", err), false
		}

		file, err := os.Open(params.ArchivePath)
		if err != nil {
			return fmt.Errorf("open %q: %w", params.ArchivePath, err), false
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			return fmt.Errorf("stat %q: %w", params.ArchivePath, err), false
		}

		kvWriter, err := kvClient.Put(ctx, kv.PutParams{
			Name:      buildCacheKey,
			Sha256Sum: params.Sha256Sum,
			FileSize:  stat.Size(),
		})
		if err != nil {
			return fmt.Errorf("create kv put client: %w", err), false
		}
		if _, err := io.Copy(kvWriter, file); err != nil {
			return fmt.Errorf("upload archive: %w", err), false
		}
		if err := kvWriter.Close(); err != nil {
			return fmt.Errorf("close upload: %w", err), false
		}
		return nil, false
	})
	if err != nil {
		return fmt.Errorf("with retries: %w", err)
	}

	logger.Debugf("")
	logger.Debugf("Acknowledge upload")
	response, err := client.acknowledgeUpload(resp.ID)
	if err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	logger.Debugf("Upload acknowledged")
	logResponseMessage(response, logger)

	return nil
}

func buildCacheKey(s string) (string, error) {
	h := sha256.New()
	if _, err := h.Write([]byte(s)); err != nil {
		return "", fmt.Errorf("write sha256: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func validateKey(key string, logger log.Logger) (string, error) {
	if strings.Contains(key, ",") {
		return "", fmt.Errorf("commas are not allowed in key")
	}

	if len(key) > maxKeyLength {
		logger.Warnf("Key is too long, truncating it to the first %d characters", maxKeyLength)
		return key[:maxKeyLength], nil
	}
	return key, nil
}

func logResponseMessage(response acknowledgeResponse, logger log.Logger) {
	if response.Message == "" || response.Severity == "" {
		return
	}

	var loggerFn func(format string, v ...interface{})
	switch response.Severity {
	case "debug":
		loggerFn = logger.Debugf
	case "info":
		loggerFn = logger.Infof
	case "warning":
		loggerFn = logger.Warnf
	case "error":
		loggerFn = logger.Errorf
	default:
		loggerFn = logger.Printf
	}

	loggerFn("\n")
	loggerFn(response.Message)
	loggerFn("\n")
}
