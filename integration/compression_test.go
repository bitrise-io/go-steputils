//go:build integration
// +build integration

package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitrise-io/go-steputils/v2/cache/compression"
	"github.com/bitrise-io/go-utils/v2/log"
)

func Test_compression(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		zstdFound        bool
		compressionLevel int
		customTarArgs    []string
	}{
		{
			name:             "zstd installed=true",
			zstdFound:        true,
			compressionLevel: 3,
		},
		{
			name:             "zstd installed=false",
			zstdFound:        false,
			compressionLevel: 3,
		},
		{
			name:             "compression_level=19",
			zstdFound:        true,
			compressionLevel: 19,
		},
		{
			name:             "compression_level=19",
			zstdFound:        true,
			compressionLevel: 1,
		},
		{
			name:             "custom arg: --format posix",
			zstdFound:        true,
			compressionLevel: 1,
			customTarArgs:    []string{"--format", "posix"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Given
			checkerMock := &compression.ArchiveDependencyCheckerMock{
				CheckDependenciesFunc: func() bool {
					return tc.zstdFound
				},
			}

			archivePath := filepath.Join(t.TempDir(),
				fmt.Sprintf("compression_test_%t.tzst", checkerMock.CheckDependencies()))
			logger := log.NewLogger()
			envRepo := fakeEnvRepo{envVars: map[string]string{
				"BITRISE_SOURCE_DIR": ".",
			}}

			// When
			archiver := compression.NewArchiver(
				logger,
				envRepo,
				checkerMock)

			err := archiver.Compress(archivePath, []string{"testdata/subfolder"}, tc.compressionLevel, tc.customTarArgs)
			if err != nil {
				t.Errorf(err.Error())
			}
			archiveContents, err := listArchiveContents(archivePath)
			if err != nil {
				t.Errorf(err.Error())
			}

			expected := []string{
				"testdata/subfolder",
				"testdata/subfolder/nested_file.txt",
			}
			assert.ElementsMatch(t, expected, archiveContents)
		})
	}
}
