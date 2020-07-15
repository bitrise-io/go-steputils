package output

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/envutil"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/require"
)

func TestZipAndExportOutputDir(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := givenEnvmanIsSetup(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0777))

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	envKey := "EXPORTED_ZIP_PATH"
	require.NoError(t, ZipAndExportOutput(sourceDir, destinationZip, envKey))

	// destination should exist
	exist, err := pathutil.IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, envKey, destinationZip, envmanStorePath)
}

func TestExportOutputFileContent(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := givenEnvmanIsSetup(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceFileContent := "test"

	destinationFile := filepath.Join(tmpDir, "destination")

	envKey := "EXPORTED_FILE_PATH"
	require.NoError(t, ExportOutputFileContent(sourceFileContent, destinationFile, envKey))

	// destination should exist
	exist, err := pathutil.IsPathExists(destinationFile)
	require.NoError(t, err)
	require.Equal(t, true, exist)

	// destination should contain the source content
	content, err := fileutil.ReadStringFromFile(destinationFile)
	require.NoError(t, err)
	require.Equal(t, sourceFileContent, content)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, envKey, destinationFile, envmanStorePath)
}

func TestExportOutputFile(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := givenEnvmanIsSetup(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceFile := filepath.Join(tmpDir, "source")
	require.NoError(t, fileutil.WriteStringToFile(sourceFile, ""))

	destinationFile := filepath.Join(tmpDir, "destination")

	envKey := "EXPORTED_FILE_PATH"
	require.NoError(t, ExportOutputFile(sourceFile, destinationFile, envKey))

	// destination should exist
	exist, err := pathutil.IsPathExists(destinationFile)
	require.NoError(t, err)
	require.Equal(t, true, exist)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, envKey, destinationFile, envmanStorePath)
}

func TestExportOutputDir(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := givenEnvmanIsSetup(t)
	defer func() {
		err := envmanClearFn()
		require.NoError(t, err)
	}()

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0777))

	destinationDir := filepath.Join(tmpDir, "destination")

	envKey := "EXPORTED_DIR_PATH"
	require.NoError(t, ExportOutputDir(sourceDir, destinationDir, envKey))

	// destination should exist
	exist, err := pathutil.IsDirExists(destinationDir)
	require.NoError(t, err)
	require.Equal(t, true, exist)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, envKey, destinationDir, envmanStorePath)
}

func Test_RunAndExportOutput(t *testing.T) {
	scenarios := []struct {
		cmd                   *command.Model
		numberOfLines         int
		expectedConsoleOutput string
		expectedLogOutput     string
	}{
		{
			cmd:                   command.New("echo", "testing"),
			numberOfLines:         0,
			expectedConsoleOutput: "",
			expectedLogOutput:     "testing\n",
		},
		{
			cmd:                   command.New("echo", "testing"),
			numberOfLines:         1,
			expectedConsoleOutput: "testing\n",
			expectedLogOutput:     "testing\n",
		},
		{
			cmd:                   command.New("echo", "testing\ntesting"),
			numberOfLines:         1,
			expectedConsoleOutput: "testing\n",
			expectedLogOutput:     "testing\ntesting\n",
		},
	}

	for _, scenario := range scenarios {
		// Given
		logFilePath := givenTmpLogFilePath(t)
		envKey := "TEST_OUTPUT_KEY"
		envmanStorePath, envmanClearFn := givenEnvmanIsSetup(t)
		defer func() {
			err := envmanClearFn()
			require.NoError(t, err)
		}()

		// When
		var cmdErr error
		output := captureOuput(t, func() {
			cmdErr = RunAndExportOutput(scenario.cmd, logFilePath, envKey, scenario.numberOfLines)
		})

		// Then
		require.NoError(t, cmdErr)
		requireFileContents(t, scenario.expectedLogOutput, logFilePath)
		require.Equal(t, scenario.expectedConsoleOutput, output)
		requireEnvmanContainsValueForKey(t, envKey, logFilePath, envmanStorePath)
	}
}

func givenTmpLogFilePath(t *testing.T) string {
	tmp, err := ioutil.TempDir("", "log")
	require.NoError(t, err)

	return path.Join(tmp, "log.txt")
}

func captureOuput(t *testing.T, fn func()) string {
	tmp, err := ioutil.TempDir("", "log")
	require.NoError(t, err)

	pth := path.Join(tmp, "output")
	fi, err := os.Create(pth)
	require.NoError(t, err)

	pre := os.Stdout
	os.Stdout = fi
	fn()
	defer func() { os.Stdout = pre }()

	b, err := ioutil.ReadFile(pth)
	require.NoError(t, err)

	return string(b)
}

func requireFileContents(t *testing.T, contents, filePath string) {
	byteContents, err := ioutil.ReadFile(filePath)
	require.NoError(t, err)

	stringContents := string(byteContents)
	require.Equal(t, contents, stringContents)
}

func givenEnvmanIsSetup(t *testing.T) (string, func() error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	tmpDir, err = ioutil.TempDir("", "log")
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

func requireEnvmanContainsValueForKey(t *testing.T, key, value, envmanStorePath string) {
	envstoreContent, err := fileutil.ReadStringFromFile(envmanStorePath)
	require.NoError(t, err)
	t.Logf("envstoreContent: %s\n", envstoreContent)
	require.Equal(t, true, strings.Contains(envstoreContent, "- "+key+": "+value), envstoreContent)
}
