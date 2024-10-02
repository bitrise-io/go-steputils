package keytemplate

import (
	"testing"

	"fmt"
	"github.com/bitrise-io/go-utils/v2/log"
	"os"
	"path/filepath"
)

var triggerEnvVars = map[string]string{
	"BITRISE_TRIGGERED_WORKFLOW_ID": "primary",
	"BITRISE_GIT_BRANCH":            "PLANG-2007-key-template–parsing",
	"BITRISE_GIT_COMMIT":            "8d722f4cc4e70373bd0b42139fa428d43e0527f0",
}

func TestEvaluate(t *testing.T) {
	type args struct {
		input   string
		envVars map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Static key",
			args: args{
				input:   "my-cache-key",
				envVars: triggerEnvVars,
			},
			want:    "my-cache-key",
			wantErr: false,
		},
		{
			name: "Key with variables",
			args: args{
				input:   "npm-cache-{{ .OS }}-{{ .Arch }}-{{ .Branch }}",
				envVars: triggerEnvVars,
			},
			want:    "npm-cache-darwin-arm64-PLANG-2007-key-template–parsing",
			wantErr: false,
		},
		{
			name: "Key with missing variables",
			args: args{
				input: "npm-cache-{{ .Branch }}-{{ .CommitHash }}-v1",
				envVars: map[string]string{
					"BITRISE_TRIGGERED_WORKFLOW_ID": "primary",
				},
			},
			want:    "npm-cache---v1",
			wantErr: false,
		},
		{
			name: "Key with env vars",
			args: args{
				input: `npm-cache-{{ getenv "BUILD_TYPE" }}`,
				envVars: map[string]string{
					"BUILD_TYPE":  "release",
					"ANOTHER_ENV": "false",
				},
			},
			want:    "npm-cache-release",
			wantErr: false,
		},
		{
			name: "Key with missing env var",
			args: args{
				input: `npm-cache-{{ getenv "BUILD_TYPE" }}`,
				envVars: map[string]string{
					"ANOTHER_ENV": "false",
				},
			},
			want:    "npm-cache-",
			wantErr: false,
		},
		{
			name: "Key with file checksum",
			args: args{
				input:   `gradle-cache-{{ checksum "testdata/**/*.gradle*" }}`,
				envVars: triggerEnvVars,
			},
			want:    "gradle-cache-563cf037f336453ee1888c3dcbe1c687ebeb6c593d4d0bd57ccc5fc49daa3951",
			wantErr: false,
		},
		{
			name: "Key with multiple file checksum params",
			args: args{
				input:   `gradle-cache-{{ checksum "testdata/**/*.gradle*" "testdata/package-lock.json" }}`,
				envVars: triggerEnvVars,
			},
			want:    "gradle-cache-f7a92b852d03a958a99e8c04b831d1e709ee2e9b7a00d851317e66d617188a8b",
			wantErr: false,
		},
		{
			name: "No explicit commit hash",
			args: args{
				input: "cache-key-{{ .CommitHash }}",
				envVars: map[string]string{
					"BITRISE_GIT_COMMIT":    "",
					"GIT_CLONE_COMMIT_HASH": "8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				},
			},
			want:    "cache-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := Model{
				envRepo: envRepository{envVars: tt.args.envVars},
				logger:  log.NewLogger(),
				os:      "darwin",
				arch:    "arm64",
			}
			got, err := model.Evaluate(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() got = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("Key with home file", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		tmpDirPath := filepath.Join(home, "/go-steputils-test/")
		err = os.MkdirAll(tmpDirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer func(path string) {
			err := os.RemoveAll(path)
			if err != nil {
				t.Errorf("Failed to remove temp directory: %v", err)
			}
		}(tmpDirPath)

		tmpFile, err := os.Create(filepath.Join(tmpDirPath, "testfile"))
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		_, err = tmpFile.WriteString("test")
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		err = tmpFile.Close()
		if err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}

		checksum, err := checksumOfFile(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to calculate checksum: %v", err)
		}

		l := log.NewLogger()
		l.EnableDebugLog(true)
		l.Debugf("Tempfile: %s", tmpFile.Name())
		model := Model{
			envRepo: envRepository{},
			logger:  l,
			os:      "darwin",
			arch:    "arm64",
		}
		got, err := model.Evaluate(`gradle-cache-{{ checksum "~/go-steputils-test/*" }}`)
		if err != nil {
			t.Errorf("Evaluate() error = %v", err)
			return
		}

		want := fmt.Sprintf("gradle-cache-%x", checksum)
		if got != want {
			t.Errorf("Evaluate() got = %v, want %v", got, want)
		}
	})
}

type envRepository struct {
	envVars map[string]string
}

func (repo envRepository) Get(key string) string {
	value, ok := repo.envVars[key]
	if ok {
		return value
	}
	return ""
}

func (repo envRepository) Set(key, value string) error {
	repo.envVars[key] = value
	return nil
}

func (repo envRepository) Unset(key string) error {
	repo.envVars[key] = ""
	return nil
}

func (repo envRepository) List() []string {
	var values []string
	for _, v := range repo.envVars {
		values = append(values, v)
	}
	return values
}
