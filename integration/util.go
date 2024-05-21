//go:build integration
// +build integration

package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
)

var logger = log.NewLogger()

func checksumOf(bytes []byte) string {
	hash := sha256.New()
	hash.Write(bytes)
	return hex.EncodeToString(hash.Sum(nil))
}

func listArchiveContents(path string) ([]string, error) {
	output, err := command.NewFactory(env.NewRepository()).
		Create("tar", []string{"-tf", path}, nil).
		RunAndReturnTrimmedCombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("failed to list archive contents, out: %s, error: %w", output, err)
	}

	contentList := strings.Split(output, "\n")
	for i, content := range contentList {
		contentList[i] = strings.TrimSuffix(content, string(os.PathSeparator))
	}

	return contentList, nil
}

func checkTools() {
	_, err := exec.LookPath("zstd")
	if err != nil {
		panic("zstd is required for integration tests")
	}
}

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
	var values []string
	for _, v := range repo.envVars {
		values = append(values, v)
	}
	return values
}
