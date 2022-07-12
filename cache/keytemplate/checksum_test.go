package keytemplate

import (
	"runtime"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
)

func TestChecksum(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  string
	}{
		{
			name:  "Single file",
			paths: []string{"testdata/package-lock.json"},
			want:  "c048b369d6e8b0616971ccc5aa33df2910d6b78c408041a8d6b11cfb8d38b29e",
		},
		{
			name:  "No file",
			paths: []string{},
			want:  "",
		},
		{
			name:  "Invalid file path",
			paths: []string{"not_going_to_work"},
			want:  "",
		},
		{
			name:  "File list",
			paths: []string{"testdata/package-lock.json", "testdata/build.gradle"},
			want:  "3332b32f95e07206f0915399e16444d6cbfb59dfb1f821b51a67ea3270f758d7",
		},
		{
			name:  "File list, one file is invalid",
			paths: []string{"testdata/package-lock.json", "testdata/build.gradle", "invalid"},
			want:  "3332b32f95e07206f0915399e16444d6cbfb59dfb1f821b51a67ea3270f758d7",
		},
		{
			name:  "Single glob star",
			paths: []string{"testdata/*.gradle"},
			want:  "db094ffe3aea59fc48766cb408894ada1c67dbd355d25085729394df82fb1eda",
		},
		{
			name:  "Double glob star",
			paths: []string{"testdata/**/*.gradle"},
			want:  "3a6e11679515ce19ef1728549588e672e76b00c9b6855ed1d33d0305ec5ecad3",
		},
		{
			name:  "Multiple glob stars",
			paths: []string{"testdata/**/*.gradle*"},
			want:  "563cf037f336453ee1888c3dcbe1c687ebeb6c593d4d0bd57ccc5fc49daa3951",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.NewLogger()
			logger.EnableDebugLog(true)
			m := Model{
				envRepo: envRepository{},
				logger:  logger,
				os:      runtime.GOOS,
				arch:    runtime.GOARCH,
			}
			if got := m.checksum(tt.paths...); got != tt.want {
				t.Errorf("checksum() = %v, want %v", got, tt.want)
			}
		})
	}
}
