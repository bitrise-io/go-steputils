package network

import (
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/mock"
)

func Test_validateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{
			name: "valid key",
			key:  "my-cache-key",
			want: "my-cache-key",
		},
		{
			name:    "key with comma",
			key:     "my-cache-k,ey",
			wantErr: true,
		},
		{
			name: "key that is too long",
			key:  strings.Repeat("cache", 103),
			want: "cachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecachecacheca",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateKey(tt.key, log.NewLogger())
			if (err != nil) != tt.wantErr {
				t.Errorf("validateKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateKey() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_logResponseMessage(t *testing.T) {
	tests := []struct {
		name           string
		response       acknowledgeResponse
		wantLogMessage string
		wantLogFn      string
		wantSkip       bool
	}{
		{
			name: "Debug message",
			response: acknowledgeResponse{
				Message:  "Upload acknowledged: 1.6 GB used of 2 GB storage.",
				Severity: "debug",
			},
			wantLogMessage: "Upload acknowledged: 1.6 GB used of 2 GB storage.",
			wantLogFn:      "Debugf",
		},
		{
			name: "Warning message",
			response: acknowledgeResponse{
				Message:  "Upload acknowledged but quota exceeded: 8.6 GB used of 2 GB storage.",
				Severity: "warning",
			},
			wantLogMessage: "Upload acknowledged but quota exceeded: 8.6 GB used of 2 GB storage.",
			wantLogFn:      "Warnf",
		},
		{
			name:     "Empty response",
			response: acknowledgeResponse{},
			wantSkip: true,
		},
		{
			name: "Unrecognized severity",
			response: acknowledgeResponse{
				Message:  "Message from the future!",
				Severity: "fatal",
			},
			wantLogMessage: "Message from the future!",
			wantLogFn:      "Printf",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			mockLogger := new(MockLogger)
			if !tt.wantSkip {
				mockLogger.On(tt.wantLogFn, "\n", mock.Anything).Return()
				mockLogger.On(tt.wantLogFn, tt.wantLogMessage, mock.Anything).Return()
				mockLogger.On(tt.wantLogFn, "\n", mock.Anything).Return()
			}

			// When
			logResponseMessage(tt.response, mockLogger)

			// Then
			mockLogger.AssertExpectations(t)
		})
	}
}
