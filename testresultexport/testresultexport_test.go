package testresultexport_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/testresultexport"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/stretchr/testify/require"
)

func TestExportTest_file(t *testing.T) {
	xmlPath := filepath.Join(t.TempDir(), "junit.xml")
	writeFile(t, xmlPath, "<testsuites/>")

	exportRoot := t.TempDir()
	e := testresultexport.NewExporter(exportRoot, fileutil.NewFileManager())

	require.NoError(t, e.ExportTest("Unit tests", xmlPath))

	requireTestInfo(t, filepath.Join(exportRoot, "Unit tests"), "Unit tests")
	requireFile(t, filepath.Join(exportRoot, "Unit tests", "junit.xml"), "<testsuites/>")
}

func TestExportTest_bundleKeepsItsFolder(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "UITests.xcresult")
	writeFile(t, filepath.Join(bundle, "Info.plist"), "plist")
	writeFile(t, filepath.Join(bundle, "Data", "data.0"), "data")

	exportRoot := t.TempDir()
	e := testresultexport.NewExporter(exportRoot, fileutil.NewFileManager())

	require.NoError(t, e.ExportTest("UI tests", bundle))

	requireTestInfo(t, filepath.Join(exportRoot, "UI tests"), "UI tests")
	requireFile(t, filepath.Join(exportRoot, "UI tests", "UITests.xcresult", "Info.plist"), "plist")
	requireFile(t, filepath.Join(exportRoot, "UI tests", "UITests.xcresult", "Data", "data.0"), "data")
}

func TestExportTest_trailingSlashExportsTheFolderContents(t *testing.T) {
	resultsDir := filepath.Join(t.TempDir(), "testDebugUnitTest")
	writeFile(t, filepath.Join(resultsDir, "TEST-A.xml"), "a")
	writeFile(t, filepath.Join(resultsDir, "TEST-B.xml"), "b")

	exportRoot := t.TempDir()
	e := testresultexport.NewExporter(exportRoot, fileutil.NewFileManager())

	require.NoError(t, e.ExportTest("Unit tests", resultsDir+string(filepath.Separator)))

	requireTestInfo(t, filepath.Join(exportRoot, "Unit tests"), "Unit tests")
	requireFile(t, filepath.Join(exportRoot, "Unit tests", "TEST-A.xml"), "a")
	requireFile(t, filepath.Join(exportRoot, "Unit tests", "TEST-B.xml"), "b")
}

func TestExportTest_overwritesPreviousExport(t *testing.T) {
	xmlPath := filepath.Join(t.TempDir(), "junit.xml")
	exportRoot := t.TempDir()
	e := testresultexport.NewExporter(exportRoot, fileutil.NewFileManager())

	writeFile(t, xmlPath, "first")
	require.NoError(t, e.ExportTest("tests", xmlPath))
	writeFile(t, xmlPath, "second")
	require.NoError(t, e.ExportTest("tests", xmlPath))

	requireFile(t, filepath.Join(exportRoot, "tests", "junit.xml"), "second")
}

func TestExportTest_missingTestResult(t *testing.T) {
	e := testresultexport.NewExporter(t.TempDir(), fileutil.NewFileManager())

	err := e.ExportTest("tests", filepath.Join(t.TempDir(), "missing.xml"))

	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestExportTest_mkdirFails(t *testing.T) {
	// Use a path where a regular file blocks directory creation.
	base := t.TempDir()
	blockingFile := filepath.Join(base, "blocker")
	require.NoError(t, os.WriteFile(blockingFile, []byte{}, 0644))

	e := testresultexport.NewExporter(blockingFile, fileutil.NewFileManager())

	err := e.ExportTest("suite", base)
	require.Error(t, err)
}

func TestResultDescriptorFileName(t *testing.T) {
	require.Equal(t, "test-info.json", testresultexport.ResultDescriptorFileName)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func requireFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}

func requireTestInfo(t *testing.T, exportDir, wantName string) {
	t.Helper()
	infoBytes, err := os.ReadFile(filepath.Join(exportDir, testresultexport.ResultDescriptorFileName))
	require.NoError(t, err)
	var info testresultexport.TestInfo
	require.NoError(t, json.Unmarshal(infoBytes, &info))
	require.Equal(t, wantName, info.Name)
}
