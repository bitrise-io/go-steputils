package testattachment

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/fileutil"
)

var (
	// ErrTrackedByGit is returned for a file that is tracked by git, e.g. a committed reference image.
	ErrTrackedByGit = errors.New("file is tracked by git")
	// ErrDuplicateName is returned when more than one collected file has the same name.
	ErrDuplicateName = errors.New("more than one file has this name")
	// ErrAlreadyExported is returned for a file that an earlier step already exported unchanged.
	ErrAlreadyExported = errors.New("file was already exported by an earlier step")
)

// Skipping these only saves time: they hold dependencies, never test output.
var skippedDirNames = map[string]bool{
	".git":         true,
	".idea":        true,
	"node_modules": true,
	"Pods":         true,
	"Carthage":     true,
	"CordovaLib":   true,
	".build":       true,
	".dart_tool":   true,
	".venv":        true,
	"venv":         true,
}

// Candidate is an attachment file that belongs to a test case of the index.
type Candidate struct {
	Path  string
	Match Match
}

// Skipped is a file that follows the naming convention but is not attached, with the reason.
type Skipped struct {
	Path   string
	Reason error
}

// CollectResult is the outcome of Collect.
type CollectResult struct {
	Candidates []Candidate
	Skipped    []Skipped
	// GitCheckErr is set when tracked files could not be filtered out (e.g. root is not in a git
	// repository). The candidates are then returned unfiltered.
	GitCheckErr error
}

// Collector finds and copies attachment files for a test report.
type Collector struct {
	cmdFactory  command.Factory
	fileManager fileutil.FileManager
}

// NewCollector returns a Collector that runs git through cmdFactory and copies files with fileManager.
func NewCollector(cmdFactory command.Factory, fileManager fileutil.FileManager) Collector {
	return Collector{cmdFactory: cmdFactory, fileManager: fileManager}
}

// Collect walks root and returns the files that belong to a test case of idx. Dependency folders,
// deployDir (where earlier steps exported their files), files tracked by git and file names found
// more than once are left out.
func (c Collector) Collect(root, deployDir string, idx Index) (CollectResult, error) {
	var result CollectResult
	var matched []Candidate
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (isSkippedDir(path) || filepath.Clean(path) == filepath.Clean(deployDir)) {
				return filepath.SkipDir
			}
			return nil
		}

		match, err := idx.Match(path)
		switch {
		case errors.Is(err, ErrUnsupportedType), errors.Is(err, ErrNoConvention):
			return nil
		case err != nil:
			result.Skipped = append(result.Skipped, Skipped{Path: path, Reason: err})
			return nil
		}
		matched = append(matched, Candidate{Path: path, Match: match})
		return nil
	})
	if err != nil {
		return CollectResult{}, fmt.Errorf("walk %s: %w", root, err)
	}

	// Tracked files go first: a committed reference image usually has the same name as the fresh
	// screenshot, and must not knock it out as a duplicate.
	untracked := matched
	tracked, err := c.trackedFiles(root, matched)
	if err != nil {
		result.GitCheckErr = err
	} else {
		untracked = nil
		for _, candidate := range matched {
			if tracked[candidate.Path] {
				result.Skipped = append(result.Skipped, Skipped{Path: candidate.Path, Reason: ErrTrackedByGit})
				continue
			}
			untracked = append(untracked, candidate)
		}
	}

	byName := map[string][]Candidate{}
	for _, candidate := range untracked {
		name := filepath.Base(candidate.Path)
		byName[name] = append(byName[name], candidate)
	}
	for _, name := range sortedKeys(byName) {
		found := byName[name]
		if len(found) > 1 {
			for _, candidate := range found {
				result.Skipped = append(result.Skipped, Skipped{Path: candidate.Path, Reason: ErrDuplicateName})
			}
			continue
		}
		result.Candidates = append(result.Candidates, found[0])
	}
	return result, nil
}

func isSkippedDir(path string) bool {
	name := filepath.Base(path)
	if skippedDirNames[name] || strings.HasSuffix(name, ".framework") {
		return true
	}
	return name == "bundle" && filepath.Base(filepath.Dir(path)) == "vendor"
}

// trackedFiles returns the candidate paths that git tracks, in a single git call.
func (c Collector) trackedFiles(root string, candidates []Candidate) (map[string]bool, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	args := []string{"ls-files", "-z", "--"}
	relToPath := map[string]string{}
	for _, candidate := range candidates {
		rel, err := filepath.Rel(root, candidate.Path)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		relToPath[rel] = candidate.Path
		args = append(args, rel)
	}

	out, err := c.cmdFactory.Create("git", args, &command.Opts{Dir: root}).RunAndReturnTrimmedOutput()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	tracked := map[string]bool{}
	for _, rel := range strings.Split(out, "\x00") {
		if path, ok := relToPath[rel]; ok {
			tracked[path] = true
		}
	}
	return tracked, nil
}

// SkipExported leaves out the candidates that an earlier step already exported into deployDir:
// a file with the same name, size and modification time. CopyToReport keeps the modification
// time, which is what makes an unchanged file recognisable.
func SkipExported(deployDir string, candidates []Candidate) ([]Candidate, []Skipped, error) {
	exported := map[string][]fs.FileInfo{}
	err := filepath.WalkDir(deployDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == deployDir && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		exported[d.Name()] = append(exported[d.Name()], info)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk %s: %w", deployDir, err)
	}

	var kept []Candidate
	var skipped []Skipped
	for _, candidate := range candidates {
		info, err := os.Stat(candidate.Path)
		if err != nil {
			return nil, nil, err
		}
		if isExported(info, exported[filepath.Base(candidate.Path)]) {
			skipped = append(skipped, Skipped{Path: candidate.Path, Reason: ErrAlreadyExported})
			continue
		}
		kept = append(kept, candidate)
	}
	return kept, skipped, nil
}

func isExported(info fs.FileInfo, exported []fs.FileInfo) bool {
	for _, e := range exported {
		if e.Size() == info.Size() && e.ModTime().Equal(info.ModTime()) {
			return true
		}
	}
	return false
}

// CopyToReport copies the candidates into reportDir under their own name, keeping their
// modification time.
func (c Collector) CopyToReport(reportDir string, candidates []Candidate) error {
	for _, candidate := range candidates {
		dst := filepath.Join(reportDir, filepath.Base(candidate.Path))
		// Copying a file onto itself would truncate it. Possible when base_path is inside the deploy dir.
		if filepath.Clean(dst) == filepath.Clean(candidate.Path) {
			continue
		}
		if err := c.fileManager.CopyFile(candidate.Path, dst, &fileutil.CopyOptions{Overwrite: true}); err != nil {
			return fmt.Errorf("copy %s to %s: %w", candidate.Path, reportDir, err)
		}
	}
	return nil
}

func sortedKeys(m map[string][]Candidate) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
