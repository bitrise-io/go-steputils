package testattachment

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var loginReport = reportOf(
	testCase("com.example.LoginTest", "emptyState"),
	testCase("com.example.LoginTest", "wrongPassword"),
)

func TestCollect(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	deployDir := filepath.Join(root, "deploy")
	inRoot := func(rel string) string { return filepath.Join(root, rel) }

	writeFile(t, inRoot("build/screenshots/com.example.LoginTest__emptyState__1.png"), "a")
	writeFile(t, inRoot("com.example.LoginTest__wrongPassword__api34.mp4"), "b")
	writeFile(t, inRoot("node_modules/pkg/com.example.LoginTest__emptyState__2.png"), "c")
	writeFile(t, inRoot("ios/Pods/com.example.LoginTest__emptyState__3.png"), "c")
	writeFile(t, inRoot("vendor/bundle/com.example.LoginTest__emptyState__4.png"), "c")
	writeFile(t, inRoot("Some.framework/com.example.LoginTest__emptyState__5.png"), "c")
	writeFile(t, filepath.Join(deployDir, "com.example.LoginTest__emptyState__6.png"), "c")
	writeFile(t, inRoot("com.example.LoginTest__otherTest__1.png"), "d")
	writeFile(t, inRoot("debug.log"), "e")
	writeFile(t, inRoot("a/com.example.LoginTest__emptyState__dup.png"), "f")
	writeFile(t, inRoot("b/com.example.LoginTest__emptyState__dup.png"), "g")

	result, err := newTestCollector().Collect(root, deployDir, NewIndex(loginReport))
	require.NoError(t, err)

	require.NoError(t, result.GitCheckErr)
	assert.Equal(t, []string{
		inRoot("build/screenshots/com.example.LoginTest__emptyState__1.png"),
		inRoot("com.example.LoginTest__wrongPassword__api34.mp4"),
	}, candidatePaths(result.Candidates))
	assert.Equal(t, map[string]error{
		inRoot("com.example.LoginTest__otherTest__1.png"):      ErrUnknownTest,
		inRoot("a/com.example.LoginTest__emptyState__dup.png"): ErrDuplicateName,
		inRoot("b/com.example.LoginTest__emptyState__dup.png"): ErrDuplicateName,
	}, skippedReasons(result.Skipped))
}

func TestCollect_skipsFilesTrackedByGit(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	committed := filepath.Join(root, "src/test/snapshots/com.example.LoginTest__emptyState__1.png")
	fresh := filepath.Join(root, "build/com.example.LoginTest__wrongPassword__1.png")
	writeFile(t, committed, "reference")
	writeFile(t, fresh, "new")

	runGit(t, root, "init", "-q")
	runGit(t, root, "add", "src")
	runGit(t, root, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "reference images")

	result, err := newTestCollector().Collect(root, filepath.Join(t.TempDir(), "deploy"), NewIndex(loginReport))
	require.NoError(t, err)

	require.NoError(t, result.GitCheckErr)
	assert.Equal(t, []string{fresh}, candidatePaths(result.Candidates))
	assert.Equal(t, map[string]error{committed: ErrTrackedByGit}, skippedReasons(result.Skipped))
}

func TestCollect_referenceImageDoesNotHideFreshScreenshot(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	reference := filepath.Join(root, "app/src/test/snapshots/com.example.LoginTest__emptyState__1.png")
	fresh := filepath.Join(root, "app/build/screenshots/com.example.LoginTest__emptyState__1.png")
	writeFile(t, reference, "reference")
	writeFile(t, fresh, "new")

	runGit(t, root, "init", "-q")
	runGit(t, root, "add", "app/src")
	runGit(t, root, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "reference images")

	result, err := newTestCollector().Collect(root, filepath.Join(t.TempDir(), "deploy"), NewIndex(loginReport))
	require.NoError(t, err)

	require.NoError(t, result.GitCheckErr)
	assert.Equal(t, []string{fresh}, candidatePaths(result.Candidates))
	assert.Equal(t, map[string]error{reference: ErrTrackedByGit}, skippedReasons(result.Skipped))
}

func TestCollect_outsideGitRepository(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "rev-parse").Run(); err == nil {
		t.Skip("the temp dir is inside a git repository")
	}
	path := filepath.Join(root, "com.example.LoginTest__emptyState__1.png")
	writeFile(t, path, "content")

	result, err := newTestCollector().Collect(root, filepath.Join(t.TempDir(), "deploy"), NewIndex(loginReport))
	require.NoError(t, err)

	assert.Error(t, result.GitCheckErr)
	assert.Equal(t, []string{path}, candidatePaths(result.Candidates))
}

func TestSkipExported(t *testing.T) {
	dir := t.TempDir()
	deployDir := filepath.Join(dir, "deploy")
	mtime := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

	unchanged := filepath.Join(dir, "src/com.example.LoginTest__emptyState__1.png")
	changed := filepath.Join(dir, "src/com.example.LoginTest__wrongPassword__1.png")
	notExported := filepath.Join(dir, "src/com.example.LoginTest__wrongPassword__2.png")
	for _, path := range []string{unchanged, changed, notExported} {
		writeFile(t, path, "content")
		require.NoError(t, os.Chtimes(path, mtime, mtime))
	}

	writeFile(t, filepath.Join(deployDir, "step_1/UI tests/com.example.LoginTest__emptyState__1.png"), "content")
	require.NoError(t, os.Chtimes(filepath.Join(deployDir, "step_1/UI tests/com.example.LoginTest__emptyState__1.png"), mtime, mtime))
	writeFile(t, filepath.Join(deployDir, "step_1/UI tests/com.example.LoginTest__wrongPassword__1.png"), "content")

	candidates := []Candidate{{Path: unchanged}, {Path: changed}, {Path: notExported}}
	kept, skipped, err := skipExported(deployDir, candidates)
	require.NoError(t, err)

	assert.Equal(t, []string{changed, notExported}, candidatePaths(kept))
	assert.Equal(t, map[string]error{unchanged: ErrAlreadyExported}, skippedReasons(skipped))
}

func TestSkipExported_missingDeployDir(t *testing.T) {
	candidates := []Candidate{{Path: filepath.Join(t.TempDir(), "x")}}
	writeFile(t, candidates[0].Path, "content")

	kept, skipped, err := skipExported(filepath.Join(t.TempDir(), "missing"), candidates)
	require.NoError(t, err)

	assert.Equal(t, candidates, kept)
	assert.Empty(t, skipped)
}

func TestCopyToReport(t *testing.T) {
	dir := t.TempDir()
	reportDir := filepath.Join(dir, "report")
	require.NoError(t, os.MkdirAll(reportDir, 0o755))
	src := filepath.Join(dir, "build/shots/com.example.LoginTest__emptyState__1.png")
	writeFile(t, src, "content")
	mtime := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(src, mtime, mtime))

	candidates := []Candidate{{Path: src}}
	require.NoError(t, newTestCollector().CopyToReport(reportDir, candidates))

	copied, err := os.Stat(filepath.Join(reportDir, "com.example.LoginTest__emptyState__1.png"))
	require.NoError(t, err)
	assert.True(t, copied.ModTime().Equal(mtime))

	kept, skipped, err := skipExported(reportDir, candidates)
	require.NoError(t, err)
	assert.Empty(t, kept)
	assert.Equal(t, map[string]error{src: ErrAlreadyExported}, skippedReasons(skipped))
}

func TestCopyToReport_fileAlreadyInReportDir(t *testing.T) {
	reportDir := t.TempDir()
	path := filepath.Join(reportDir, "com.example.LoginTest__emptyState__1.png")
	writeFile(t, path, "content")

	require.NoError(t, newTestCollector().CopyToReport(reportDir, []Candidate{{Path: path}}))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestCollect_basePathInsideDeployDir(t *testing.T) {
	deployDir := t.TempDir()
	root := filepath.Join(deployDir, "step_4")
	reportDir := filepath.Join(root, "API 34")
	path := filepath.Join(reportDir, "com.example.LoginTest__emptyState__1.png")
	writeFile(t, path, "content")
	collector := newTestCollector()

	result, err := collector.Collect(root, deployDir, NewIndex(loginReport))
	require.NoError(t, err)

	assert.Empty(t, result.Candidates)
	assert.Equal(t, map[string]error{path: ErrAlreadyExported}, skippedReasons(result.Skipped))

	require.NoError(t, collector.CopyToReport(reportDir, []Candidate{{Path: path}}))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestCollect_secondRunInDifferentFolder(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	deployDir := filepath.Join(t.TempDir(), "deploy")
	collector := newTestCollector()
	idx := NewIndex(loginReport)

	first := filepath.Join(root, "out/run1/com.example.LoginTest__emptyState__1.png")
	writeFile(t, first, "first run")
	result, err := collector.Collect(root, deployDir, idx)
	require.NoError(t, err)
	reportDir := filepath.Join(deployDir, "step_2", "Run 1")
	require.NoError(t, os.MkdirAll(reportDir, 0o755))
	require.NoError(t, collector.CopyToReport(reportDir, result.Candidates))

	second := filepath.Join(root, "out/run2/com.example.LoginTest__emptyState__1.png")
	writeFile(t, second, "second run")
	result, err = collector.Collect(root, deployDir, idx)
	require.NoError(t, err)

	assert.Equal(t, []string{second}, candidatePaths(result.Candidates))
	assert.Equal(t, map[string]error{first: ErrAlreadyExported}, skippedReasons(result.Skipped))
}

func TestCollect_manyCandidates(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	runGit(t, root, "init", "-q")

	report := reportOf()
	for i := 0; i < 10_000; i++ {
		name := fmt.Sprintf("givenAVeryDescriptiveState%dWhenSomethingHappensThenSomethingIsShown", i)
		report.TestSuites[0].TestCases = append(report.TestSuites[0].TestCases, testCase("com.example.feature.SomeScreenTest", name))
		writeFile(t, filepath.Join(root, "app/build/outputs/screenshots", Key("com.example.feature.SomeScreenTest", name)+"__1.png"), "x")
	}
	committed := filepath.Join(root, "app/src/test/snapshots/com.example.feature.SomeScreenTest__givenAVeryDescriptiveState0WhenSomethingHappensThenSomethingIsShown__reference.png")
	writeFile(t, committed, "reference")
	runGit(t, root, "add", "app/src")
	runGit(t, root, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-q", "-m", "reference images")

	result, err := newTestCollector().Collect(root, filepath.Join(t.TempDir(), "deploy"), NewIndex(report))
	require.NoError(t, err)

	require.NoError(t, result.GitCheckErr)
	assert.Len(t, result.Candidates, 10_000)
	assert.Equal(t, map[string]error{committed: ErrTrackedByGit}, skippedReasons(result.Skipped))
}

func TestCollect_gitErrorDoesNotListFiles(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "rev-parse").Run(); err == nil {
		t.Skip("the temp dir is inside a git repository")
	}
	path := filepath.Join(root, "com.example.LoginTest__emptyState__1.png")
	writeFile(t, path, "content")

	result, err := newTestCollector().Collect(root, filepath.Join(t.TempDir(), "deploy"), NewIndex(loginReport))
	require.NoError(t, err)

	require.Error(t, result.GitCheckErr)
	assert.NotContains(t, result.GitCheckErr.Error(), filepath.Base(path))
}

func TestCollect_unreadableFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any folder")
	}
	root := t.TempDir()
	readable := filepath.Join(root, "build/com.example.LoginTest__emptyState__1.png")
	writeFile(t, readable, "content")
	locked := filepath.Join(root, "locked")
	require.NoError(t, os.MkdirAll(locked, 0o755))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	result, err := newTestCollector().Collect(root, filepath.Join(t.TempDir(), "deploy"), NewIndex(loginReport))
	require.NoError(t, err)

	assert.Equal(t, []string{readable}, candidatePaths(result.Candidates))
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, locked, result.Skipped[0].Path)
	assert.ErrorIs(t, result.Skipped[0].Reason, fs.ErrPermission)
}

func TestCollect_missingRoot(t *testing.T) {
	_, err := newTestCollector().Collect(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "deploy"), NewIndex(loginReport))

	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func newTestCollector() Collector {
	return NewCollector(command.NewFactory(env.NewRepository()), fileutil.NewFileManager())
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
}

func candidatePaths(candidates []Candidate) []string {
	var paths []string
	for _, candidate := range candidates {
		paths = append(paths, candidate.Path)
	}
	sort.Strings(paths)
	return paths
}

func skippedReasons(skipped []Skipped) map[string]error {
	reasons := map[string]error{}
	for _, s := range skipped {
		reasons[s.Path] = s.Reason
	}
	return reasons
}
