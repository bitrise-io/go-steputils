package mocks

import "github.com/stretchr/testify/mock"

type CommandLocator struct {
	mock.Mock
}

func (_m *CommandLocator) LookPath(file string) (string, error) {
	ret := _m.Called(file)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(file)
	} else {
		r0, _ = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(file)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
