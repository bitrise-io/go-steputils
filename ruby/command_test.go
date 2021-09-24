package ruby

import (
	"reflect"
	"testing"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/stretchr/testify/require"
)

func Test_sudoNeeded(t *testing.T) {
	t.Log("sudo NOT need")
	{
		require.Equal(t, false, sudoNeeded(Unknown, "ls"))
		require.Equal(t, false, sudoNeeded(SystemRuby, "ls"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "ls"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "ls"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "ls"))
	}

	t.Log("sudo needed for SystemRuby in case of gem list management command")
	{
		require.Equal(t, false, sudoNeeded(Unknown, "gem", "install", "fastlane"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "gem", "install", "fastlane"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "gem", "install", "fastlane"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "gem", "install", "fastlane"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "gem", "install", "fastlane"))

		require.Equal(t, false, sudoNeeded(Unknown, "gem", "uninstall", "fastlane"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "gem", "uninstall", "fastlane"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "gem", "uninstall", "fastlane"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "gem", "uninstall", "fastlane"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "gem", "uninstall", "fastlane"))

		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "install"))
		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "_2.0.2_", "install"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "install"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "_2.0.2_", "install"))
		require.Equal(t, false, sudoNeeded(SystemRuby, "bundle", "_2.0.2_"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "bundle", "install"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "bundle", "install"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "bundle", "install"))

		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "update"))
		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "_2.0.2_", "update"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "update"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "_2.0.2_", "update"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "bundle", "update"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "bundle", "update"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "bundle", "update"))
	}
}

func Test_gemInstallCommandArgs(t *testing.T) {
	tests := []struct {
		name             string
		gem              string
		version          string
		enablePrerelease bool
		force            bool
		want             []string
	}{
		{
			name:             "latest",
			gem:              "fastlane",
			version:          "",
			enablePrerelease: false,
			force:            false,
			want:             []string{"install", "fastlane", "--no-document"},
		},
		{
			name:             "latest including prerelease",
			gem:              "fastlane",
			version:          "",
			enablePrerelease: true,
			force:            false,
			want:             []string{"install", "fastlane", "--no-document", "--prerelease"},
		},
		{
			name:             "version range including prerelease",
			gem:              "fastlane",
			version:          ">=2.149.1",
			enablePrerelease: true,
			force:            false,
			want:             []string{"install", "fastlane", "--no-document", "--prerelease", "-v", ">=2.149.1"},
		},
		{
			name:             "fixed version",
			gem:              "fastlane",
			version:          "2.149.1",
			enablePrerelease: false,
			force:            false,
			want:             []string{"install", "fastlane", "--no-document", "-v", "2.149.1"},
		},
		{
			name:             "force install",
			gem:              "fastlane",
			version:          "2.149.1",
			enablePrerelease: false,
			force:            true,
			want:             []string{"install", "fastlane", "--no-document", "-v", "2.149.1", "--force"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gemInstallCommandArgs(tt.gem, tt.version, tt.enablePrerelease, tt.force)
			require.Equal(t, tt.want, got, "gemInstallCommand() return value")
		})
	}
}

func TestFactory_Create(t *testing.T) {
	tests := []struct {
		title   string
		factory CommandFactory
		name    string
		args    []string
		opts    *command.Opts
		want    string
	}{
		{
			title:   "Command without sudo",
			factory: commandFactory{cmdFactory: command.NewFactory(env.NewRepository()), installType: RbenvRuby},
			name:    "gem",
			args:    []string{"install", "bitrise"},
			opts:    nil,
			want:    `gem "install" "bitrise"`,
		},
		{
			title:   "Command with sudo",
			factory: commandFactory{cmdFactory: command.NewFactory(env.NewRepository()), installType: SystemRuby},
			name:    "gem",
			args:    []string{"install", "bitrise"},
			opts:    nil,
			want:    `sudo "gem" "install" "bitrise"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if got := tt.factory.Create(tt.name, tt.args, tt.opts); !reflect.DeepEqual(got.PrintableCommandArgs(), tt.want) {
				t.Errorf("Create() = %v, want %v", got.PrintableCommandArgs(), tt.want)
			}
		})
	}
}
