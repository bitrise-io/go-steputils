package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	internaltesting "github.com/bitrise-io/go-steputils/v2/internal/testing"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportOutput(t *testing.T) {
	envmanStorePath := setupEnvman(t)

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutput("my_key", "my value"))

	requireEnvmanContainsValueForKey(t, "my_key", "my value", false, envmanStorePath)
}

func TestExportSecretOutput(t *testing.T) {
	envmanStorePath := setupEnvman(t)

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportSecretOutput("my_key", "my secret value"))

	requireEnvmanContainsValueForKey(t, "my_key", "my secret value", true, envmanStorePath)
}

func TestExportOutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := setupEnvman(t)

	sourcePath := filepath.Join(tmpDir, "test_file_source")
	destinationPath := filepath.Join(tmpDir, "test_file_destination")
	require.NoError(t, os.WriteFile(sourcePath, []byte("hello"), 0700))

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFile("my_key", sourcePath, destinationPath))

	requireEnvmanContainsValueForKey(t, "my_key", destinationPath, false, envmanStorePath)
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
	exist, err := pathutil.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, false, envmanStorePath)
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
	exist, err := pathutil.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	requireEnvmanContainsValueForKey(t, key, destinationZip, false, envmanStorePath)
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

func TestExportOutputDirE2E(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	// umask in tmp is likely 022, so testing with compatible permissions (0700, 0755)
	srcDir := createSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	extraDir := filepath.Join(srcDir, "extraDir")
	require.NoError(t, os.MkdirAll(extraDir, 0700))
	linkTarget := filepath.Join(srcDir, "file1")
	os.Symlink(linkTarget, filepath.Join(extraDir, "link")) // nolint:errcheck
	//os.Chown(srcDir+"/file1", os.Getuid(), os.Getgid())     // nolint:errcheck
	os.Chown(srcDir+"/file1", os.Getuid(), os.Getgid()) // nolint:errcheck

	dstDir := filepath.Join(tmpDir, "dst-dir")

	sut := NewExporter((command.NewFactory(env.NewRepository())))
	assert.NoError(t, sut.ExportOutputDir("ENV_KEY", srcDir, dstDir))
	requireEnvmanContainsValueForKey(t, "ENV_KEY", dstDir, false, envmanStorePath)

	assert.NoError(t,
		internaltesting.NewFileChecker(dstDir).IsDir().ModeEquals(0755).Check(),
	)
	assert.NoError(t,
		internaltesting.NewFileChecker(dstDir+"/extraDir").IsDir().ModeEquals(0700).Check(),
	)
	assert.NoError(t,
		internaltesting.NewFileChecker(dstDir+"/file1").IsFile().ModeEquals(0755).Check(),
	)
	assert.NoError(t,
		internaltesting.NewFileChecker(dstDir+"/file2").IsFile().ModeEquals(0755).Check(),
	)
	assert.NoError(t,
		internaltesting.NewFileChecker(dstDir+"/file3").IsFile().ModeEquals(0755).Check(),
	)
	assert.NoError(t,
		internaltesting.NewFileChecker(dstDir+"/extraDir/link").IsSymlink().Check(),
	)
}

func TestExportOutputDir_GivenSrcIsFile_Fails(t *testing.T) {

	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	srcDir := createSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	extraDir := filepath.Join(srcDir, "empty-folder")
	require.NoError(t, os.MkdirAll(extraDir, 0777))

	dstDir := filepath.Join(tmpDir, "dst-dir")

	e := NewExporter((command.NewFactory(env.NewRepository())))
	assert.Error(t, e.ExportOutputDir("ENV_KEY", srcDir+"/file1", dstDir))
}

func TestExportOutputDir_GivenMissingSrc_Fails(t *testing.T) {

	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	dstDir := filepath.Join(tmpDir, "dst-dir")

	e := NewExporter((command.NewFactory(env.NewRepository())))
	assert.Error(t, e.ExportOutputDir("ENV_KEY", dstDir+"/file1", dstDir))
}

func TestExportStringToFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	e := NewExporter((command.NewFactory(env.NewRepository())))
	require.NoError(t, e.ExportStringToFileOutput("ENV_KEY", "content", tmpDir+"/file.txt"))
	requireEnvmanContainsValueForKey(t, "ENV_KEY", tmpDir+"/file.txt", false, envmanStorePath)

	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").IsFile().Check())
	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").Content("content").Check())

}

func TestExportStringToFileOutputAndReturnLastNLines(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	content := `line 1
line 2
line 3

line 4
line 5


`

	e := NewExporter((command.NewFactory(env.NewRepository())))
	lines, err := e.ExportStringToFileOutputAndReturnLastNLines("ENV_KEY", content, tmpDir+"/file.txt", 4)
	require.NoError(t, err)
	requireEnvmanContainsValueForKey(t, "ENV_KEY", tmpDir+"/file.txt", false, envmanStorePath)

	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").IsFile().Check())
	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").Content(content).Check())
	assert.Equal(t, "line 3\n\nline 4\nline 5", lines)
}

// ---------------------------
// Helpers
// ---------------------------

func createSrcDirWithFiles(t *testing.T, baseDir string, fileNames []string) string {
	srcDir := filepath.Join(baseDir, "src-dir")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	for _, name := range fileNames {
		sourceFile := filepath.Join(srcDir, name)
		require.NoError(t, os.WriteFile(sourceFile, []byte(name), 0755))
	}
	return srcDir
}

func requireEnvmanContainsValueForKey(t *testing.T, key, value string, secret bool, envmanStorePath string) {
	b, err := os.ReadFile(envmanStorePath)
	require.NoError(t, err)
	envstoreContent := string(b)

	t.Logf("envstoreContent: %s\n", envstoreContent)
	require.Equal(t, true, strings.Contains(envstoreContent, "- "+key+": "+value), envstoreContent)

	if secret {
		require.Equal(t, true, strings.Contains(envstoreContent, "is_sensitive: true"), envstoreContent)
	}
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

func setup() (Exporter, *MockFileManager) {
	mockFileManager := &MockFileManager{}
	return Exporter{
		cmdFactory:  command.NewFactory(env.NewRepository()),
		fileManager: mockFileManager,
	}, mockFileManager
}
