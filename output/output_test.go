package output

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/envutil"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/stretchr/testify/require"
)

func TestZipDirectoriesAndExportOutput(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := setupEnvman(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceA := filepath.Join(tmpDir, "sourceA")
	require.NoError(t, os.MkdirAll(sourceA, 0777))

	sourceB := filepath.Join(tmpDir, "sourceB")
	require.NoError(t, os.MkdirAll(sourceB, 0777))

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	key := "EXPORTED_ZIP_PATH"
	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFilesZip(key, []string{sourceA, sourceB}, destinationZip))

	// destination should exist
	exist, err := pathutil.IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, envmanStorePath)
}

func TestZipFilesAndExportOutput(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := setupEnvman(t)
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

	key := "EXPORTED_ZIP_PATH"
	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFilesZip(key, sourceFilePaths, destinationZip))

	// destination should exist
	exist, err := pathutil.IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, envmanStorePath)
}

func TestZipMixedFilesAndFoldersAndExportOutput(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	_, envmanClearFn := setupEnvman(t)
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

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.Error(t, e.ExportOutputFilesZip("EXPORTED_ZIP_PATH", sourceFilePaths, destinationZip))
}

func requireEnvmanContainsValueForKey(t *testing.T, key, value, envmanStorePath string) {
	b, err := ioutil.ReadFile(envmanStorePath)
	require.NoError(t, err)
	envstoreContent := string(b)

	t.Logf("envstoreContent: %s\n", envstoreContent)
	require.Equal(t, true, strings.Contains(envstoreContent, "- "+key+": "+value), envstoreContent)
}

func setupEnvman(t *testing.T) (string, func() error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	tmpDir, err = ioutil.TempDir("", "envman")
	revokeFn, err := pathutil.RevokableChangeDir(tmpDir)
	require.NoError(t, err)

	tmpEnvStorePth := filepath.Join(tmpDir, ".envstore.yml")
	require.NoError(t, fileutil.WriteStringToFile(tmpEnvStorePth, ""))

	envstoreRevokeFn, err := envutil.RevokableSetenv("ENVMAN_ENVSTORE_PATH", tmpEnvStorePth)
	require.NoError(t, err)

	return tmpEnvStorePth, func() error {
		if err := revokeFn(); err != nil {
			return err
		}

		return envstoreRevokeFn()
	}
}
