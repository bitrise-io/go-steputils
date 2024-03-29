package stepconf_test

import (
	"fmt"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-steputils/v2/stepconf/mocks"
)

type config struct {
	// Env vars specified in the struct tags are converted to the respective basic data types.
	Name        string `env:"name"`
	BuildNumber int    `env:"build_number"`
	IsUpdate    bool   `env:"is_update"`

	// List items have to be separated by pipe '|', like: "item1|item2".
	Items []string `env:"items"`

	// Secrets are not shown in the output.
	Password stepconf.Secret `env:"password"`

	// If the env var is not set, the field will be set to the type's default value.
	Empty string `env:"empty"`

	// Env vars marked as 'required' has to be set.
	Mandatory string `env:"mandatory,required"`

	// File validation checks if the file exists in the specified path.
	TempFile string `env:"tmpfile,file"`

	// Dir checks if the file exists and it is a directory.
	TempDir string `env:"tmpdir,dir"`

	// Value options can be listed using the notation "opt[opt1,opt2,opt3]".
	// The value of the env var should be one of the options.
	ExportMethod string `env:"export_method,opt[dev,qa,prod]"`

	// Version
	Version string `env:"version,range[0..9]"`
}

var envs = map[string]string{
	"name":          "Example",
	"build_number":  "11",
	"is_update":     "yes",
	"items":         "item1|item2|item3",
	"password":      "pass1234",
	"empty":         "",
	"mandatory":     "present",
	"tmpfile":       "/etc/hosts",
	"tmpdir":        "/tmp",
	"export_method": "dev",
	"version":       "",
}

func TestExample(t *testing.T) {
	var cfg config

	envGetter := new(mocks.Repository)
	for key, value := range envs {
		envGetter.On("Get", key).Return(value)
	}

	if err := stepconf.NewInputParser(envGetter).Parse(&cfg); err != nil {
		t.Errorf("Couldn't create config: %v\n", err)
	}
	fmt.Println(cfg)
	// Output: {Example 11 true [item1 item2 item3] *****  present /etc/hosts /tmp dev}
}
