//go:build integration
// +build integration

package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitrise-io/go-steputils/v2/cache/compression"
	"github.com/bitrise-io/go-utils/v2/log"
)

func Test_compression(t *testing.T) {
	checkTools()

	// Given
	archivePath := filepath.Join(t.TempDir(), "compression_test.tzst")
	logger := log.NewLogger()
	envRepo := fakeEnvRepo{envVars: map[string]string{
		"BITRISE_SOURCE_DIR": ".",
	}}

	// When
	err := compression.Compress(archivePath, []string{"../step/testdata/"}, logger, envRepo)
	if err != nil {
		t.Errorf(err.Error())
	}
	archiveContents, err := listArchiveContents(archivePath)
	if err != nil {
		t.Errorf(err.Error())
	}

	expected := []string{
		"../step/testdata/",
		"../step/testdata/subfolder/",
		"../step/testdata/dummy_file.txt",
		"../step/testdata/subfolder/nested_file.txt",
	}
	assert.ElementsMatch(t, expected, archiveContents)
}
