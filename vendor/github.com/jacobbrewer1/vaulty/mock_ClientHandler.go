// Code generated by mockery. DO NOT EDIT.

package vaulty

import (
	api "github.com/hashicorp/vault/api"
	mock "github.com/stretchr/testify/mock"
)

// MockClientHandler is an autogenerated mock type for the ClientHandler type
type MockClientHandler struct {
	mock.Mock
}

// Client provides a mock function with no fields
func (_m *MockClientHandler) Client() *api.Client {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Client")
	}

	var r0 *api.Client
	if rf, ok := ret.Get(0).(func() *api.Client); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.Client)
		}
	}

	return r0
}

// NewMockClientHandler creates a new instance of MockClientHandler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClientHandler(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClientHandler {
	mock := &MockClientHandler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
