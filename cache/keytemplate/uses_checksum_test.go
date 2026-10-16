package keytemplate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsesChecksum(t *testing.T) {
	tests := []struct {
		template string
		want     bool
	}{
		{template: `npm-{{ checksum "package-lock.json" }}`, want: true},
		{template: `{{ .OS }}-{{ checksum "go.sum" "**/*.gradle*" }}`, want: true},
		{template: `{{ "go.sum" | checksum }}`, want: true},
		{template: `{{ if .Branch }}{{ checksum "a" }}{{ end }}`, want: true},
		{template: `{{ with .Branch }}x{{ else }}{{ checksum "a" }}{{ end }}`, want: true},
		{template: `cache-key`, want: false},
		{template: `npm-{{ .Branch }}`, want: false},
		{template: `{{ .OS }}-{{ .Arch }}`, want: false},
		{template: `checksum-{{ .Branch }}`, want: false},
		{template: `{{ getenv "checksum" }}`, want: false},
		{template: `{{ checksum "a"`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			assert.Equal(t, tt.want, UsesChecksum(tt.template))
		})
	}
}
