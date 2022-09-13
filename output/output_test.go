package output

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-steputils/internal/test"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/require"
)

func TestZipDirectoriesAndExportOutput(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := test.EnvmanIsSetup(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceA := filepath.Join(tmpDir, "sourceA")
	require.NoError(t, os.MkdirAll(sourceA, 0777))

	sourceB := filepath.Join(tmpDir, "sourceB")
	require.NoError(t, os.MkdirAll(sourceB, 0777))

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	envKey := "EXPORTED_ZIP_PATH"
	require.NoError(t, ZipAndExportOutput([]string{sourceA, sourceB}, destinationZip, envKey))

	// destination should exist
	exist, err := pathutil.IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	test.RequireEnvmanContainsValueForKey(t, envKey, destinationZip, envmanStorePath)
}

func TestZipFilesAndExportOutput(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := test.EnvmanIsSetup(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0777))

	var sourceFilePaths []string
	for _, name := range []string{"A", "B", "C"} {
		sourceFile := filepath.Join(sourceDir, "sourceFile"+name)
		require.NoError(t, fileutil.WriteStringToFile(sourceFile, name))

		sourceFilePaths = append(sourceFilePaths, sourceFile)
	}

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	envKey := "EXPORTED_ZIP_PATH"
	require.NoError(t, ZipAndExportOutput(sourceFilePaths, destinationZip, envKey))

	// destination should exist
	exist, err := pathutil.IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	test.RequireEnvmanContainsValueForKey(t, envKey, destinationZip, envmanStorePath)
}

func TestZipMixedFilesAndFoldersAndExportOutput(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	_, envmanClearFn := test.EnvmanIsSetup(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0777))

	var sourceFilePaths []string
	for _, name := range []string{"A", "B", "C"} {
		sourceFile := filepath.Join(sourceDir, "sourceFile"+name)
		require.NoError(t, fileutil.WriteStringToFile(sourceFile, name))

		sourceFilePaths = append(sourceFilePaths, sourceFile)
	}

	extraDir := filepath.Join(sourceDir, "empty-folder")
	require.NoError(t, os.MkdirAll(extraDir, 0777))

	sourceFilePaths = append(sourceFilePaths, extraDir)

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	require.Error(t, ZipAndExportOutput(sourceFilePaths, destinationZip, "EXPORTED_ZIP_PATH"))
}
