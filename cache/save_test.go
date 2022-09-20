package cache

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
)

func Test_ProcessConfig(t *testing.T) {
	testdataAbsPath, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf(err.Error())
	}
	homeAbsPath, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf(err.Error())
	}

	tests := []struct {
		name    string
		input   SaveCacheInput
		want    saveCacheConfig
		wantErr bool
	}{
		{
			name: "Invalid key input",
			input: SaveCacheInput{
				Verbose: false,
				Key:     "  ",
				Paths:   "/dev/null",
			},
			want:    saveCacheConfig{},
			wantErr: true,
		},
		{
			name: "Single file path",
			input: SaveCacheInput{
				Verbose: false,
				Key:     "cache-key",
				Paths:   "testdata/dummy_file.txt",
			},
			want: saveCacheConfig{
				Verbose:        false,
				Key:            "cache-key",
				Paths:          []string{filepath.Join(testdataAbsPath, "dummy_file.txt")},
				APIBaseURL:     "fake cache service URL",
				APIAccessToken: "fake cache service access token",
			},
			wantErr: false,
		},
		{
			name: "Absolute path and tilde in pattern path",
			input: SaveCacheInput{
				Verbose: false,
				Key:     "cache-key",
				Paths:   "~/.bash_h*",
			},
			want: saveCacheConfig{
				Verbose:        false,
				Key:            "cache-key",
				Paths:          []string{filepath.Join(homeAbsPath, ".bash_history")},
				APIBaseURL:     "fake cache service URL",
				APIAccessToken: "fake cache service access token",
			},
			wantErr: false,
		},
		{
			name: "Multiple file paths with wildcards",
			input: SaveCacheInput{
				Verbose: false,
				Key:     "cache-key",
				Paths:   "testdata/dummy_file.txt\ntestdata/**/nested_*.txt",
			},
			want: saveCacheConfig{
				Verbose: false,
				Key:     "cache-key",
				Paths: []string{
					filepath.Join(testdataAbsPath, "dummy_file.txt"),
					filepath.Join(testdataAbsPath, "subfolder", "nested_file.txt"),
				},
				APIBaseURL:     "fake cache service URL",
				APIAccessToken: "fake cache service access token",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := saver{
				logger:       log.NewLogger(),
				pathChecker:  pathutil.NewPathChecker(),
				pathProvider: pathutil.NewPathProvider(),
				pathModifier: pathutil.NewPathModifier(),
				envRepo: fakeEnvRepo{envVars: map[string]string{
					"BITRISEIO_ABCS_API_URL":      "fake cache service URL",
					"BITRISEIO_ABCS_ACCESS_TOKEN": "fake cache service access token",
				}},
			}
			got, err := step.createConfig(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ProcessConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_evaluateKey(t *testing.T) {
	type args struct {
		keyTemplate string
		envRepo     fakeEnvRepo
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Happy path",
			args: args{
				keyTemplate: "npm-cache-{{ .Branch }}",
				envRepo: fakeEnvRepo{envVars: map[string]string{
					"BITRISE_TRIGGERED_WORKFLOW_ID": "primary",
					"BITRISE_GIT_BRANCH":            "main",
					"BITRISE_GIT_COMMIT":            "9de033412f24b70b59ca8392ccb9f61ac5af4cc3",
				}},
			},
			want:    "npm-cache-main",
			wantErr: false,
		},
		{
			name: "Empty env vars",
			args: args{
				keyTemplate: "npm-cache-{{ .Branch }}",
				envRepo:     fakeEnvRepo{},
			},
			want:    "npm-cache-",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := saver{
				logger:      log.NewLogger(),
				pathChecker: pathutil.NewPathChecker(),
				envRepo:     tt.args.envRepo,
			}
			got, err := step.evaluateKey(tt.args.keyTemplate)
			if (err != nil) != tt.wantErr {
				t.Errorf("evaluateKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("evaluateKey() got = %v, want %v", got, tt.want)
			}
		})
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
