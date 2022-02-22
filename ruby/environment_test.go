package ruby

import (
	"fmt"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/ruby/mocks"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_findGemInList(t *testing.T) {
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

func Test_environment_IsSpecifiedRbenvRubyInstalled(t *testing.T) {
	type fields struct {
		factory CommandFactory
		logger  log.Logger
	}
	tests := []struct {
		name        string
		fields      fields
		wantInstall bool
		wantVersion string
		wantErr     bool
	}{
		{name: "Parse missing ruby version even if command fails",
			fields:      fields{createFailingCommandFactory(), log.NewLogger()},
			wantInstall: false,
			wantVersion: "2.7.4",
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := environment{
				factory: tt.fields.factory,
				logger:  tt.fields.logger,
			}
			got, got1, err := m.IsSpecifiedRbenvRubyInstalled("/")
			if (err != nil) != tt.wantErr {
				t.Errorf("IsSpecifiedRbenvRubyInstalled() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantInstall {
				t.Errorf("IsSpecifiedRbenvRubyInstalled() got = %v, want %v", got, tt.wantInstall)
			}
			if got1 != tt.wantVersion {
				t.Errorf("IsSpecifiedRbenvRubyInstalled() got1 = %v, want %v", got1, tt.wantVersion)
			}
		})
	}
}

func createFailingCommandFactory() CommandFactory {
	response := `rbenv: version ` + "`" + `2.7.4' is not installed (set by /Users/vagrant/git/.ruby-version)
	(set by /Users/vagrant/git/.ruby-version)`
	mockCommand := new(mocks.Command)
	mockCommand.On("RunAndReturnTrimmedCombinedOutput").Return(response, fmt.Errorf("exit status 1"))
	mockCommandFactory := new(mocks.CommandFactory)
	mockCommandFactory.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(mockCommand)
	return mockCommandFactory
}
