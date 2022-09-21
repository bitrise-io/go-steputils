package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bitrise-io/go-steputils/v2/cache/compression"
	"github.com/bitrise-io/go-steputils/v2/cache/keytemplate"
	"github.com/bitrise-io/go-steputils/v2/cache/network"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/docker/go-units"
)

// RestoreCacheInput is the information that comes from the cache steps that call this shared implementation
type RestoreCacheInput struct {
	// StepId identifies the exact cache step. Used for logging events.
	StepId  string
	Verbose bool
	Keys    []string
}

// Restorer ...
type Restorer interface {
	Restore(input RestoreCacheInput) error
}

type restoreCacheConfig struct {
	Verbose        bool
	Keys           []string
	APIBaseURL     stepconf.Secret
	APIAccessToken stepconf.Secret
}

type restorer struct {
	envRepo env.Repository
	logger  log.Logger
}

// NewRestorer ...
func NewRestorer(envRepo env.Repository, logger log.Logger) *restorer {
	return &restorer{envRepo: envRepo, logger: logger}
}

// Restore ...
func (r *restorer) Restore(input RestoreCacheInput) error {
	config, err := r.createConfig(input)
	if err != nil {
		return fmt.Errorf("failed to parse inputs: %w", err)
	}

	tracker := newStepTracker(input.StepId, r.envRepo, r.logger)

	r.logger.Println()
	r.logger.Infof("Downloading archive...")
	downloadStartTime := time.Now()
	archivePath, err := r.download(config)
	if err != nil {
		if errors.Is(err, network.ErrCacheNotFound) {
			r.logger.Donef("No cache entry found for the provided key")
			return nil
		}
		return fmt.Errorf("download failed: %w", err)
	}
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		return err
	}
	r.logger.Printf("Archive size: %s", units.HumanSizeWithPrecision(float64(fileInfo.Size()), 3))
	downloadTime := time.Since(downloadStartTime).Round(time.Second)
	r.logger.Donef("Downloaded archive in %s", downloadTime)
	tracker.logArchiveDownloaded(downloadTime, fileInfo, len(config.Keys))

	r.logger.Println()
	r.logger.Infof("Restoring archive...")
	extractionStartTime := time.Now()
	if err := compression.Decompress(archivePath, r.logger, r.envRepo); err != nil {
		return fmt.Errorf("failed to decompress cache archive: %w", err)
	}
	extractionTime := time.Since(extractionStartTime).Round(time.Second)
	r.logger.Donef("Restored archive in %s", extractionTime)
	tracker.logArchiveExtracted(extractionTime, len(config.Keys))
	tracker.wait()

	return nil
}

func (r *restorer) createConfig(input RestoreCacheInput) (restoreCacheConfig, error) {
	apiBaseURL := r.envRepo.Get("BITRISEIO_ABCS_API_URL")
	if apiBaseURL == "" {
		return restoreCacheConfig{}, fmt.Errorf("the secret 'BITRISEIO_ABCS_API_URL' is not defined")
	}
	apiAccessToken := r.envRepo.Get("BITRISEIO_ABCS_ACCESS_TOKEN")
	if apiAccessToken == "" {
		return restoreCacheConfig{}, fmt.Errorf("the secret 'BITRISEIO_ABCS_ACCESS_TOKEN' is not defined")
	}

	keys, err := r.evaluateKeys(input.Keys)
	if err != nil {
		return restoreCacheConfig{}, fmt.Errorf("failed to evaluate keys: %w", err)
	}

	return restoreCacheConfig{
		Verbose:        input.Verbose,
		Keys:           keys,
		APIBaseURL:     stepconf.Secret(apiBaseURL),
		APIAccessToken: stepconf.Secret(apiAccessToken),
	}, nil
}

func (r *restorer) evaluateKeys(keys []string) ([]string, error) {
	model := keytemplate.NewModel(r.envRepo, r.logger)
	buildContext := keytemplate.BuildContext{
		Workflow:   r.envRepo.Get("BITRISE_TRIGGERED_WORKFLOW_ID"),
		Branch:     r.envRepo.Get("BITRISE_GIT_BRANCH"),
		CommitHash: r.envRepo.Get("BITRISE_GIT_COMMIT"),
	}

	var evaluatedKeys []string
	for _, key := range keys {
		if key == "" {
			continue
		}

		r.logger.Println()
		r.logger.Printf("Evaluating key template: %s", key)
		evaluatedKey, err := model.Evaluate(key, buildContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate key template: %s", err)
		}
		r.logger.Donef("Cache key: %s", evaluatedKey)
		evaluatedKeys = append(evaluatedKeys, evaluatedKey)
	}

	return evaluatedKeys, nil
}

func (r *restorer) download(config restoreCacheConfig) (string, error) {
	dir, err := os.MkdirTemp("", "restore-cache")
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("cache-%s.tzst", time.Now().UTC().Format("20060102-150405"))
	downloadPath := filepath.Join(dir, name)

	params := network.DownloadParams{
		APIBaseURL:   string(config.APIBaseURL),
		Token:        string(config.APIAccessToken),
		CacheKeys:    config.Keys,
		DownloadPath: downloadPath,
	}
	err = network.Download(params, r.logger)
	if err != nil {
		return "", err
	}

	r.logger.Debugf("Archive downloaded to %s", downloadPath)

	return downloadPath, nil
}
