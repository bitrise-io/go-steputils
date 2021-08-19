package jsdependency

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallGlobalDependencyCommand(t *testing.T) {
	tests := []struct {
		name           string
		packageManager Tool
		dependency     string
		version        string
		want           []InstallCommand
		wantErr        bool
	}{
		{
			name:           "Install latest ionic with yarn",
			packageManager: Yarn,
			dependency:     "ionic",
			version:        "latest",
			want: []InstallCommand{
				{
					Slice:       temporaryFactory.Create("yarn", []string{"remove", "ionic"}, nil),
					IgnoreError: true,
				},
				{
					Slice:       temporaryFactory.Create("yarn", []string{"global", "remove", "@ionic/cli"}, nil),
					IgnoreError: true,
				},
				{
					Slice:       temporaryFactory.Create("yarn", []string{"global", "add", "ionic@latest"}, nil),
					IgnoreError: false,
				},
			},
		},
		{
			name:           "Install latest @ionic/cli with yarn",
			packageManager: Yarn,
			dependency:     "@ionic/cli",
			version:        "latest",
			want: []InstallCommand{
				{
					Slice:       temporaryFactory.Create("yarn", []string{"remove", "@ionic/cli"}, nil),
					IgnoreError: true,
				},
				{
					Slice:       temporaryFactory.Create("yarn", []string{"global", "remove", "ionic"}, nil),
					IgnoreError: true,
				},
				{
					Slice:       temporaryFactory.Create("yarn", []string{"global", "add", "@ionic/cli@latest"}, nil),
					IgnoreError: false,
				},
			},
		},
		{
			name:           "Install latest corodva with yarn",
			packageManager: Yarn,
			dependency:     "cordova",
			version:        "latest",
			want: []InstallCommand{
				{
					Slice:       temporaryFactory.Create("yarn", []string{"remove", "cordova"}, nil),
					IgnoreError: true,
				},
				{
					Slice:       temporaryFactory.Create("yarn", []string{"global", "add", "cordova@latest"}, nil),
					IgnoreError: false,
				},
			},
		},
		{
			name:           "Install latest @ionic/cli with npm",
			packageManager: Npm,
			dependency:     "@ionic/cli",
			version:        "latest",
			want: []InstallCommand{
				{
					Slice:       temporaryFactory.Create("npm", []string{"remove", "@ionic/cli", "--force"}, nil),
					IgnoreError: false,
				},
				{
					Slice:       temporaryFactory.Create("npm", []string{"install", "-g", "@ionic/cli@latest", "--force"}, nil),
					IgnoreError: false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InstallGlobalDependencyCommand(tt.packageManager, tt.dependency, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("InstallGlobalDependencyCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got, "InstallGlobalDependencyCommand() return value")
		})
	}
}
