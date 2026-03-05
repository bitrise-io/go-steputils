// Package chunkdownloader provides a reusable, optimized parallel download system.
// It wraps the got library with retry, hung chunk detection, and HTTP transport tuning.
package chunkdownloader

import (
	"context"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bitrise-io/got"
)

// Logger is a minimal logging interface satisfied by *zap.SugaredLogger,
// go-utils/v2/log.Logger, and most structured loggers out of the box.
type Logger interface {
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

// Downloader handles parallel chunked downloads with retry and hung detection.
type Downloader struct {
	config Config
	logger Logger
}

// New creates a new Downloader with the given configuration and logger.
func New(config Config, logger Logger) *Downloader {
	return &Downloader{
		config: config,
		logger: logger,
	}
}

// DownloadFile downloads the file at url to dest using parallel Range requests.
// If the server does not support Range requests, it falls back to a single-stream download.
func (d *Downloader) DownloadFile(ctx context.Context, url, dest string) error {
	client := d.config.HTTPClient
	if client == nil {
		client = got.DefaultClient
	}

	applyTransportTuning(client)

	d.logger.Infof("Downloading %s to %s", url, dest)

	downloader := got.NewWithContext(ctx)
	downloader.Client = client

	dl := got.NewDownload(ctx, url, dest)
	dl.Client = client
	dl.Concurrency = d.config.Concurrency
	dl.MaxRetryPerChunk = d.config.MaxRetryPerChunk
	dl.ChunkRetryThreshold = d.config.ChunkRetryThreshold

	if err := downloader.Do(dl); err != nil {
		d.logger.Errorf("Download failed: %s", err)
		return err
	}

	d.logger.Infof("Download complete: %s", dest)
	return nil
}

// applyTransportTuning adjusts http.Transport settings from environment variables.
func applyTransportTuning(client *http.Client) {
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		return
	}

	if val, err := strconv.Atoi(os.Getenv("BITRISEIO_DEPENDENCY_CACHE_MAX_IDLE_CONNS_PER_HOST")); err == nil {
		transport.MaxIdleConnsPerHost = val
	}

	if val, err := strconv.Atoi(os.Getenv("BITRISEIO_DEPENDENCY_CACHE_MAX_IDLE_CONNS")); err == nil {
		transport.MaxIdleConns = val
	}

	env := os.Getenv("BITRISEIO_DEPENDENCY_CACHE_FORCE_ATTEMPT_HTTP2")
	transport.ForceAttemptHTTP2 = env == "true" || env == "1"

	env = os.Getenv("BITRISEIO_DEPENDENCY_CACHE_DUALSTACK")
	dualStack := env == "true" || env == "1"
	transport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: dualStack,
	}).DialContext
}
