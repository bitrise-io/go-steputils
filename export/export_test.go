package export

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	pathutil2 "github.com/bitrise-io/go-utils/v2/pathutil"
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
	exist, err := pathutil2.NewPathChecker().IsPathExists(destinationZip)
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
		require.NoError(t, ioutil.WriteFile(sourceFile, []byte(name), 0777))

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

func TestExportOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	srcDir := createSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	extraDir := filepath.Join(srcDir, "empty-folder")
	require.NoError(t, os.MkdirAll(extraDir, 0777))
	linkTarget := filepath.Join(srcDir, "file1")
	os.Symlink(linkTarget, filepath.Join(extraDir, "link")) // nolint:errcheck

	dstDir := filepath.Join(tmpDir, "dst-dir")

	e := NewExporter((command.NewFactory(env.NewRepository())))
	assert.NoError(t, e.ExportOutputDir(srcDir, dstDir, "ENV_KEY"))
	requireEnvmanContainsValueForKey(t, "ENV_KEY", dstDir, false, envmanStorePath)

	assertDirExists(t, dstDir)
	assertDirExists(t, dstDir+"/empty-folder")
	assertFileExists(t, dstDir+"/file1")
	assertFileExists(t, dstDir+"/file2")
	assertFileExists(t, dstDir+"/file3")
	assertIsSymlink(t, dstDir+"/empty-folder/link")
}

func TestExportOutputDir_GivenSrcIsFile_Fails(t *testing.T) {

	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	srcDir := createSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	extraDir := filepath.Join(srcDir, "empty-folder")
	require.NoError(t, os.MkdirAll(extraDir, 0777))

	dstDir := filepath.Join(tmpDir, "dst-dir")

	e := NewExporter((command.NewFactory(env.NewRepository())))
	assert.Error(t, e.ExportOutputDir(srcDir+"/file1", dstDir, "ENV_KEY"))
}

func TestExportOutputDir_GivenMissingSrc_Fails(t *testing.T) {

	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	dstDir := filepath.Join(tmpDir, "dst-dir")

	e := NewExporter((command.NewFactory(env.NewRepository())))
	assert.Error(t, e.ExportOutputDir("src-dir", dstDir, "ENV_KEY"))
}

func TestExportOutputFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	e := NewExporter((command.NewFactory(env.NewRepository())))
	require.NoError(t, e.ExportOutputFileContent("content", tmpDir+"/file.txt", "ENV_KEY"))
	requireEnvmanContainsValueForKey(t, "ENV_KEY", tmpDir+"/file.txt", false, envmanStorePath)

	assertFileExists(t, tmpDir+"/file.txt")
	assertFileContentEqual(t, tmpDir+"/file.txt", "content")
}

func TestExportOutputFileContentAndReturnLastNLines(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	content := `line 1
line 2
line 3

line 4
line 5


`

	e := NewExporter((command.NewFactory(env.NewRepository())))
	lines, err := e.ExportOutputFileContentAndReturnLastNLines(content, tmpDir+"/file.txt", "ENV_KEY", 4)
	require.NoError(t, err)
	requireEnvmanContainsValueForKey(t, "ENV_KEY", tmpDir+"/file.txt", false, envmanStorePath)

	assertFileExists(t, tmpDir+"/file.txt")
	assertFileContentEqual(t, tmpDir+"/file.txt", content)
	assert.Equal(t, "line 3\n\nline 4\nline 5", lines)
}

func createSrcDirWithFiles(t *testing.T, baseDir string, fileNames []string) string {
	srcDir := filepath.Join(baseDir, "src-dir")
	require.NoError(t, os.MkdirAll(srcDir, 0777))
	for _, name := range fileNames {
		sourceFile := filepath.Join(srcDir, name)
		require.NoError(t, os.WriteFile(sourceFile, []byte(name), 0777))
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

// assertDirExists fails the test if path does not exist or is not a directory.
func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			t.Logf("expected directory to exist, but it does not: %s", path)
		}
		t.Logf("unexpected error statting path %s: %v", path, err)
		t.Fail()
		return
	}

	if !info.IsDir() {
		t.Logf("expected %s to be a directory, but it's not", path)
		t.Fail()
		return
	}
}

// assertFileExists fails the test if path does not exist or is not a regular file.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			t.Logf("expected file to exist, but it does not: %s", path)
		}
		t.Logf("unexpected error statting path %s: %v", path, err)
		t.Fail()
		return
	}

	if info.IsDir() {
		t.Logf("expected %s to be a file, but it's a directory", path)
		t.Fail()
		return
	}
}

// isSymlink returns true if path exists and is a symlink
func assertIsSymlink(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)

	if err != nil {
		if os.IsNotExist(err) {
			t.Logf("expected file to exist, but it does not: %s", path)
		}
		t.Logf("unexpected error statting path %s: %v", path, err)
		t.Fail()
		return
	}

	if (info.Mode() & os.ModeSymlink) == 0 {
		t.Logf("expected %s to be a symlink, but it's not", path)
		t.Fail()
		return
	}
}

func assertFileContentEqual(t *testing.T, path string, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Logf("read file %s: %v", path, err)
		t.Fail()
		return
	}
	got := string(b)
	if got != want {
		t.Logf("file %s content mismatch\nwant:\n%q\n\ngot:\n%q", path, want, got)
		t.Fail()
		return
	}
}
