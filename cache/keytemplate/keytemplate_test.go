package keytemplate

import (
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
)

var buildContext = BuildContext{
	Workflow:   "primary",
	Branch:     "PLANG-2007-key-template–parsing",
	CommitHash: "8d722f4cc4e70373bd0b42139fa428d43e0527f0",
}

func TestEvaluate(t *testing.T) {
	type args struct {
		input        string
		buildContext BuildContext
		envVars      map[string]string
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
				input:        "my-cache-key",
				buildContext: buildContext,
			},
			want:    "my-cache-key",
			wantErr: false,
		},
		{
			name: "Key with variables",
			args: args{
				input:        "npm-cache-{{ .OS }}-{{ .Arch }}-{{ .Branch }}",
				buildContext: buildContext,
			},
			want:    "npm-cache-darwin-arm64-PLANG-2007-key-template–parsing",
			wantErr: false,
		},
		{
			name: "Key with missing variables",
			args: args{
				input: "npm-cache-{{ .Branch }}-{{ .CommitHash }}-v1",
				buildContext: BuildContext{
					Workflow:   "primary",
					Branch:     "",
					CommitHash: "",
				},
			},
			want:    "npm-cache---v1",
			wantErr: false,
		},
		{
			name: "Key with env vars",
			args: args{
				input:        `npm-cache-{{ getenv "BUILD_TYPE" }}`,
				buildContext: buildContext,
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
				input:        `npm-cache-{{ getenv "BUILD_TYPE" }}`,
				buildContext: buildContext,
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
				input:        `gradle-cache-{{ checksum "testdata/**/*.gradle*" }}`,
				buildContext: buildContext,
			},
			want:    "gradle-cache-563cf037f336453ee1888c3dcbe1c687ebeb6c593d4d0bd57ccc5fc49daa3951",
			wantErr: false,
		},
		{
			name: "Key with multiple file checksum params",
			args: args{
				input:        `gradle-cache-{{ checksum "testdata/**/*.gradle*" "testdata/package-lock.json" }}`,
				buildContext: buildContext,
			},
			want:    "gradle-cache-f7a92b852d03a958a99e8c04b831d1e709ee2e9b7a00d851317e66d617188a8b",
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
			got, err := model.Evaluate(tt.args.input, tt.args.buildContext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() got = %v, want %v", got, tt.want)
			}
		})
	}
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
