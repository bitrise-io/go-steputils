// Package jsdependency provides helpers to build npm/yarn install and
// remove commands for a detected JS package manager.
package jsdependency

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/bitrise-io/go-utils/v2/command"
)

// Tool identifies a JS package manager.
type Tool string

// Supported JS package manager tools.
const (
	Npm  Tool = "npm"
	Yarn Tool = "yarn"
)

// CommandScope describes whether a package manager command applies globally or locally.
type CommandScope string

// Supported command scopes.
const (
	Local  CommandScope = "local"
	Global CommandScope = "global"
)

type managerCommand string

const (
	addCommand    managerCommand = "add"
	removeCommand managerCommand = "remove"
)

// InstallCommand pairs a command with a flag indicating whether its failure
// can be ignored (e.g. yarn exits non-zero when removing a package that is
// not installed).
type InstallCommand struct {
	Command     command.Command
	IgnoreError bool
}

// DetectTool reports which JS package manager to use by checking for a
// yarn.lock file next to package.json. Falls back to Npm when absent.
func DetectTool(packageJSONDir string) (Tool, error) {
	_, err := os.Stat(filepath.Join(packageJSONDir, "yarn.lock"))
	switch {
	case err == nil:
		return Yarn, nil
	case errors.Is(err, os.ErrNotExist):
		return Npm, nil
	default:
		return Npm, fmt.Errorf("check for yarn.lock in %s: %w", packageJSONDir, err)
	}
}

// AddCommand returns the command that installs the given packages in the
// requested scope for the given tool, built via factory.
func AddCommand(factory command.Factory, tool Tool, scope CommandScope, pkgs ...string) command.Command {
	args := commandArgs(tool, addCommand, scope, pkgs...)
	return factory.Create(args[0], args[1:], nil)
}

// RemoveCommand returns the command that removes the given packages in the
// requested scope for the given tool, built via factory.
func RemoveCommand(factory command.Factory, tool Tool, scope CommandScope, pkgs ...string) command.Command {
	args := commandArgs(tool, removeCommand, scope, pkgs...)
	return factory.Create(args[0], args[1:], nil)
}

// InstallGlobalDependencyCommand returns the sequence of commands that
// removes any existing local copy and adds the dependency globally at the
// requested version.
func InstallGlobalDependencyCommand(factory command.Factory, tool Tool, dependency, version string) ([]InstallCommand, error) {
	if dependency == "" {
		return nil, errors.New("dependency name unspecified")
	}

	cmds := []InstallCommand{{
		Command:     RemoveCommand(factory, tool, Local, dependency),
		IgnoreError: tool == Yarn,
	}}

	if tool == Yarn {
		// Yarn does not link a binary (e.g. ionic) if the same binary is
		// already installed under a different package name, so remove the
		// alternative before adding the requested one.
		ionicNames := []string{"ionic", "@ionic/cli"}
		if i := slices.Index(ionicNames, dependency); i != -1 {
			other := ionicNames[1-i]
			cmds = append(cmds, InstallCommand{
				Command:     RemoveCommand(factory, tool, Global, other),
				IgnoreError: true,
			})
		}
	}

	cmds = append(cmds, InstallCommand{
		Command:     AddCommand(factory, tool, Global, dependency+"@"+version),
		IgnoreError: false,
	})

	return cmds, nil
}

func commandArgs(tool Tool, mc managerCommand, scope CommandScope, pkgs ...string) []string {
	switch tool {
	case Npm:
		args := []string{"npm", toolSubcommand(tool, mc)}
		if scope == Global {
			args = append(args, "-g")
		}
		args = append(args, pkgs...)
		args = append(args, "--force")
		return args
	case Yarn:
		args := []string{"yarn"}
		if scope == Global {
			args = append(args, "global")
		}
		args = append(args, toolSubcommand(tool, mc))
		args = append(args, pkgs...)
		return args
	}
	return nil
}

func toolSubcommand(tool Tool, mc managerCommand) string {
	if mc == removeCommand {
		return "remove"
	}
	if tool == Npm {
		return "install"
	}
	return "add"
}

