package network

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/retryhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_prepareMultipartUpload(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		want        prepareMultipartUploadResponse
		wantSkipped *UploadSkippedError
		wantErr     bool
	}{
		{
			name:   "201 returns upload URLs",
			status: http.StatusCreated,
			body:   `{"id":"upload-id","chunk_size_bytes":10,"chunk_count":1,"last_chunk_size_bytes":10,"urls":[{"method":"PUT","url":"https://storage/part1"}]}`,
			want: prepareMultipartUploadResponse{
				ID: "upload-id", ChunkSizeBytes: 10, ChunkCount: 1, LastChunkSizeBytes: 10,
				URLs: []prepareMultipartUploadURL{{Method: "PUT", URL: "https://storage/part1"}},
			},
		},
		{
			name:        "200 with skipped:true returns UploadSkippedError",
			status:      http.StatusOK,
			body:        `{"skipped":true,"reason":"identical_key_upload_in_progress","existing_id":"existing-id"}`,
			wantSkipped: &UploadSkippedError{Reason: "identical_key_upload_in_progress", ExistingID: "existing-id"},
		},
		{
			name:    "200 without skipped:true is an error",
			status:  http.StatusOK,
			body:    `{"id":"upload-id","urls":[]}`,
			wantErr: true,
		},
		{
			name:    "200 with invalid body is an error",
			status:  http.StatusOK,
			body:    `not json`,
			wantErr: true,
		},
		{
			name:    "400 is an error",
			status:  http.StatusBadRequest,
			body:    `{"message":"bad request"}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotSkipHeader string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/multipart-upload", r.URL.Path)
				gotSkipHeader = r.Header.Get(uploadSkipHeader)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			logger := log.NewLogger()
			client := newAPIClient(retryhttp.NewClient(logger), server.URL, "token", logger)

			got, err := client.prepareMultipartUpload(prepareUploadRequest{CacheKey: "key", ArchiveSizeInBytes: 10})

			assert.Equal(t, "1", gotSkipHeader)
			switch {
			case tt.wantSkipped != nil:
				require.ErrorIs(t, err, ErrUploadSkipped)
				var skipped UploadSkippedError
				require.True(t, errors.As(err, &skipped))
				assert.Equal(t, *tt.wantSkipped, skipped)
			case tt.wantErr:
				require.Error(t, err)
				assert.NotErrorIs(t, err, ErrUploadSkipped)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_Upload_serverSkip(t *testing.T) {
	var acknowledged bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			acknowledged = true
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"skipped":true,"reason":"identical_key_recently_uploaded","existing_id":"existing-id"}`))
	}))
	defer server.Close()

	err := DefaultUploader{}.Upload(context.Background(), UploadParams{
		APIBaseURL:  server.URL,
		Token:       "token",
		ArchivePath: "/does/not/exist.tzst", // never opened on skip
		ArchiveSize: 10,
		CacheKey:    "key",
	}, log.NewLogger())

	require.ErrorIs(t, err, ErrUploadSkipped)
	assert.False(t, acknowledged, "a skipped upload must not be acknowledged")
}
