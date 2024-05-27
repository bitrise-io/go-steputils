package network

import (
	"context"

	"github.com/bitrise-io/go-utils/v2/log"
)

// Uploader ...
type Uploader interface {
	Upload(context.Context, UploadParams, log.Logger) error
}

// Downloader ...
type Downloader interface {
	Download(context.Context, DownloadParams, log.Logger) (string, error)
}
