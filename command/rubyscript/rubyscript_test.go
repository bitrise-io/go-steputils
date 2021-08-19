package rubyscript

import (
	"github.com/bitrise-io/go-steputils/command/rubyscript/mocks"
	"github.com/bitrise-io/go-utils/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/require"
)

const gemfileContent = `# frozen_string_literal: true
source "https://rubygems.org"

gem "json"
`

const gemfileLockContent = `GEM
  remote: https://rubygems.org/
  specs:
    json (2.3.0)

PLATFORMS
  ruby

DEPENDENCIES
  json

BUNDLED WITH
   1.15.3
`

const rubyScriptWithGemContent = `require 'json'

begin
  messageObj = '{"message":"Hi Bitrise"}'
  messageJSON = JSON.parse(messageObj)
  puts "#{{ :data =>  messageJSON['message'] }.to_json}"
rescue => e
	puts "#{{ :error => e.to_s }.to_json}"
end
`

const rubyScriptContent = `puts '{"data":"Hi Bitrise"}'`

func TestNew(t *testing.T) {
	t.Log("initialize new ruby script runner with the ruby script content")
	{
		runner := New(rubyScriptContent)
		require.NotNil(t, runner)
	}
}

func Test_ensureTmpDir(t *testing.T) {
	t.Log("ensure runner holds a tmp dir path")
	{
		runner := New(rubyScriptContent)
		require.NotNil(t, runner)

		tmpDir, err := runner.ensureTmpDir()
		require.NoError(t, err)

		exist, err := pathutil.IsDirExists(tmpDir)
		require.NoError(t, err)
		require.True(t, exist)
	}
}

func TestBundleInstallCommand(t *testing.T) {
	t.Log("bundle install gems")
	{
		mockFactory := new(mocks.Factory)
		mockCommand := new(mocks.Command)
		mockCommand.On("Run").Return(nil)
		mockFactory.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(mockCommand)
		temporaryFactory = mockFactory
		pathProvider = func(rootPath string, file string) string {
			return file
		}

		runner := New(rubyScriptWithGemContent)
		require.NotNil(t, runner)

		bundleInstallCmd, err := runner.BundleInstallCommand(gemfileContent, gemfileLockContent)
		require.NoError(t, err)

		called := false
		for _, call := range mockFactory.Calls {
			if call.Method == "Create" && len(call.Arguments) == 3 && call.Arguments[0] == "bundle" {
				s, ok := call.Arguments[1].([]string)
				if ok && len(s) == 2 {
					if s[0] == "install" {
						split := strings.Split(s[1], "=")
						if len(split) == 2 && split[0] == "--gemfile" && filepath.Base(split[1]) == "Gemfile" {
							called = true
						}
					}
				}
			}
		}
		if !called {
			assert.Fail(t, "No invocation of Create with right arguments")
		}

		require.NoError(t, bundleInstallCmd.Run())
	}
}

func TestRunScriptCommand(t *testing.T) {
	mockFactory := new(mocks.Factory)
	mockCommand := new(mocks.Command)
	mockCommand.On("Run").Return(nil)
	mockCommand.On("RunAndReturnTrimmedCombinedOutput").Return("{\"data\":\"Hi Bitrise\"}", nil)
	mockFactory.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(mockCommand)
	temporaryFactory = mockFactory
	pathProvider = func(rootPath string, file string) string {
		return file
	}

	t.Log("runs 'ruby script.rb'")
	{
		runner := New(rubyScriptContent)
		require.NotNil(t, runner)

		runCmd, err := runner.RunScriptCommand()
		require.NoError(t, err)

		mockFactory.AssertCalled(t, "Create", "ruby", []string{"script.rb"}, (*command.Opts)(nil))

		out, err := runCmd.RunAndReturnTrimmedCombinedOutput()
		require.NoError(t, err)
		require.Equal(t, `{"data":"Hi Bitrise"}`, out)
	}

	t.Log("run 'bundle exec ruby script.rb', if Gemfile installed with bundler")
	{
		runner := New(rubyScriptWithGemContent)
		require.NotNil(t, runner)

		bundleInstallCmd, err := runner.BundleInstallCommand(gemfileContent, gemfileLockContent)
		require.NoError(t, err)
		require.NoError(t, bundleInstallCmd.Run())

		runCmd, err := runner.RunScriptCommand()
		require.NoError(t, err)

		mockFactory.AssertCalled(t, "Create", "bundle", []string{"install", "--gemfile=Gemfile"}, (*command.Opts)(nil))
		mockFactory.AssertCalled(t, "Create", "bundle", []string{"exec", "ruby", "script.rb"}, &command.Opts{Env: []string{"BUNDLE_GEMFILE=Gemfile"}})

		out, err := runCmd.RunAndReturnTrimmedCombinedOutput()
		require.NoError(t, err, out)
		require.Equal(t, `{"data":"Hi Bitrise"}`, out)
	}
}
