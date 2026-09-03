package ruby

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/ruby/mocks"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	utilmocks "github.com/bitrise-io/go-utils/v2/mocks"
	"github.com/stretchr/testify/mock"
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
		require.Equal(t, false, sudoNeeded(ASDFRuby, "ls"))
	}

	t.Log("sudo needed for SystemRuby in case of gem list management command")
	{
		require.Equal(t, false, sudoNeeded(Unknown, "gem", "install", "fastlane"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "gem", "install", "fastlane"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "gem", "install", "fastlane"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "gem", "install", "fastlane"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "gem", "install", "fastlane"))
		require.Equal(t, false, sudoNeeded(ASDFRuby, "gem", "install", "fastlane"))

		require.Equal(t, false, sudoNeeded(Unknown, "gem", "uninstall", "fastlane"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "gem", "uninstall", "fastlane"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "gem", "uninstall", "fastlane"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "gem", "uninstall", "fastlane"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "gem", "uninstall", "fastlane"))
		require.Equal(t, false, sudoNeeded(ASDFRuby, "gem", "uninstall", "fastlane"))

		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "install"))
		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "_2.0.2_", "install"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "install"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "_2.0.2_", "install"))
		require.Equal(t, false, sudoNeeded(SystemRuby, "bundle", "_2.0.2_"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "bundle", "install"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "bundle", "install"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "bundle", "install"))
		require.Equal(t, false, sudoNeeded(ASDFRuby, "bundle", "install"))

		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "update"))
		require.Equal(t, false, sudoNeeded(Unknown, "bundle", "_2.0.2_", "update"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "update"))
		require.Equal(t, true, sudoNeeded(SystemRuby, "bundle", "_2.0.2_", "update"))
		require.Equal(t, false, sudoNeeded(BrewRuby, "bundle", "update"))
		require.Equal(t, false, sudoNeeded(RVMRuby, "bundle", "update"))
		require.Equal(t, false, sudoNeeded(RbenvRuby, "bundle", "update"))
		require.Equal(t, false, sudoNeeded(ASDFRuby, "bundle", "update"))
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

func Test_NewCommandFactory_WhenRubyNotInPath_ThenReturnsErrRubyNotFound(t *testing.T) {
	mockCommandLocator := new(mocks.CommandLocator)
	mockCommandLocator.On("LookPath", "ruby").Return("", fmt.Errorf("exit status 1"))

	factory, err := NewCommandFactory(command.NewFactory(env.NewRepository()), mockCommandLocator, new(utilmocks.Logger))

	require.ErrorIs(t, err, ErrRubyNotFound)
	require.Nil(t, factory)
}

func Test_NewCommandFactory_WhenInstallTypeIsUnknown_ThenWarnsAndReturnsUsableFactory(t *testing.T) {
	mockCommandLocator := new(mocks.CommandLocator)
	mockCommandLocator.On("LookPath", "ruby").Return("/Users/vagrant/.local/share/mise/shims/ruby", nil)
	mockCommandLocator.On("LookPath", "rbenv").Return("", fmt.Errorf("exit status 1"))
	mockCommandLocator.On("LookPath", "rvm").Return("", fmt.Errorf("exit status 1"))
	mockCommandLocator.On("LookPath", "asdf").Return("", fmt.Errorf("exit status 1"))

	mockLogger := new(utilmocks.Logger)
	mockLogger.On("Warnf", mock.Anything).Return()

	factory, err := NewCommandFactory(command.NewFactory(env.NewRepository()), mockCommandLocator, mockLogger)

	require.NoError(t, err)
	require.NotNil(t, factory)
	mockLogger.AssertCalled(t, "Warnf", mock.Anything)

	// An unrecognised install type has to behave as a plain passthrough.
	cmd := factory.Create("gem", []string{"install", "bitrise"}, nil)
	require.Equal(t, `gem "install" "bitrise"`, cmd.PrintableCommandArgs())
}

func Test_NewCommandFactory_WhenInstallTypeIsKnown_ThenReturnsFactoryWithoutWarning(t *testing.T) {
	mockCommandLocator := new(mocks.CommandLocator)
	mockCommandLocator.On("LookPath", "ruby").Return(systemRubyPth, nil)

	mockLogger := new(utilmocks.Logger)

	factory, err := NewCommandFactory(command.NewFactory(env.NewRepository()), mockCommandLocator, mockLogger)

	require.NoError(t, err)
	require.NotNil(t, factory)
	mockLogger.AssertNotCalled(t, "Warnf", mock.Anything)

	cmd := factory.Create("gem", []string{"install", "bitrise"}, nil)
	require.Equal(t, `sudo "gem" "install" "bitrise"`, cmd.PrintableCommandArgs())
}
