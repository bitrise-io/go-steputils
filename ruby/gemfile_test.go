package ruby

import (
	"reflect"
	"testing"
)

func Test_ParseVersionFromBundle(t *testing.T) {
	tests := []struct {
		name               string
		gemName            string
		gemfileLockContent string
		wantGemVersion     Version
		wantErr            bool
	}{
		{
			name:               "Parse fastlane version",
			gemfileLockContent: gemfileLockContent,
			gemName:            "fastlane",
			wantGemVersion: Version{
				Version: "2.13.0",
				Found:   true,
			},
		},
		{
			name:               "Bad case which can happen if other gem depends on fastlane, not an issue as the version should only be used for logging.",
			gemfileLockContent: badFastlaneVersion,
			gemName:            "fastlane",
			wantGemVersion: Version{
				Version: ">= 2.0",
				Found:   true,
			},
		},
		{
			name:               "Cocoapods is not a dependency",
			gemfileLockContent: noCocoapods,
			gemName:            "cocoapods",
			wantGemVersion: Version{
				Found: false,
			},
		},
		{
			name:               "Cocoapods is a dependency",
			gemfileLockContent: hasCocoapods,
			gemName:            "cocoapods",
			wantGemVersion: Version{
				Version: "1.0.0",
				Found:   true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGemVersion, err := ParseVersionFromBundle(tt.gemName, tt.gemfileLockContent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersionFromBundle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotGemVersion, tt.wantGemVersion) {
				t.Errorf("ParseVersionFromBundle() = %v, want %v", gotGemVersion, tt.wantGemVersion)
			}
		})
	}
}

func Test_ParseBundlerVersion(t *testing.T) {
	tests := []struct {
		name               string
		gemfileLockContent string
		want               Version
		wantErr            bool
	}{
		{
			name:               "should match",
			gemfileLockContent: gemfileLockContent,
			want: Version{
				Version: "1.13.6",
				Found:   true,
			},
		},
		{
			name: "newline after version",
			gemfileLockContent: `BUNDLED WITH
      1.13.6

      `,
			want: Version{
				Version: "1.13.6",
				Found:   true,
			},
		},
		{
			name: "newline before version",
			gemfileLockContent: `BUNDLED WITH

      1.13.6`,
			want: Version{
				Version: "1.13.6",
				Found:   true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBundlerVersion(tt.gemfileLockContent)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBundlerVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseBundlerVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

const badFastlaneVersion = `GEM
  remote: https://rubygems.org/
  specs:
    CFPropertyList (3.0.0)
    addressable (2.6.0)
      public_suffix (>= 2.0.2, < 4.0)
    atomos (0.1.3)
    babosa (1.0.2)
    badge (0.8.5)
      curb (~> 0.9)
      fastimage (>= 1.6)
      fastlane (>= 2.0)
      mini_magick (>= 4.5)
    claide (1.0.2)
    colored (1.2)
    colored2 (3.1.2)
    commander-fastlane (4.4.6)
      highline (~> 1.7.2)
    curb (0.9.4)
    declarative (0.0.10)
    declarative-option (0.1.0)
    digest-crc (0.4.1)
    domain_name (0.5.20180417)
      unf (>= 0.0.5, < 1.0.0)
    dotenv (2.7.2)
    emoji_regex (1.0.1)
    excon (0.62.0)
    faraday (0.15.4)
      multipart-post (>= 1.2, < 3)
    faraday-cookie_jar (0.0.6)
      faraday (>= 0.7.4)
      http-cookie (~> 1.0.0)
    faraday_middleware (0.13.1)
      faraday (>= 0.7.4, < 1.0)
    fastimage (2.1.5)
    fastlane (2.120.0)
      CFPropertyList (>= 2.3, < 4.0.0)

PLATFORMS
  ruby

DEPENDENCIES
  badge
  fastlane

BUNDLED WITH
  1.16.1
`

const gemfileLockContent = `
GIT
  remote: git://xyz.git
  revision: xyz
  branch: patch-1
  specs:
    fastlane-xyz (1.0.2)

GEM
  remote: https://rubygems.org/
  specs:
    CFPropertyList (2.3.5)
    activesupport (4.2.7.1)
      i18n (~> 0.7)
    cocoapods (1.1.1)
      activesupport (>= 4.0.2, < 5)
    fastlane (2.13.0)
      activesupport (< 5)

PLATFORMS
  ruby

DEPENDENCIES
  cocoapods (~> 1.1.0)
  fastlane (~> 2.0)

BUNDLED WITH
   1.13.6`

const noCocoapods = `GEM
remote: https://rubygems.org/
specs:
  activesupport (4.2.6)
    i18n (~> 0.7)
  claide (1.0.0)
  fastlane (2.13.0)
    activesupport (< 5)

PLATFORMS
ruby

DEPENDENCIES
fastlane

BUNDLED WITH
 1.10.6
`

const hasCocoapods = `GEM
remote: https://rubygems.org/
specs:
  activesupport (4.2.6)
    i18n (~> 0.7)
  cocoapods (1.0.0)
    activesupport (>= 4.0.2)
  fastlane (2.13.0)
    activesupport (< 5)

PLATFORMS
ruby

DEPENDENCIES
cocoapods (~> 1.0)

BUNDLED WITH
 1.10.6
`
