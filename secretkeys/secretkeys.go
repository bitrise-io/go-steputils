package secretkeys

import (
	"strings"

	"github.com/bitrise-io/go-utils/v2/env"
)

const (
	EnvKey    = "BITRISE_SECRET_ENV_KEY_LIST"
	separator = ","
)

type Manager interface {
	Load(envRepository env.Repository) []string
	Format(keys []string) string
}

type manager struct {
}

func NewManager() Manager {
	return manager{}
}

func (manager) Load(envRepository env.Repository) []string {
	value := envRepository.Get(EnvKey)
	keys := strings.Split(value, separator)
	return keys
}

func (manager) Format(keys []string) string {
	return strings.Join(keys, separator)
}
