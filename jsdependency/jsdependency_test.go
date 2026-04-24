package jsdependency

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/stretchr/testify/require"
)

func Test_commandArgs(t *testing.T) {
	tests := []struct {
		name   string
		tool   Tool
		cmd    managerCommand
		scope  CommandScope
		pkgs   []string
		expect []string
	}{
		{"npm local install", Npm, addCommand, Local, []string{"foo"}, []string{"npm", "install", "foo", "--force"}},
		{"npm global install", Npm, addCommand, Global, []string{"foo@1.0"}, []string{"npm", "install", "-g", "foo@1.0", "--force"}},
		{"npm local remove", Npm, removeCommand, Local, []string{"foo"}, []string{"npm", "remove", "foo", "--force"}},
		{"yarn local add", Yarn, addCommand, Local, []string{"foo"}, []string{"yarn", "add", "foo"}},
		{"yarn global add", Yarn, addCommand, Global, []string{"foo@1.0"}, []string{"yarn", "global", "add", "foo@1.0"}},
		{"yarn local remove", Yarn, removeCommand, Local, []string{"foo"}, []string{"yarn", "remove", "foo"}},
		{"yarn global remove", Yarn, removeCommand, Global, []string{"ionic"}, []string{"yarn", "global", "remove", "ionic"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, commandArgs(tt.tool, tt.cmd, tt.scope, tt.pkgs...))
		})
	}
}

func TestDetectTool(t *testing.T) {
	dir := t.TempDir()

	tool, err := DetectTool(dir)
	require.NoError(t, err)
	require.Equal(t, Npm, tool)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte{}, 0644))

	tool, err = DetectTool(dir)
	require.NoError(t, err)
	require.Equal(t, Yarn, tool)
}

func TestInstallGlobalDependencyCommand(t *testing.T) {
	factory := command.NewFactory(env.NewRepository())

	tests := []struct {
		name         string
		tool         Tool
		dependency   string
		version      string
		wantPrinted  []string
		wantIgnore   []bool
	}{
		{
			name:        "yarn install ionic",
			tool:        Yarn,
			dependency:  "ionic",
			version:     "latest",
			wantPrinted: []string{`yarn "remove" "ionic"`, `yarn "global" "remove" "@ionic/cli"`, `yarn "global" "add" "ionic@latest"`},
			wantIgnore:  []bool{true, true, false},
		},
		{
			name:        "yarn install @ionic/cli",
			tool:        Yarn,
			dependency:  "@ionic/cli",
			version:     "latest",
			wantPrinted: []string{`yarn "remove" "@ionic/cli"`, `yarn "global" "remove" "ionic"`, `yarn "global" "add" "@ionic/cli@latest"`},
			wantIgnore:  []bool{true, true, false},
		},
		{
			name:        "yarn install cordova",
			tool:        Yarn,
			dependency:  "cordova",
			version:     "latest",
			wantPrinted: []string{`yarn "remove" "cordova"`, `yarn "global" "add" "cordova@latest"`},
			wantIgnore:  []bool{true, false},
		},
		{
			name:        "npm install @ionic/cli",
			tool:        Npm,
			dependency:  "@ionic/cli",
			version:     "latest",
			wantPrinted: []string{`npm "remove" "@ionic/cli" "--force"`, `npm "install" "-g" "@ionic/cli@latest" "--force"`},
			wantIgnore:  []bool{false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InstallGlobalDependencyCommand(factory, tt.tool, tt.dependency, tt.version)
			require.NoError(t, err)
			require.Len(t, got, len(tt.wantPrinted))
			for i, w := range tt.wantPrinted {
				require.Equal(t, w, got[i].Command.PrintableCommandArgs(), "cmd %d", i)
				require.Equal(t, tt.wantIgnore[i], got[i].IgnoreError, "ignore %d", i)
			}
		})
	}
}

func TestInstallGlobalDependencyCommand_emptyDependency(t *testing.T) {
	factory := command.NewFactory(env.NewRepository())
	_, err := InstallGlobalDependencyCommand(factory, Npm, "", "latest")
	require.Error(t, err)
}
