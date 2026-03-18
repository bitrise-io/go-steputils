package export_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-steputils/v2/export/mocks"
	internaltesting "github.com/bitrise-io/go-steputils/v2/internal/testing"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExportOutput(t *testing.T) {
	envmanStorePath := export.SetupEnvman(t)

	e := export.NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutput("my_key", "my value"))

	export.RequireEnvmanContainsValueForKey(t, "my_key", "my value", false, envmanStorePath)
}

func TestExportSecretOutput(t *testing.T) {
	envmanStorePath := export.SetupEnvman(t)

	e := export.NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportSecretOutput("my_key", "my secret value"))

	export.RequireEnvmanContainsValueForKey(t, "my_key", "my secret value", true, envmanStorePath)
}

func TestExportOutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := export.SetupEnvman(t)

	sourcePath := filepath.Join(tmpDir, "test_file_source")
	destinationPath := filepath.Join(tmpDir, "test_file_destination")
	require.NoError(t, os.WriteFile(sourcePath, []byte("hello"), 0700))

	e := export.NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFile("my_key", sourcePath, destinationPath, nil))

	export.RequireEnvmanContainsValueForKey(t, "my_key", destinationPath, false, envmanStorePath)
}

func TestExportOutputFile_GivenCopyFails_WillFail(t *testing.T) {
	tmpDir := t.TempDir()

	_ = export.SetupEnvman(t)
	fileManager := mocks.NewFileManager(t)
	sut := export.NewExporterWithFileManager(command.NewFactory(env.NewRepository()), fileManager)
	fileManager.EXPECT().CopyFile(mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("test"))

	srcDir := export.CreateSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	dstDir := filepath.Join(tmpDir, "dst-dir")

	require.ErrorContains(t, sut.ExportOutputFile("my_key", srcDir, dstDir, nil), "test")
}

func TestExportOutputFile_GivenSameSrcAndDst_SkipsCopy(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := export.SetupEnvman(t)
	fileManager := mocks.NewFileManager(t)
	sut := export.NewExporterWithFileManager(command.NewFactory(env.NewRepository()), fileManager)

	srcDir := export.CreateSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})

	require.NoError(t, sut.ExportOutputFile("my_key", srcDir, srcDir, nil))
	export.RequireEnvmanContainsValueForKey(t, "my_key", srcDir, false, envmanStorePath)
}

func TestZipDirectoriesAndExportOutput(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := export.SetupEnvman(t)

	sourceA := filepath.Join(tmpDir, "sourceA")
	require.NoError(t, os.MkdirAll(sourceA, 0777))

	sourceB := filepath.Join(tmpDir, "sourceB")
	require.NoError(t, os.MkdirAll(sourceB, 0777))

	destinationZip := filepath.Join(tmpDir, "destination.zip")

	key := "EXPORTED_ZIP_PATH"
	e := export.NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFilesZip(key, []string{sourceA, sourceB}, destinationZip, nil))

	// destination should exist
	exist, err := pathutil.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	export.RequireEnvmanContainsValueForKey(t, key, destinationZip, false, envmanStorePath)
}

func TestZipFilesAndExportOutput(t *testing.T) {
	tmpDir := t.TempDir()

	envmanStorePath := export.SetupEnvman(t)

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
	e := export.NewExporter(command.NewFactory(env.NewRepository()))
	require.NoError(t, e.ExportOutputFilesZip(key, sourceFilePaths, destinationZip, nil))

	// destination should exist
	exist, err := pathutil.NewPathChecker().IsPathExists(destinationZip)
	require.NoError(t, err)
	require.Equal(t, true, exist, tmpDir)

	// destination should be exported
	export.RequireEnvmanContainsValueForKey(t, key, destinationZip, false, envmanStorePath)
}

func TestZipMixedFilesAndFoldersAndExportOutput(t *testing.T) {
	tmpDir := t.TempDir()

	_ = export.SetupEnvman(t)

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

	e := export.NewExporter(command.NewFactory(env.NewRepository()))
	require.Error(t, e.ExportOutputFilesZip("EXPORTED_ZIP_PATH", sourceFilePaths, destinationZip, nil))
}

func TestExportOutputDirE2E(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := export.SetupEnvman(t)

	// umask in tmp is likely 022, so testing with compatible permissions (0700, 0755)
	srcDir := export.CreateSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	extraDir := filepath.Join(srcDir, "extraDir")
	require.NoError(t, os.MkdirAll(extraDir, 0700))
	linkTarget := filepath.Join(srcDir, "file1")
	os.Symlink(linkTarget, filepath.Join(extraDir, "link")) // nolint:errcheck
	os.Chown(srcDir+"/file1", os.Getuid(), os.Getgid())     // nolint:errcheck

	dstDir := filepath.Join(tmpDir, "dst-dir")

	sut := export.NewExporter((command.NewFactory(env.NewRepository())))
	assert.NoError(t, sut.ExportOutputDir("ENV_KEY", srcDir, dstDir, nil))
	export.RequireEnvmanContainsValueForKey(t, "ENV_KEY", dstDir, false, envmanStorePath)

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
	_ = export.SetupEnvman(t)

	srcDir := export.CreateSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	extraDir := filepath.Join(srcDir, "empty-folder")
	require.NoError(t, os.MkdirAll(extraDir, 0777))

	dstDir := filepath.Join(tmpDir, "dst-dir")

	e := export.NewExporter((command.NewFactory(env.NewRepository())))
	assert.Error(t, e.ExportOutputDir("ENV_KEY", srcDir+"/file1", dstDir, nil))
}

func TestExportOutputDir_GivenMissingSrc_Fails(t *testing.T) {

	tmpDir := t.TempDir()
	_ = export.SetupEnvman(t)

	dstDir := filepath.Join(tmpDir, "dst-dir")

	e := export.NewExporter((command.NewFactory(env.NewRepository())))
	assert.Error(t, e.ExportOutputDir("ENV_KEY", dstDir+"/file1", dstDir, nil))
}

func TestExportStringToFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := export.SetupEnvman(t)

	e := export.NewExporter((command.NewFactory(env.NewRepository())))
	require.NoError(t, e.ExportStringToFileOutput("ENV_KEY", "content", tmpDir+"/file.txt", nil))
	export.RequireEnvmanContainsValueForKey(t, "ENV_KEY", tmpDir+"/file.txt", false, envmanStorePath)

	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").IsFile().Check())
	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").Content("content").Check())
}

func TestExportStringToFileOutputAndReturnLastNLines(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := export.SetupEnvman(t)

	content := `line 1
line 2
line 3

line 4
line 5


`

	e := export.NewExporter((command.NewFactory(env.NewRepository())))
	lines, err := e.ExportStringToFileOutputAndReturnLastNLines("ENV_KEY", content, tmpDir+"/file.txt", 4, nil)
	require.NoError(t, err)
	export.RequireEnvmanContainsValueForKey(t, "ENV_KEY", tmpDir+"/file.txt", false, envmanStorePath)

	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").IsFile().Check())
	assert.NoError(t, internaltesting.NewFileChecker(tmpDir+"/file.txt").Content(content).Check())
	assert.Equal(t, "line 3\n\nline 4\nline 5", lines)
}

func TestExportOutputDir_GivenLStatSrcFails_Fails(t *testing.T) {
	tmpDir := t.TempDir()
	_ = export.SetupEnvman(t)

	fileManager := mocks.NewFileManager(t)
	sut := export.NewExporterWithFileManager(command.NewFactory(env.NewRepository()), fileManager)

	srcDir := export.CreateSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	dstDir := filepath.Join(tmpDir, "dst-dir")

	fileManager.EXPECT().Lstat(srcDir).Return(nil, fmt.Errorf("test"))
	assert.ErrorContains(t, sut.ExportOutputDir("ENV_KEY", srcDir, dstDir, nil), "test")
}

func TestExportOutputDir_GivenMatchingSrcAndDst_SkipsCopy(t *testing.T) {
	tmpDir := t.TempDir()
	envmanStorePath := export.SetupEnvman(t)

	fileManager := mocks.NewFileManager(t)
	sut := export.NewExporterWithFileManager(command.NewFactory(env.NewRepository()), fileManager)

	srcDir := export.CreateSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})

	fileManager.EXPECT().Lstat(srcDir).Return(os.Stat(srcDir))
	assert.NoError(t, sut.ExportOutputDir("ENV_KEY", srcDir, srcDir, nil), "test")
	export.RequireEnvmanContainsValueForKey(t, "ENV_KEY", srcDir, false, envmanStorePath)
}

func TestExportOutputDir_GivenFileManagerCopyFails_Fails(t *testing.T) {
	tmpDir := t.TempDir()
	_ = export.SetupEnvman(t)

	fileManager := mocks.NewFileManager(t)
	sut := export.NewExporterWithFileManager(command.NewFactory(env.NewRepository()), fileManager)

	srcDir := export.CreateSrcDirWithFiles(t, tmpDir, []string{"file1", "file2", "file3"})
	dstDir := filepath.Join(tmpDir, "dst-dir")

	fileManager.EXPECT().Lstat(srcDir).Return(os.Lstat(srcDir))
	fileManager.EXPECT().CopyDir(srcDir, dstDir, mock.Anything).Return(fmt.Errorf("test"))

	assert.ErrorContains(t, sut.ExportOutputDir("ENV_KEY", srcDir, dstDir, nil), "test")
}

func TestExportStringToFileOutput_GivenWriteBytesFails_WillFail(t *testing.T) {
	tmpDir := t.TempDir()
	_ = export.SetupEnvman(t)

	fileManager := mocks.NewFileManager(t)
	sut := export.NewExporterWithFileManager(command.NewFactory(env.NewRepository()), fileManager)

	fileManager.EXPECT().WriteBytes(tmpDir+"/file.txt", []byte("content")).Return(fmt.Errorf("test"))

	require.ErrorContains(t, sut.ExportStringToFileOutput("ENV_KEY", "content", tmpDir+"/file.txt", nil), "test")
}
