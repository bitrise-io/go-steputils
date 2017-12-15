package input_test

import (
	"fmt"
	"log"
	"os"

	"github.com/kdobmayer/input"
)

type Configuration struct {

	// Env vars will be converted to the respective basic data types. If the variable
	// can't be parsed, the New command will fail.
	Name        string `env:"name"`
	BuildNumber int    `env:"build_number"`
	IsUpdate    bool   `env:"is_update"`

	// List items have to be separated by pipe '|', like: "item1|item2"
	Items []string `env:"items"`

	// Secrets are not shown in the output
	Password input.Secret `env:"password"`

	// If the env var is not set, the field will be set to the type's default value
	Empty string `env:"empty"`

	// Env vars marked as 'required' have to be set, otherwise the New command will fail
	Mandatory string `env:"mandatory" validate:"required"`

	// Path validation is checks if the file is exists in the specified path
	// and returns an error if not.
	TempFile string `env:"file" validate:"path"`

	// Dir checks if the file exists and it is a directory.
	TempDir string `env:"dir" validate:"dir"`

	// Value options can be listed using commas between the alternatives. The
	// value of the env var should be one of the options.
	ExportMethod string `env:"export_method" opts:"dev,qa,prod"`
}

var envs = map[string]string{
	"name":          "Example",
	"build_number":  "11",
	"is_update":     "yes",
	"items":         "item1|item2|item3",
	"password":      "pass1234",
	"empty":         "",
	"mandatory":     "present",
	"file":          "/etc/hosts",
	"dir":           "/tmp",
	"export_method": "dev",
}

func Example() {
	var c Configuration
	os.Clearenv()

	// Set env vars for the example.
	for env, value := range envs {
		err := os.Setenv(env, value)
		if err != nil {
			log.Fatalf("Couldn't set env vars: %v\n", err)
		}
	}

	if err := input.New(&c); err != nil {
		log.Fatalf("Couldn't create config: %v\n", err)
	}

	fmt.Println(c)
	// Output: {Example 11 true [item1 item2 item3] *****  present /etc/hosts /tmp dev}
}
