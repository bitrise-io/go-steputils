package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/cache/network"
	"github.com/bitrise-io/go-utils/v2/analytics"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUploader struct {
	err    error
	params []network.UploadParams
}

func (u *fakeUploader) Upload(_ context.Context, params network.UploadParams, _ log.Logger) error {
	u.params = append(u.params, params)
	return u.err
}

type trackedEvent struct {
	name       string
	properties analytics.Properties
}

type fakeTracker struct {
	events []trackedEvent
}

func (t *fakeTracker) Enqueue(eventName string, properties ...analytics.Properties) {
	merged := analytics.Properties{}
	for _, p := range properties {
		for k, v := range p {
			merged[k] = v
		}
	}
	t.events = append(t.events, trackedEvent{name: eventName, properties: merged})
}

func (t *fakeTracker) Wait()            {}
func (t *fakeTracker) IsTracking() bool { return true }

func (t *fakeTracker) named(name string) []trackedEvent {
	var events []trackedEvent
	for _, e := range t.events {
		if e.name == name {
			events = append(events, e)
		}
	}
	return events
}

func Test_Save_serverUploadSkip(t *testing.T) {
	tests := []struct {
		name          string
		uploadErr     error
		wantErr       bool
		wantSkipped   bool
		wantReason    string
		wantUploadEvt int
	}{
		{
			name:        "server skip: in progress",
			uploadErr:   fmt.Errorf("upload with multipart: %w", network.UploadSkippedError{Reason: "identical_key_upload_in_progress", ExistingID: "id"}),
			wantSkipped: true,
			wantReason:  "identical_key_upload_in_progress",
		},
		{
			name:        "server skip: recently uploaded",
			uploadErr:   network.UploadSkippedError{Reason: "identical_key_recently_uploaded", ExistingID: "id"},
			wantSkipped: true,
			wantReason:  "identical_key_recently_uploaded",
		},
		{
			name:       "upload failure",
			uploadErr:  errors.New("HTTP 500"),
			wantErr:    true,
			wantReason: reasonNoRestore.String(),
		},
		{
			name:          "upload success",
			wantReason:    reasonNoRestore.String(),
			wantUploadEvt: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envRepo := fakeEnvRepo{envVars: map[string]string{
				"BITRISEIO_ABCS_API_URL":                  "https://cache.example",
				"BITRISEIO_BITRISE_SERVICES_ACCESS_TOKEN": "token",
			}}
			uploader := &fakeUploader{err: tt.uploadErr}
			tracker := &fakeTracker{}
			logger := log.NewLogger()
			s := NewSaver(envRepo, logger, pathutil.NewPathProvider(), pathutil.NewPathModifier(), pathutil.NewPathChecker(), uploader)
			s.newTracker = func(string) stepTracker { return stepTracker{tracker: tracker, logger: logger} }

			err := s.Save(SaveCacheInput{StepId: "save-cache", Key: "cache-key", Paths: []string{"testdata/dummy_file.txt"}})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, uploader.params, 1)
			assert.True(t, uploader.params[0].AllowServerSkip, "Save handles server skips, so it must opt in")

			skipEvents := tracker.named("step_save_cache_upload_skipped")
			require.Len(t, skipEvents, 1, "exactly one upload-skip result per save")
			assert.Equal(t, tt.wantSkipped, skipEvents[0].properties["is_upload_skipped"])
			assert.Equal(t, tt.wantReason, skipEvents[0].properties["reason"])
			assert.Len(t, tracker.named("step_save_cache_archive_uploaded"), tt.wantUploadEvt)
		})
	}
}
