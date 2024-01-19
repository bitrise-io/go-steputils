//go:build integration
// +build integration

package integration

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitrise-io/go-steputils/v2/cache/compression"
	"github.com/bitrise-io/go-utils/v2/log"
)

func Test_compression(t *testing.T) {
	havingZstdMock := &compression.ArchiveDependencyCheckerMock{
		CheckZstdFunc: func() bool {
			return true
		},
	}
	notHavingZstdMock := &compression.ArchiveDependencyCheckerMock{
		CheckZstdFunc: func() bool {
			return false
		},
	}
	zstdCheckerMocks := []*compression.ArchiveDependencyCheckerMock{havingZstdMock, notHavingZstdMock}

	// Given
	archivePath := filepath.Join(t.TempDir(), "compression_test.tzst")
	logger := log.NewLogger()
	envRepo := fakeEnvRepo{envVars: map[string]string{
		"BITRISE_SOURCE_DIR": ".",
	}}

	for _, zstdCheckerkMock := range zstdCheckerMocks {
		testCaseName := fmt.Sprintf("Compression with zstd - having zstd installed: %s", strconv.FormatBool(zstdCheckerkMock.CheckZstd()))
		t.Run(testCaseName, func(t *testing.T) {
			// When
			archiver := compression.NewArchiver(
				logger,
				envRepo,
				zstdCheckerkMock)

			err := archiver.Compress(archivePath, []string{"testdata/subfolder"})
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
