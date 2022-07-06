package keytemplate

import (
	"bytes"
	"fmt"
	"runtime"
	"text/template"

	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
)

type Model struct {
	envRepo env.Repository
	logger  log.Logger
	os      string
	arch    string
}

type BuildContext struct {
	Workflow   string
	Branch     string
	CommitHash string
}

type templateInventory struct {
	OS         string
	Arch       string
	Workflow   string
	Branch     string
	CommitHash string
}

func NewModel(envRepo env.Repository, logger log.Logger) Model {
	return Model{
		envRepo: envRepo,
		logger:  logger,
		os:      runtime.GOOS,
		arch:    runtime.GOARCH,
	}
}

// Evaluate returns the final string from a key template and the provided build context
func (m Model) Evaluate(key string, buildContext BuildContext) (string, error) {
	funcMap := template.FuncMap{
		"getenv": m.getEnvVar,
	}

	tmpl, err := template.New("").Funcs(funcMap).Parse(key)
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}

	inventory := templateInventory{
		OS:         m.os,
		Arch:       m.arch,
		Workflow:   buildContext.Workflow,
		Branch:     buildContext.Branch,
		CommitHash: buildContext.CommitHash,
	}
	m.validateInventory(inventory)

	resultBuffer := bytes.Buffer{}
	if err := tmpl.Execute(&resultBuffer, inventory); err != nil {
		return "", err
	}
	return resultBuffer.String(), nil
}

func (m Model) getEnvVar(key string) string {
	return m.envRepo.Get(key)
}

func (m Model) validateInventory(inventory templateInventory) {
	m.warnIfEmpty("Workflow", inventory.Workflow)
	m.warnIfEmpty("Branch", inventory.Branch)
	m.warnIfEmpty("CommitHash", inventory.CommitHash)
}

func (m Model) warnIfEmpty(name, value string) {
	if value == "" {
		m.logger.Warnf("Template variable .%s is not defined", name)
	}
}
