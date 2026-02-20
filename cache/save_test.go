package cache

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
)

func Test_ProcessSaveConfig(t *testing.T) {
	testdataAbsPath, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get test data absolute path, error: %s", err)
	}
	homeAbsPath, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home absolute path, error: %s", err)
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
				Verbose:          false,
				Key:              "  ",
				Paths:            []string{"/dev/null"},
				CompressionLevel: 1,
			},
			want:    saveCacheConfig{},
			wantErr: true,
		},
		{
			name: "Key with template",
			input: SaveCacheInput{
				Verbose: false,
				Key:     `test-key-{{ getenv "INTEGRATION_TEST_ENV" }}`,
				Paths:   []string{"/dev/null"},
			},
			want: saveCacheConfig{
				Verbose:          false,
				Key:              "test-key-test_value",
				Paths:            []string{"/dev/null"},
				APIBaseURL:       "fake cache service URL",
				APIAccessToken:   "fake cache service access token",
				CompressionLevel: 3,
			},
			wantErr: false,
		},
		{
			name: "Custom tar arguments",
			input: SaveCacheInput{
				Verbose:       false,
				Key:           "cache-key",
				Paths:         []string{"testdata/dummy_file.txt"},
				CustomTarArgs: []string{"--format", "posix"},
			},
			want: saveCacheConfig{
				Verbose:          false,
				Key:              "cache-key",
				Paths:            []string{filepath.Join(testdataAbsPath, "dummy_file.txt")},
				APIBaseURL:       "fake cache service URL",
				APIAccessToken:   "fake cache service access token",
				CompressionLevel: 3,
				CustomTarArgs:    []string{"--format", "posix"},
			},
			wantErr: false,
		},
		{
			name: "Single file path",
			input: SaveCacheInput{
				Verbose: false,
				Key:     "cache-key",
				Paths:   []string{"testdata/dummy_file.txt"},
			},
			want: saveCacheConfig{
				Verbose:          false,
				Key:              "cache-key",
				Paths:            []string{filepath.Join(testdataAbsPath, "dummy_file.txt")},
				APIBaseURL:       "fake cache service URL",
				APIAccessToken:   "fake cache service access token",
				CompressionLevel: 3,
			},
			wantErr: false,
		},
		{
			name: "Absolute path and tilde in pattern path",
			input: SaveCacheInput{
				Verbose: false,
				Key:     "cache-key",
				Paths:   []string{"~/.ss*"},
			},
			want: saveCacheConfig{
				Verbose:          false,
				Key:              "cache-key",
				Paths:            []string{filepath.Join(homeAbsPath, ".ssh")},
				APIBaseURL:       "fake cache service URL",
				APIAccessToken:   "fake cache service access token",
				CompressionLevel: 3,
			},
			wantErr: false,
		},
		{
			name: "Multiple file paths with wildcards",
			input: SaveCacheInput{
				Verbose: false,
				Key:     "cache-key",
				Paths: []string{
					"testdata/dummy_file.txt",
					"testdata/**/nested_*.txt",
				},
			},
			want: saveCacheConfig{
				Verbose: false,
				Key:     "cache-key",
				Paths: []string{
					filepath.Join(testdataAbsPath, "dummy_file.txt"),
					filepath.Join(testdataAbsPath, "subfolder", "nested_file.txt"),
				},
				APIBaseURL:       "fake cache service URL",
				APIAccessToken:   "fake cache service access token",
				CompressionLevel: 3,
			},
			wantErr: false,
		},
		{
			name: "Invalid compression level < 1",
			input: SaveCacheInput{
				Verbose:          false,
				Key:              "cache-key",
				Paths:            []string{"~/.ss*"},
				CompressionLevel: -1,
			},
			want:    saveCacheConfig{},
			wantErr: true,
		},
		{
			name: "Invalid compression level > 19",
			input: SaveCacheInput{
				Verbose:          false,
				Key:              "cache-key",
				Paths:            []string{"~/.ss*"},
				CompressionLevel: 20,
			},
			want:    saveCacheConfig{},
			wantErr: true,
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
					"BITRISEIO_ABCS_API_URL":                  "fake cache service URL",
					"BITRISEIO_BITRISE_SERVICES_ACCESS_TOKEN": "fake cache service access token",
					"INTEGRATION_TEST_ENV":                    "test_value",
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
