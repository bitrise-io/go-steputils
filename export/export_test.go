package export

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/envutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/require"
)

func TestExportOutput(t *testing.T) {
	envmanStorePath, envmanClearFn := setupEnvman(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutput("my_key", "my value"))

	requireEnvmanContainsValueForKey(t, "my_key", "my value", envmanStorePath)
}

func TestExportOutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath, envmanClearFn := setupEnvman(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourcePath := filepath.Join(tmpDir, "test_file_source")
	destinationPath := filepath.Join(tmpDir, "test_file_destination")
	require.NoError(t, ioutil.WriteFile(sourcePath, []byte("hello"), 0700))

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFile("my_key", sourcePath, destinationPath))

	requireEnvmanContainsValueForKey(t, "my_key", destinationPath, envmanStorePath)
}

func TestExportOutputDirZip(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath, envmanClearFn := setupEnvman(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceA := filepath.Join(tmpDir, "sourceA.txt")
	require.NoError(t, ioutil.WriteFile(sourceA, []byte("hello"), 0777))

	sourceB := filepath.Join(tmpDir, "sourceB.txt")
	require.NoError(t, ioutil.WriteFile(sourceB, []byte(strings.Repeat("bitrise", 10)), 0777))

	destinationZip := filepath.Join(t.TempDir(), "destination.zip")

	key := "EXPORTED_ZIP_PATH"
	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDirZip(key, tmpDir, destinationZip))

	// destination should exist
	exist, err := pathutil.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, envmanStorePath)
}

func TestExportOutputFilesZip(t *testing.T) {
	tmpDir := t.TempDir()

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
		require.NoError(t, ioutil.WriteFile(sourceFile, []byte(name), 0777))

		sourceFilePaths = append(sourceFilePaths, sourceFile)
	}

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	key := "EXPORTED_ZIP_PATH"
	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFilesZip(key, sourceFilePaths, destinationZip))

	// destination should exist
	exist, err := pathutil.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, envmanStorePath)
}

func TestZipMixedFilesAndFoldersAndExportOutput(t *testing.T) {
	tmpDir := t.TempDir()

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
		require.NoError(t, ioutil.WriteFile(sourceFile, []byte(name), 0777))

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
	tmpDir := t.TempDir()
	revokeFn, err := pathutil.RevokableChangeDir(tmpDir)
	require.NoError(t, err)

	tmpEnvStorePth := filepath.Join(tmpDir, ".envstore.yml")
	require.NoError(t, ioutil.WriteFile(tmpEnvStorePth, []byte(""), 0777))

	envstoreRevokeFn, err := envutil.RevokableSetenv("ENVMAN_ENVSTORE_PATH", tmpEnvStorePth)
	require.NoError(t, err)

	return tmpEnvStorePth, func() error {
		if err := revokeFn(); err != nil {
			return err
		}

		return envstoreRevokeFn()
	}
}
