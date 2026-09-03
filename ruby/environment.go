package ruby

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
)

const systemRubyPth = "/usr/bin/ruby"

// brewRubyPths are the ruby executable paths of a Homebrew install, covering both the Intel
// (/usr/local) and the Apple Silicon (/opt/homebrew) Homebrew prefixes.
var brewRubyPths = []string{
	"/usr/local/bin/ruby",
	"/usr/local/opt/ruby/bin/ruby",
	"/opt/homebrew/bin/ruby",
	"/opt/homebrew/opt/ruby/bin/ruby",
}

// InstallType ...
type InstallType int8

const (
	// Unknown means that the Ruby install type could not be determined.
	Unknown InstallType = iota
	// SystemRuby ...
	SystemRuby
	// BrewRuby ...
	BrewRuby
	// RVMRuby ...
	RVMRuby
	// RbenvRuby ...
	RbenvRuby
	// ASDFRuby ...
	ASDFRuby
	// MiseRuby means that ruby is managed by mise (https://mise.jdx.dev).
	MiseRuby
)

// ErrRubyNotFound is returned when there is no ruby executable in the PATH.
var ErrRubyNotFound = errors.New("ruby executable not found in PATH")

// Environment ...
type Environment interface {
	RubyInstallType() InstallType
	IsGemInstalled(gem, version string) (bool, error)
	IsSpecifiedRbenvRubyInstalled(workdir string) (bool, string, error)
	IsSpecifiedASDFRubyInstalled(workdir string) (bool, string, error)
}

type environment struct {
	factory    CommandFactory
	cmdLocator env.CommandLocator
	logger     log.Logger
}

// NewEnvironment ...
func NewEnvironment(factory CommandFactory, cmdLocator env.CommandLocator, logger log.Logger) Environment {
	return environment{
		factory:    factory,
		cmdLocator: cmdLocator,
		logger:     logger,
	}
}

// RubyInstallType returns which version manager was used for the ruby install.
// It returns Unknown both when there is no ruby executable in the PATH and when ruby is installed
// but its version manager is not recognised. Use NewCommandFactory to tell those cases apart.
func (m environment) RubyInstallType() InstallType {
	installType, _ := rubyInstallType(m.cmdLocator)
	return installType
}

// rubyInstallType returns the version manager of the ruby install found in the PATH.
// It returns an error wrapping ErrRubyNotFound if there is no ruby executable in the PATH, and
// Unknown with no error if ruby is installed but its version manager is not recognised.
func rubyInstallType(cmdLocator env.CommandLocator) (InstallType, error) {
	pth, err := cmdLocator.LookPath("ruby")
	if err != nil {
		return Unknown, fmt.Errorf("%w: %w", ErrRubyNotFound, err)
	}

	installType := Unknown
	// The checks below inspect the ruby path itself and are exact. The rvm, asdf and rbenv checks
	// after them probe for a version manager binary and are looser, so they have to stay last: rvm
	// in particular matches on binary presence alone, and would shadow anything placed after it.
	if pth == systemRubyPth {
		installType = SystemRuby
	} else if slices.Contains(brewRubyPths, pth) {
		installType = BrewRuby
	} else if isMiseRubyPth(pth) {
		installType = MiseRuby
	} else if _, err := cmdLocator.LookPath("rvm"); err == nil {
		installType = RVMRuby
	} else if _, err := cmdLocator.LookPath("asdf"); err == nil && strings.Contains(pth, ".asdf/shims/ruby") {
		installType = ASDFRuby
	} else if _, err := cmdLocator.LookPath("rbenv"); err == nil {
		installType = RbenvRuby
	}

	return installType, nil
}

// isMiseRubyPth reports whether pth points into a mise managed ruby install.
//
// mise exposes a tool either directly, by prepending <data dir>/installs/<tool>/<version>/bin to
// the PATH (the default), or through <data dir>/shims (opt-in via `mise activate --shims`). The
// data dir is $MISE_DATA_DIR, $XDG_DATA_HOME/mise or ~/.local/share/mise, all of which end in a
// "mise" path segment.
//
// The check deliberately does not look for the mise binary in the PATH: a mise provisioned ruby
// can be on the PATH without mise itself. A data dir renamed via $MISE_DATA_DIR is not recognised
// and falls back to Unknown, which is a warning rather than a failure.
func isMiseRubyPth(pth string) bool {
	return strings.Contains(pth, "/mise/installs/") || strings.Contains(pth, "/mise/shims/")
}

// IsGemInstalled returns true if the specified gem version is installed
func (m environment) IsGemInstalled(gem, version string) (bool, error) {
	cmd := m.factory.Create("gem", []string{"list"}, nil)

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return false, fmt.Errorf("%s: error: %s", out, err)
	}

	return findGemInList(out, gem, version)
}

// IsSpecifiedRbenvRubyInstalled checks if the selected ruby version is installed via rbenv.
// Ruby version is set by
// 1. The RBENV_VERSION environment variable
// 2. The first .ruby-version file found by searching the directory of the script you are executing and each of its
// parent directories until reaching the root of your filesystem.
// 3.The first .ruby-version file found by searching the current working directory and each of its parent directories
// until reaching the root of your filesystem.
// 4. The global ~/.rbenv/version file. You can modify this file using the rbenv global command.
// src: https://github.com/rbenv/rbenv#choosing-the-ruby-version
func (m environment) IsSpecifiedRbenvRubyInstalled(workdir string) (bool, string, error) {
	absWorkdir, err := pathutil.NewPathModifier().AbsPath(workdir)
	if err != nil {
		return false, "", fmt.Errorf("failed to get absolute path for ( %s ), error: %s", workdir, err)
	}

	cmd := m.factory.Create("rbenv", []string{"version"}, &command.Opts{Dir: absWorkdir})
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		m.logger.Warnf("failed to check installed ruby version, %s error: %s", out, err)
	}
	return isSpecifiedRbenvRubyInstalled(out)
}

func isSpecifiedRbenvRubyInstalled(message string) (bool, string, error) {
	//
	// Not installed
	regexPattern := "rbenv: version \x60.*' is not installed" // \x60 == ` (The go linter suggested to use the hex code instead)
	reg, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse regex ( %s ) on the error message, error: %s", regexPattern, err)
	}

	var version string
	if reg.MatchString(message) {
		message := reg.FindString(message)
		version = strings.Split(strings.Split(message, "`")[1], "'")[0]
		return false, version, nil
	}

	//
	// Installed
	reg, err = regexp.Compile(`.* \(set by`)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse regex ( %s ) on the error message, error: %s", ".* \\(set by", err)
	}

	if reg.MatchString(message) {
		s := reg.FindString(message)
		version = strings.Split(s, " (set by")[0]
		return true, version, nil
	}
	return false, version, nil
}

// IsSpecifiedASDFRubyInstalled ...
func (m environment) IsSpecifiedASDFRubyInstalled(workdir string) (isInstalled bool, versionInstalled string, error error) {
	absWorkdir, err := pathutil.NewPathModifier().AbsPath(workdir)
	if err != nil {
		return false, "", fmt.Errorf("failed to get absolute path for ( %s ), error: %s", workdir, err)
	}

	cmd := m.factory.Create("asdf", []string{"current", "ruby"}, &command.Opts{Dir: absWorkdir})
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		m.logger.Warnf("failed to check installed ruby version, %s error: %s", out, err)
	}

	return isSpecifiedASDFRubyInstalled(out)
}

func isSpecifiedASDFRubyInstalled(message string) (isInstalled bool, versionInstalled string, error error) {
	regexPattern := "Not installed. Run \"asdf install ruby .*\""
	reg, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse regex ( %s ) on the error message, error: %s", regexPattern, err)
	}

	var version string
	if reg.MatchString(message) {
		//
		// Not installed
		version = strings.Split(strings.Split(message, "\"asdf install ruby ")[1], "\"")[0]
		return false, version, nil
	}
	//
	// Installed
	patternTerminator := "/"
	if strings.Contains(message, "ASDF_RUBY_VERSION") {
		patternTerminator = "ASDF_RUBY_VERSION"
	}
	version = strings.Split(strings.Split(message, "ruby ")[1], patternTerminator)[0]
	version = strings.TrimSpace(version)
	return true, version, nil
}

func findGemInList(gemList, gem, version string) (bool, error) {
	// minitest (5.10.1, 5.9.1, 5.9.0, 5.8.3, 4.7.5)
	pattern := fmt.Sprintf(`^%s \(.*%s.*\)`, gem, version)
	re := regexp.MustCompile(pattern)

	reader := bytes.NewReader([]byte(gemList))
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindString(line)
		if match != "" {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}
