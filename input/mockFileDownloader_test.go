package input

import (
	"github.com/stretchr/testify/mock"
)

// MockFileDownloader ...
type MockFileDownloader struct {
	mock.Mock
}

// Get ...
func (m *MockFileDownloader) Get(destination, source string) error {
	args := m.Called(destination, source)
	return args.Error(0)
}

// GivenGetFails ...
func (m *MockFileDownloader) GivenGetFails(reason error) *MockFileDownloader {
	m.On("Get", mock.Anything, mock.Anything).Return(reason)
	return m
}

// GivenGetSucceed ...
func (m *MockFileDownloader) GivenGetSucceed() *MockFileDownloader {
	m.On("Get", mock.Anything, mock.Anything).Return(nil)
	return m
}
