package analytics

import (
	"fmt"

	"github.com/bitrise-io/go-utils/v2/analytics"
	"github.com/bitrise-io/go-utils/v2/env"
)

type TrackerFactory func(...analytics.Properties) analytics.Tracker

const (
	StepExecutionIDEnvKey = "BITRISE_STEP_EXECUTION_ID"
	StepExecutionID       = "step_execution_id"
)

func NewStepTracker(repository env.Repository, trackerFactory TrackerFactory) (analytics.Tracker, error) {
	stepExecutionID := repository.Get(StepExecutionIDEnvKey)
	if stepExecutionID == "" {
		return nil, fmt.Errorf("no step execution ID found")
	}
	return trackerFactory(analytics.Properties{StepExecutionID: stepExecutionID}), nil
}

func NewDefaultStepTracker(repository env.Repository) (analytics.Tracker, error) {
	return NewStepTracker(repository, analytics.NewDefaultTracker)
}
