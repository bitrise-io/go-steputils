package ruby

import (
	"reflect"
	"testing"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"

	"github.com/stretchr/testify/require"
)

func TestSudoNeeded(t *testing.T) {
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

func TestFindGemInList(t *testing.T) {
	t.Log("finds gem")
	{
		gemList := `
*** LOCAL GEMS ***

addressable (2.5.0, 2.4.0, 2.3.8)
activesupport (5.0.0.1, 4.2.7.1, 4.2.6, 4.2.5, 4.1.16, 4.0.13)
angularjs-rails (1.5.8)`

		found, err := findGemInList(gemList, "activesupport", "")
		require.NoError(t, err)
		require.Equal(t, true, found)
	}

	t.Log("finds gem with version")
	{
		gemList := `
*** LOCAL GEMS ***

addressable (2.5.0, 2.4.0, 2.3.8)
activesupport (5.0.0.1, 4.2.7.1, 4.2.6, 4.2.5, 4.1.16, 4.0.13)
angularjs-rails (1.5.8)`

		found, err := findGemInList(gemList, "activesupport", "4.2.5")
		require.NoError(t, err)
		require.Equal(t, true, found)
	}

	t.Log("gem version not found in list")
	{
		gemList := `
*** LOCAL GEMS ***

addressable (2.5.0, 2.4.0, 2.3.8)
activesupport (5.0.0.1, 4.2.7.1, 4.2.6, 4.2.5, 4.1.16, 4.0.13)
angularjs-rails (1.5.8)`

		found, err := findGemInList(gemList, "activesupport", "0.9.0")
		require.NoError(t, err)
		require.Equal(t, false, found)
	}

	t.Log("gem not found in list")
	{
		gemList := `
*** LOCAL GEMS ***

addressable (2.5.0, 2.4.0, 2.3.8)
activesupport (5.0.0.1, 4.2.7.1, 4.2.6, 4.2.5, 4.1.16, 4.0.13)
angularjs-rails (1.5.8)`

		found, err := findGemInList(gemList, "fastlane", "")
		require.NoError(t, err)
		require.Equal(t, false, found)
	}

	t.Log("gem with version not found in list")
	{
		gemList := `
*** LOCAL GEMS ***

addressable (2.5.0, 2.4.0, 2.3.8)
activesupport (5.0.0.1, 4.2.7.1, 4.2.6, 4.2.5, 4.1.16, 4.0.13)
angularjs-rails (1.5.8)`

		found, err := findGemInList(gemList, "fastlane", "2.70")
		require.NoError(t, err)
		require.Equal(t, false, found)
	}
}

func Test_isSpecifiedRbenvRubyInstalled(t *testing.T) {

	t.Log("RBENV_VERSION installed -  2.3.5 (set by RBENV_VERSION environment variable)")
	{
		message := "2.3.5 (set by RBENV_VERSION environment variable)"
		installed, version, err := isSpecifiedRbenvRubyInstalled(message)
		require.NoError(t, err)
		require.Equal(t, true, installed)
		require.Equal(t, "2.3.5", version)
	}

	t.Log("RBENV_VERSION not installed - rbenv: version `2.34.0' is not installed (set by RBENV_VERSION environment variable)")
	{
		message := "rbenv: version `2.34.0' is not installed (set by RBENV_VERSION environment variable)"
		installed, version, err := isSpecifiedRbenvRubyInstalled(message)
		require.NoError(t, err)
		require.Equal(t, false, installed)
		require.Equal(t, "2.34.0", version)
	}

	t.Log("Global ruby installed - 2.3.5 (set by /Users/Vagrant/.rbenv/version)")
	{

		message := "2.3.5 (set by /Users/Vagrant/.rbenv/version)"
		installed, version, err := isSpecifiedRbenvRubyInstalled(message)
		require.NoError(t, err)
		require.Equal(t, true, installed)
		require.Equal(t, "2.3.5", version)
	}

	t.Log("Global ruby not installed - rbenv: version `2.4.2' is not installed (set by /Users/Vagrant/.rbenv/version)")
	{

		message := "rbenv: version `2.4.2' is not installed (set by /Users/Vagrant/.rbenv/version)"
		installed, version, err := isSpecifiedRbenvRubyInstalled(message)
		require.NoError(t, err)
		require.Equal(t, false, installed)
		require.Equal(t, "2.4.2", version)
	}

	t.Log(".ruby-version not installed - rbenv: version `2.89.2' is not installed (set by /Users/Vagrant/.ruby-version)")
	{

		message := "rbenv: version `2.89.2' is not installed (set by /Users/Vagrant/.ruby-version)"
		installed, version, err := isSpecifiedRbenvRubyInstalled(message)
		require.NoError(t, err)
		require.Equal(t, false, installed)
		require.Equal(t, "2.89.2", version)
	}

	t.Log(".ruby-version installed 2.3.5 (set by /Users/Vagrant/.ruby-version)")
	{

		message := "2.3.5 (set by /Users/Vagrant/.ruby-version)"
		installed, version, err := isSpecifiedRbenvRubyInstalled(message)
		require.NoError(t, err)
		require.Equal(t, true, installed)
		require.Equal(t, "2.3.5", version)
	}
}

func Test_gemInstallCommand(t *testing.T) {
	tests := []struct {
		name             string
		gem              string
		version          string
		enablePrerelease bool
		want             []string
	}{
		{
			name:             "latest",
			gem:              "fastlane",
			version:          "",
			enablePrerelease: false,
			want:             []string{"install", "fastlane", "--no-document"},
		},
		{
			name:             "latest including prerelease",
			gem:              "fastlane",
			version:          "",
			enablePrerelease: true,
			want:             []string{"install", "fastlane", "--no-document", "--prerelease"},
		},
		{
			name:             "version range including prerelease",
			gem:              "fastlane",
			version:          ">=2.149.1",
			enablePrerelease: true,
			want:             []string{"install", "fastlane", "--no-document", "--prerelease", "-v", ">=2.149.1"},
		},
		{
			name:             "fixed version",
			gem:              "fastlane",
			version:          "2.149.1",
			enablePrerelease: false,
			want:             []string{"install", "fastlane", "--no-document", "-v", "2.149.1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gemInstallCommandArgs(tt.gem, tt.version, tt.enablePrerelease, false)
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
