package tools

import (
	"fmt"
	"strings"

	"encoding/json"

	"github.com/bitrise-io/go-utils/command"
)

// ExportEnvironmentWithEnvman ...
func ExportEnvironmentWithEnvman(key, value string) error {
	cmd := command.New("envman", "add", "--key", key)
	cmd.SetStdin(strings.NewReader(value))
	return cmd.Run()
}

// GetEnvironmentValueWithEnvman ...
func GetEnvironmentValueWithEnvman(key string) (string, error) {
	cmd := command.New("envman", "print", "--format", "json")
	output, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s\n%s", output, err)
	}

	var data map[string]string
	err = json.Unmarshal([]byte(output), &data)
	if err != nil {
		return "", fmt.Errorf("%s\n%s", output, err)
	}

	return data[key], nil
}
