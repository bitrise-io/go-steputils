package output

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/internal/test"
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

func TestExportOutputFileContent(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := test.EnvmanIsSetup(t)
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
	test.RequireEnvmanContainsValueForKey(t, envKey, destinationFile, envmanStorePath)
}

func TestExportOutputFile(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := test.EnvmanIsSetup(t)
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
	test.RequireEnvmanContainsValueForKey(t, envKey, destinationFile, envmanStorePath)
}

func TestExportOutputDir(t *testing.T) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	envmanStorePath, envmanClearFn := test.EnvmanIsSetup(t)
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
	test.RequireEnvmanContainsValueForKey(t, envKey, destinationDir, envmanStorePath)
}

func Test_ExportOutputFileContentAndReturnLastNLines(t *testing.T) {
	scenarios := []struct {
		content        string
		numberOfLines  int
		expectedOutput string
	}{
		{
			content:        "wow\ncontent",
			numberOfLines:  0,
			expectedOutput: "",
		},
		{
			content:        "wow",
			numberOfLines:  1,
			expectedOutput: "wow",
		},
		{
			content:        "wow\ncontent",
			numberOfLines:  1,
			expectedOutput: "content",
		},
	}

	for _, scenario := range scenarios {
		// Given
		logFilePath := givenTmpLogFilePath(t)
		envKey := "TEST_OUTPUT_KEY"
		envmanStorePath, envmanClearFn := test.EnvmanIsSetup(t)
		defer func() {
			err := envmanClearFn()
			require.NoError(t, err)
		}()

		// When
		output, err := ExportOutputFileContentAndReturnLastNLines(scenario.content, logFilePath, envKey, scenario.numberOfLines)

		// Then
		require.NoError(t, err)
		requireFileContents(t, scenario.content, logFilePath)
		require.Equal(t, scenario.expectedOutput, output)
		test.RequireEnvmanContainsValueForKey(t, envKey, logFilePath, envmanStorePath)
	}
}

func givenTmpLogFilePath(t *testing.T) string {
	tmp, err := ioutil.TempDir("", "log")
	require.NoError(t, err)

	return path.Join(tmp, "log.txt")
}

func requireFileContents(t *testing.T, contents, filePath string) {
	byteContents, err := ioutil.ReadFile(filePath)
	require.NoError(t, err)

	stringContents := string(byteContents)
	require.Equal(t, contents, stringContents)
}
