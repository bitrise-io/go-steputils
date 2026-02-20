package cache

import (
	"fmt"

	"github.com/bitrise-io/go-utils/v2/command"
)

type fakeEnvRepo struct {
	envVars map[string]string
}

func (repo fakeEnvRepo) Get(key string) string {
	value, ok := repo.envVars[key]
	if ok {
		return value
	} else {
		return ""
	}
}

func (repo fakeEnvRepo) Set(key, value string) error {
	repo.envVars[key] = value
	return nil
}

func (repo fakeEnvRepo) Unset(key string) error {
	repo.envVars[key] = ""
	return nil
}

func (repo fakeEnvRepo) List() []string {
	envs := []string{}
	for k, v := range repo.envVars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	return envs
}

type fakeCommandFactory struct{}

func (f fakeCommandFactory) Create(name string, args []string, opts *command.Opts) command.Command {
	return fakeCommand{}
}

type fakeCommand struct{}

func (c fakeCommand) PrintableCommandArgs() string                       { return "" }
func (c fakeCommand) Run() error                                         { return nil }
func (c fakeCommand) RunAndReturnExitCode() (int, error)                 { return 0, nil }
func (c fakeCommand) RunAndReturnTrimmedOutput() (string, error)         { return "", nil }
func (c fakeCommand) RunAndReturnTrimmedCombinedOutput() (string, error) { return "", nil }
func (c fakeCommand) Start() error                                       { return nil }
func (c fakeCommand) Wait() error                                        { return nil }
