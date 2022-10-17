package cache

import (
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/assert"
)

func Test_canSkipSave(t *testing.T) {
	type args struct {
		keyTemplate       string
		evaluatedKey      string
		onlyCheckCacheKey bool
	}
	tests := []struct {
		name       string
		args       args
		envs       map[string]string
		want       bool
		wantReason skipReason
	}{
		{
			name: "No cache hit, dynamic key",
			envs: map[string]string{
				"BITRISE_GIT_COMMIT": "8d722f4cc4e70373bd0b42139fa428d43e0527f0",
			},
			args: args{
				keyTemplate:       "my-cache-key-{{ .CommitHash }}",
				evaluatedKey:      "my-cache-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				onlyCheckCacheKey: true,
			},
			want:       false,
			wantReason: reasonNoRestore,
		},
		{
			name: "Cache hit on different keys",
			envs: map[string]string{
				"BITRISE_CACHE_HIT__gradle-cache": "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
				"BITRISE_GIT_COMMIT":              "8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				"BITRISE_CACHE_HIT__static-key":   "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			args: args{
				keyTemplate:       "npm-cache-{{ .CommitHash }}",
				evaluatedKey:      "npm-cache-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				onlyCheckCacheKey: true,
			},
			want:       false,
			wantReason: reasonRestoreOtherKey,
		},
		{
			name: "Cache hit on multiple keys, one is same key",
			envs: map[string]string{
				"BITRISE_CACHE_HIT__gradle-cache": "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
				"BITRISE_GIT_COMMIT":              "8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				"BITRISE_CACHE_HIT__my-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0": "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			args: args{
				keyTemplate:       "my-key-{{ .CommitHash }}",
				evaluatedKey:      "my-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				onlyCheckCacheKey: true,
			},
			want:       true,
			wantReason: reasonRestoreSameUniqueKey,
		},
		{
			name: "Cache hit on static key",
			envs: map[string]string{
				"BITRISE_CACHE_HIT__static-key": "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			args: args{
				keyTemplate:       "static-key",
				evaluatedKey:      "static-key",
				onlyCheckCacheKey: false,
			},
			want:       false,
			wantReason: reasonKeyNotDynamic,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envRepo := fakeEnvRepo{envVars: tt.envs}
			s := &saver{
				envRepo:      envRepo,
				logger:       log.NewLogger(),
				pathProvider: pathutil.NewPathProvider(),
				pathModifier: pathutil.NewPathModifier(),
				pathChecker:  pathutil.NewPathChecker(),
			}
			canSkipSave, reason := s.canSkipSave(tt.args.keyTemplate, tt.args.evaluatedKey, tt.args.onlyCheckCacheKey)
			assert.Equalf(t, tt.want, canSkipSave, "canSkipSave(%v, %v, %v)", tt.args.keyTemplate, tt.args.evaluatedKey, tt.args.onlyCheckCacheKey)
			assert.Equalf(t, tt.wantReason.String(), reason.String(), "canSkipSave(%v, %v, %v)", tt.args.keyTemplate, tt.args.evaluatedKey, tt.args.onlyCheckCacheKey)
		})
	}
}

func Test_canSkipUpload(t *testing.T) {
	type args struct {
		newCacheKey      string
		newCacheChecksum string
	}
	tests := []struct {
		name       string
		args       args
		envs       map[string]string
		want       bool
		wantReason skipReason
	}{
		{
			name: "No cache hit",
			envs: map[string]string{},
			args: args{
				newCacheKey:      "my-cache-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				newCacheChecksum: "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			want:       false,
			wantReason: reasonNoRestore,
		},
		{
			name: "Cache hit on different keys",
			envs: map[string]string{
				"BITRISE_CACHE_HIT__gradle-cache": "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
				"BITRISE_CACHE_HIT__static-key":   "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			args: args{
				newCacheKey:      "npm-cache-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				newCacheChecksum: "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			want:       false,
			wantReason: reasonRestoreOtherKey,
		},
		{
			name: "Cache hit on same key, checksum matches",
			envs: map[string]string{
				"BITRISE_CACHE_HIT__gradle-cache":                                    "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
				"BITRISE_CACHE_HIT__my-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0": "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			args: args{
				newCacheKey:      "my-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				newCacheChecksum: "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			want:       true,
			wantReason: reasonNewArchiveChecksumMatch,
		},
		{
			name: "Cache hit on same key, checksum is different",
			envs: map[string]string{
				"BITRISE_CACHE_HIT__gradle-cache":                                    "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
				"BITRISE_CACHE_HIT__my-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0": "9a30a503b2862c51c3c5acd7fbce2f1f784cf4658ccf8e87d5023a90c21c0714",
			},
			args: args{
				newCacheKey:      "my-key-8d722f4cc4e70373bd0b42139fa428d43e0527f0",
				newCacheChecksum: "6717e97f16450f0a6bb02213484ee34dd67dcda51e8660de0a0388e77c131654",
			},
			want:       false,
			wantReason: reasonNewArchiveChecksumMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envRepo := fakeEnvRepo{envVars: tt.envs}
			s := &saver{
				envRepo:      envRepo,
				logger:       log.NewLogger(),
				pathProvider: pathutil.NewPathProvider(),
				pathModifier: pathutil.NewPathModifier(),
				pathChecker:  pathutil.NewPathChecker(),
			}
			canSkipUpload, reason := s.canSkipUpload(tt.args.newCacheKey, tt.args.newCacheChecksum)
			assert.Equalf(t, tt.want, canSkipUpload, "canSkipUpload(%v, %v)", tt.args.newCacheKey, tt.args.newCacheChecksum)
			assert.Equalf(t, tt.wantReason.String(), reason.String(), "canSkipUpload(%v, %v)", tt.args.newCacheKey, tt.args.newCacheChecksum)
		})
	}
}
