package cache

import (
	"reflect"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/assert"
)

func Test_ProcessRestoreConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   RestoreCacheInput
		want    restoreCacheConfig
		wantErr bool
	}{
		{
			name: "Valid key input",
			input: RestoreCacheInput{
				Verbose: true,
				Keys:    []string{"valid-key"},
			},
			want: restoreCacheConfig{
				Verbose:        true,
				Keys:           []string{"valid-key"},
				APIBaseURL:     "fake service URL",
				APIAccessToken: "fake access token",
			},
			wantErr: false,
		},
		{
			name: "Valid key input with multiple keys",
			input: RestoreCacheInput{
				Verbose: true,
				Keys: []string{
					"valid-key",
					"valid-key-2",
				},
			},
			want: restoreCacheConfig{
				Verbose:        true,
				Keys:           []string{"valid-key", "valid-key-2"},
				APIBaseURL:     "fake service URL",
				APIAccessToken: "fake access token",
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			step := restorer{
				logger: log.NewLogger(),
				envRepo: fakeEnvRepo{envVars: map[string]string{
					"BITRISEIO_ABCS_API_URL":      "fake service URL",
					"BITRISEIO_ABCS_ACCESS_TOKEN": "fake access token",
				}},
			}

			// When
			processedConfig, err := step.createConfig(testCase.input)

			// Then
			if (err != nil) != testCase.wantErr {
				t.Errorf("ProcessConfig() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !reflect.DeepEqual(processedConfig, testCase.want) {
				t.Errorf("ProcessConfig() = %v, want %v", processedConfig, testCase.want)
			}
		})
	}
}

func Test_evaluateKeys(t *testing.T) {
	type args struct {
		keys    []string
		envRepo fakeEnvRepo
	}

	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "Happy path",
			args: args{
				keys: []string{"npm-cache-{{ .Branch }}"},
				envRepo: fakeEnvRepo{
					envVars: map[string]string{
						"BITRISE_WORKFLOW_ID": "primary",
						"BITRISE_GIT_BRANCH":  "main",
						"BITRISE_GIT_COMMIT":  "9de033412f24b70b59ca8392ccb9f61ac5af4cc3",
					},
				},
			},
			want:    []string{"npm-cache-main"},
			wantErr: false,
		},
		{
			name: "Multiple keys",
			args: args{
				keys: []string{
					"npm-cache-{{ .Branch }}",
					"npm-cache-",
					"",
				},
				envRepo: fakeEnvRepo{
					envVars: map[string]string{
						"BITRISE_WORKFLOW_ID": "primary",
						"BITRISE_GIT_BRANCH":  "main",
						"BITRISE_GIT_COMMIT":  "9de033412f24b70b59ca8392ccb9f61ac5af4cc3",
					},
				},
			},
			want: []string{
				"npm-cache-main",
				"npm-cache-",
			},
			wantErr: false,
		},
		{
			name: "Empty environment variables",
			args: args{
				keys:    []string{"npm-cache-{{ .Branch }}"},
				envRepo: fakeEnvRepo{},
			},
			want:    []string{"npm-cache-"},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			step := restorer{
				logger:  log.NewLogger(),
				envRepo: testCase.args.envRepo,
			}

			// When
			evaluatedKeys, err := step.evaluateKeys(testCase.args.keys)
			if (err != nil) != testCase.wantErr {
				t.Errorf("evaluateKey() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !reflect.DeepEqual(evaluatedKeys, testCase.want) {
				t.Errorf("evaluateKey() = %v, want %v", evaluatedKeys, testCase.want)
			}
		})
	}
}

func Test_exposeCacheHit(t *testing.T) {
	tests := []struct {
		name string
		downloadResult
		wantEnvs []string
		wantErr  bool
	}{
		{
			name:           "no cache hit",
			downloadResult: downloadResult{},
			wantEnvs:       []string{},
			wantErr:        false,
		},
		{
			name: "exact cache hit",
			downloadResult: downloadResult{
				filePath:   "testdata/dummy_file.txt",
				matchedKey: "my-cache-key",
			},
			wantEnvs: []string{
				"BITRISE_CACHE_HIT__my-cache-key=9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
		},
		{
			name: "exact cache hit",
			downloadResult: downloadResult{
				filePath:   "testdata/dummy_file.txt",
				matchedKey: "my-cache-key",
			},
			wantEnvs: []string{
				"BITRISE_CACHE_HIT__my-cache-key=9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envRepo := fakeEnvRepo{envVars: map[string]string{}}
			r := &restorer{
				envRepo: envRepo,
				logger:  log.NewLogger(),
			}
			if err := r.exposeCacheHit(tt.downloadResult); (err != nil) != tt.wantErr {
				t.Fatalf("exposeCacheHit() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.wantEnvs, envRepo.List())
		})
	}
}
