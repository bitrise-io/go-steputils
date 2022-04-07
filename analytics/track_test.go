package analytics

import (
	"testing"

	"github.com/bitrise-io/go-steputils/v2/analytics/mocks"
	"github.com/bitrise-io/go-utils/v2/analytics"
)

func TestNewStepTrackerFailsIfStepExecutionIDIsNotFound(t *testing.T) {
	repository := new(mocks.Repository)
	repository.On("Get", "BITRISE_STEP_EXECUTION_ID").Return("")
	_, err := NewDefaultStepTracker(repository)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestNewStepTrackerAddsStepExecutionIDToNewTracker(t *testing.T) {
	repository := new(mocks.Repository)
	repository.On("Get", "BITRISE_STEP_EXECUTION_ID").Return("123")
	factory := new(mocks.TrackerFactory)
	factory.On("Execute", analytics.Properties{"step_execution_id": "123"}).Return(nil, nil)
	_, err := NewStepTracker(repository, factory.Execute)
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}
	factory.AssertExpectations(t)
}
