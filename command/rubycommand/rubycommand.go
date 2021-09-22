package rubycommand

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/pathutil"
)

const (
	systemRubyPth  = "/usr/bin/ruby"
	brewRubyPth    = "/usr/local/bin/ruby"
	brewRubyPthAlt = "/usr/local/opt/ruby/bin/ruby"
)

// InstallType ...
type InstallType int8

const (
	// Unkown ...
	Unkown InstallType = iota
	// SystemRuby ...
	SystemRuby
	// BrewRuby ...
	BrewRuby
	// RVMRuby ...
	RVMRuby
	// RbenvRuby ...
	RbenvRuby
)

type factory struct {
	params []string
	command.Factory
}

func NewFactory(repository env.Repository) (command.Factory, error) {
	var params []string

	rubyInstallType := RubyInstallType()
	if rubyInstallType == Unkown {
		return nil, errors.New("unknown ruby installation type")
	}

	if sudoNeeded(rubyInstallType, params...) {
		params = append([]string{"sudo"}, params...)
	}
	f := command.NewFactory(repository)

	return factory{Factory: f, params: params}, nil
}

func (f factory) Create(name string, args []string, opts *command.Opts) command.Command {
	params := f.params
	params = append(params, name)
	params = append(params, args...)

	return f.Factory.Create(params[0], params[1:], opts)
}

// CreateGemInstall ...
func (f factory) CreateGemInstall(gem, version string, enablePrerelease bool, opts *command.Opts) ([]command.Command, error) {
	s := gemInstallCommand(gem, version, enablePrerelease)
	cmd := f.Create(s[0], s[1:], opts)
	cmds := []command.Command{cmd}

	rubyInstallType := RubyInstallType()
	if rubyInstallType == RbenvRuby {
		cmd := f.Create("rbenv", []string{"rehash"}, nil)
		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

// CreateGemUpdate ...
func (f factory) CreateGemUpdate(gem string, opts *command.Opts) ([]command.Command, error) {
	var cmds []command.Command
	cmd := f.Create("gem", []string{"update", gem, "--no-document"}, opts)
	cmds = append(cmds, cmd)

	rubyInstallType := RubyInstallType()
	if rubyInstallType == RbenvRuby {
		cmd := f.Create("rbenv", []string{"rehash"}, nil)
		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

// RubyInstallType returns which version manager was used for the ruby install
func RubyInstallType() InstallType {
	whichRuby, err := command.NewFactory(env.NewRepository()).Create("which", []string{"ruby"}, nil).RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return Unkown
	}

	installType := Unkown
	if whichRuby == systemRubyPth {
		installType = SystemRuby
	} else if whichRuby == brewRubyPth {
		installType = BrewRuby
	} else if whichRuby == brewRubyPthAlt {
		installType = BrewRuby
	} else if _, err := exec.LookPath("rvm"); err == nil {
		installType = RVMRuby
	} else if _, err := exec.LookPath("rbenv"); err == nil {
		installType = RbenvRuby
	}

	return installType
}

func sudoNeeded(installType InstallType, slice ...string) bool {
	if installType != SystemRuby {
		return false
	}

	if len(slice) < 2 {
		return false
	}

	name := slice[0]
	if name == "bundle" {
		cmd := slice[1]
		/*
			bundle command can contain version:
			`bundle _2.0.1_ install`
		*/
		const bundleVersionMarker = "_"
		if strings.HasPrefix(slice[1], bundleVersionMarker) && strings.HasSuffix(slice[1], bundleVersionMarker) {
			if len(slice) < 3 {
				return false
			}
			cmd = slice[2]
		}

		return cmd == "install" || cmd == "update"
	} else if name == "gem" {
		cmd := slice[1]
		return cmd == "install" || cmd == "uninstall"
	}

	return false
}

func gemInstallCommand(gem, version string, enablePrerelease bool) []string {
	slice := []string{"gem", "install", gem, "--no-document"}
	if enablePrerelease {
		slice = append(slice, "--prerelease")
	}
	if version != "" {
		slice = append(slice, "-v", version)
	}

	return slice
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

// IsGemInstalled ...
func IsGemInstalled(gem, version string) (bool, error) {
	f, err := NewFactory(env.NewRepository())
	if err != nil {
		return false, err
	}

	cmd := f.Create("gem", []string{"list"}, nil)

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return false, fmt.Errorf("%s: error: %s", out, err)
	}

	return findGemInList(out, gem, version)
}

func isSpecifiedRbenvRubyInstalled(message string) (bool, string, error) {
	//
	// Not installed
	reg, err := regexp.Compile("rbenv: version \x60.*' is not installed") // \x60 == ` (The go linter suggested to use the hex code instead)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse regex ( %s ) on the error message, error: %s", "rbenv: version \x60.*' is not installed", err) // \x60 == ` (The go linter suggested to use the hex code instead)
	}

	var version string
	if reg.MatchString(message) {
		message := reg.FindString(message)
		version = strings.Split(strings.Split(message, "`")[1], "'")[0]
		return false, version, nil
	}

	//
	// Installed
	reg, err = regexp.Compile(".* \\(set by")
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

// IsSpecifiedRbenvRubyInstalled checks if the selected ruby version is installed via rbenv.
// Ruby version is set by
// 1. The RBENV_VERSION environment variable
// 2. The first .ruby-version file found by searching the directory of the script you are executing and each of its
// parent directories until reaching the root of your filesystem.
// 3.The first .ruby-version file found by searching the current working directory and each of its parent directories
// until reaching the root of your filesystem.
// 4. The global ~/.rbenv/version file. You can modify this file using the rbenv global command.
// src: https://github.com/rbenv/rbenv#choosing-the-ruby-version
func IsSpecifiedRbenvRubyInstalled(workdir string) (bool, string, error) {
	absWorkdir, err := pathutil.AbsPath(workdir)
	if err != nil {
		return false, "", fmt.Errorf("failed to get absolute path for ( %s ), error: %s", workdir, err)
	}

	f := command.NewFactory(env.NewRepository())
	cmd := f.Create("rbenv", []string{"version"}, &command.Opts{Dir: absWorkdir})
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return false, "", fmt.Errorf("failed to check installed ruby version, %s error: %s", out, err)
	}
	return isSpecifiedRbenvRubyInstalled(out)
}
