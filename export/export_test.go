package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	pathutil2 "github.com/bitrise-io/go-utils/v2/pathutil"
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

func TestExportOutputDir_Success(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "subdir", "file2.txt"), []byte("content2"), 0644))

	destinationDir := filepath.Join(tmpDir, "destination")

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDir("my_key", sourceDir, destinationDir))

	requireEnvmanContainsValueForKey(t, "my_key", destinationDir, false, envmanStorePath)

	destFile1, err := os.ReadFile(filepath.Join(destinationDir, "file1.txt"))
	require.NoError(t, err)
	require.Equal(t, "content1", string(destFile1))

	destFile2, err := os.ReadFile(filepath.Join(destinationDir, "subdir", "file2.txt"))
	require.NoError(t, err)
	require.Equal(t, "content2", string(destFile2))
}

func TestExportOutputDir_SameSourceAndDestination(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0644))

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDir("my_key", sourceDir, sourceDir))

	requireEnvmanContainsValueForKey(t, "my_key", sourceDir, false, envmanStorePath)
}

func TestExportOutputDir_RelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	originalWorkDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalWorkDir)

	require.NoError(t, os.Chdir(tmpDir))

	sourceDir := "source"
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0644))

	destinationDir := "destination"

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDir("my_key", sourceDir, destinationDir))

	currentDir, err := os.Getwd()
	require.NoError(t, err)
	absDestinationDir := filepath.Join(currentDir, destinationDir)
	requireEnvmanContainsValueForKey(t, "my_key", absDestinationDir, false, envmanStorePath)
}

func TestExportOutputDir_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "empty_source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	destinationDir := filepath.Join(tmpDir, "destination")

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDir("my_key", sourceDir, destinationDir))

	requireEnvmanContainsValueForKey(t, "my_key", destinationDir, false, envmanStorePath)

	exist, err := pathutil2.NewPathChecker().IsDirExists(destinationDir)
	require.NoError(t, err)
	require.True(t, exist)
}

func TestExportOutputDir_PreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0750))

	restrictedDir := filepath.Join(sourceDir, "restricted")
	require.NoError(t, os.Mkdir(restrictedDir, 0700))

	publicDir := filepath.Join(sourceDir, "public")
	require.NoError(t, os.Mkdir(publicDir, 0755))

	executableFile := filepath.Join(sourceDir, "script.sh")
	require.NoError(t, os.WriteFile(executableFile, []byte("#!/bin/bash"), 0755))

	regularFile := filepath.Join(restrictedDir, "file.txt")
	require.NoError(t, os.WriteFile(regularFile, []byte("content"), 0644))

	destinationDir := filepath.Join(tmpDir, "destination")

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDir("my_key", sourceDir, destinationDir))

	destRestrictedDir := filepath.Join(destinationDir, "restricted")
	info, err := os.Stat(destRestrictedDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), info.Mode().Perm())

	destPublicDir := filepath.Join(destinationDir, "public")
	info, err = os.Stat(destPublicDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), info.Mode().Perm())

	destExecFile := filepath.Join(destinationDir, "script.sh")
	info, err = os.Stat(destExecFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0755), info.Mode().Perm())

	destRegularFile := filepath.Join(destinationDir, "restricted", "file.txt")
	info, err = os.Stat(destRegularFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestExportOutputDir_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "source")
	deepDir := filepath.Join(sourceDir, "a", "b", "c", "d")
	require.NoError(t, os.MkdirAll(deepDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(deepDir, "deep.txt"), []byte("deep content"), 0644))

	destinationDir := filepath.Join(tmpDir, "destination")

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDir("my_key", sourceDir, destinationDir))

	destDeepFile := filepath.Join(destinationDir, "a", "b", "c", "d", "deep.txt")
	content, err := os.ReadFile(destDeepFile)
	require.NoError(t, err)
	require.Equal(t, "deep content", string(content))
}

func TestExportOutputDir_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "nonexistent")
	destinationDir := filepath.Join(tmpDir, "destination")

	e := NewExporter(command.NewFactory(env.NewRepository()))
	err := e.ExportOutputDir("my_key", sourceDir, destinationDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stat source directory")
}

func TestExportOutputDir_SourceIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	sourceFile := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("content"), 0644))

	destinationDir := filepath.Join(tmpDir, "destination")

	e := NewExporter(command.NewFactory(env.NewRepository()))
	err := e.ExportOutputDir("my_key", sourceFile, destinationDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source is not a directory")
}

func TestExportOutputDir_FollowsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	_ = setupEnvman(t)

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	targetFile := filepath.Join(tmpDir, "target.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("target content"), 0644))

	symlinkFile := filepath.Join(sourceDir, "link.txt")
	require.NoError(t, os.Symlink(targetFile, symlinkFile))

	destinationDir := filepath.Join(tmpDir, "destination")

	e := NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputDir("my_key", sourceDir, destinationDir))

	destFile := filepath.Join(destinationDir, "link.txt")
	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	require.Equal(t, "target content", string(content))

	info, err := os.Lstat(destFile)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "destination should be a regular file, not a symlink")
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
