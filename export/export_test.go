package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	pathutil2 "github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExportOutput(t *testing.T) {
	envmanStorePath := setupEnvman(t)

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutput("my_key", "my value"))

	requireEnvmanContainsValueForKey(t, "my_key", "my value", envmanStorePath)
}

func TestExportOutputWithEnvs(t *testing.T) {
	envmanStorePath := setupEnvman(t)

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutput("my_key", "this is a value with env vars: $HOME, $PWD, $SHELL"))

	requireEnvmanContainsValueForKey(t, "my_key", "this is a value with env vars: $HOME, $PWD, $SHELL", envmanStorePath)
}

func TestExportOutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := setupEnvman(t)

	sourcePath := filepath.Join(tmpDir, "test_file_source")
	destinationPath := filepath.Join(tmpDir, "test_file_destination")
	require.NoError(t, os.WriteFile(sourcePath, []byte("hello"), 0700))

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFile("my_key", sourcePath, destinationPath))

	requireEnvmanContainsValueForKey(t, "my_key", destinationPath, envmanStorePath)
}

func TestZipDirectoriesAndExportOutput(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := setupEnvman(t)

	sourceA := filepath.Join(tmpDir, "sourceA")
	require.NoError(t, os.MkdirAll(sourceA, 0777))

	sourceB := filepath.Join(tmpDir, "sourceB")
	require.NoError(t, os.MkdirAll(sourceB, 0777))

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	key := "EXPORTED_ZIP_PATH"
	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFilesZip(key, []string{sourceA, sourceB}, destinationZip))

	// destination should exist
	exist, err := pathutil2.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, envmanStorePath)
}

func TestZipFilesAndExportOutput(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0777))

	var sourceFilePaths []string
	for _, name := range []string{"A", "B", "C"} {
		sourceFile := filepath.Join(sourceDir, "sourceFile"+name)
		require.NoError(t, os.WriteFile(sourceFile, []byte(name), 0777))

		sourceFilePaths = append(sourceFilePaths, sourceFile)
	}

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	key := "EXPORTED_ZIP_PATH"
	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFilesZip(key, sourceFilePaths, destinationZip))

	// destination should exist
	exist, err := pathutil2.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, envmanStorePath)
}

func TestZipMixedFilesAndFoldersAndExportOutput(t *testing.T) {
	tmpDir := t.TempDir()

	_ = setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0777))

	var sourceFilePaths []string
	for _, name := range []string{"A", "B", "C"} {
		sourceFile := filepath.Join(sourceDir, "sourceFile"+name)
		require.NoError(t, os.WriteFile(sourceFile, []byte(name), 0777))

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
	b, err := os.ReadFile(envmanStorePath)
	require.NoError(t, err)

	type envItemModel map[string]interface{}
	type envstoreModel struct {
		Envs []envItemModel `yaml:"envs"`
	}
	var envstoreContent envstoreModel
	err = yaml.Unmarshal(b, &envstoreContent)
	require.NoError(t, err)

	found := false
	for _, envItem := range envstoreContent.Envs {
		if _, ok := envItem[key]; ok {
			found = true
			break
		}
	}

	require.True(t, found, "envstore should contain %s=%s. Full envstore content:\n%s", key, value, envstoreContent)
}

func setupEnvman(t *testing.T) string {
	originalWorkDir, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = os.Chdir(originalWorkDir)
		require.NoError(t, err)
	})
	require.NoError(t, err)

	tmpEnvStorePth := filepath.Join(tmpDir, ".envstore.yml")
	require.NoError(t, os.WriteFile(tmpEnvStorePth, []byte(""), 0777))

	t.Setenv("ENVMAN_ENVSTORE_PATH", tmpEnvStorePth)

	return tmpEnvStorePth
}
