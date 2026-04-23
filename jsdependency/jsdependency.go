// Package jsdependency provides helpers to build npm/yarn install and
// remove commands for a detected JS package manager.
package jsdependency

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/v2/command"
)

// Tool identifies a JS package manager.
type Tool string

const (
	Npm  Tool = "npm"
	Yarn Tool = "yarn"
)

// CommandScope describes whether a package manager command applies globally or locally.
type CommandScope string

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

// Manager builds npm/yarn commands for a given package manager.
type Manager interface {
	AddCommand(scope CommandScope, pkgs ...string) command.Command
	RemoveCommand(scope CommandScope, pkgs ...string) command.Command
	InstallGlobalDependencyCommand(dependency, version string) ([]InstallCommand, error)
}

type manager struct {
	tool    Tool
	factory command.Factory
}

// NewManager returns a Manager that creates commands for the given JS
// package manager tool using the provided command.Factory.
func NewManager(factory command.Factory, tool Tool) Manager {
	return &manager{tool: tool, factory: factory}
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

func (m *manager) AddCommand(scope CommandScope, pkgs ...string) command.Command {
	args := commandArgs(m.tool, addCommand, scope, pkgs...)
	return m.factory.Create(args[0], args[1:], nil)
}

func (m *manager) RemoveCommand(scope CommandScope, pkgs ...string) command.Command {
	args := commandArgs(m.tool, removeCommand, scope, pkgs...)
	return m.factory.Create(args[0], args[1:], nil)
}

func (m *manager) InstallGlobalDependencyCommand(dependency, version string) ([]InstallCommand, error) {
	if dependency == "" {
		return nil, errors.New("dependency name unspecified")
	}

	cmds := []InstallCommand{{
		Command:     m.RemoveCommand(Local, dependency),
		IgnoreError: m.tool == Yarn,
	}}

	if m.tool == Yarn {
		// Yarn does not link a binary (e.g. ionic) if the same binary is
		// already installed under a different package name, so remove the
		// alternative before adding the requested one.
		ionicNames := []string{"ionic", "@ionic/cli"}
		if i := indexOf(dependency, ionicNames); i != -1 {
			other := ionicNames[1-i]
			cmds = append(cmds, InstallCommand{
				Command:     m.RemoveCommand(Global, other),
				IgnoreError: true,
			})
		}
	}

	cmds = append(cmds, InstallCommand{
		Command:     m.AddCommand(Global, dependency+"@"+version),
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

func indexOf(s string, ss []string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}
