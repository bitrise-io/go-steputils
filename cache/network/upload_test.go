package network

import (
	"strings"
	"testing"
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
			got, err := validateKey(tt.key)
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
