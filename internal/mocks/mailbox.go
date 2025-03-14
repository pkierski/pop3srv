// Code generated by mockery v2.52.2. DO NOT EDIT.

package mocks

import (
	io "io"

	mock "github.com/stretchr/testify/mock"
)

// Mailbox is an autogenerated mock type for the Mailbox type
type Mailbox struct {
	mock.Mock
}

// Close provides a mock function with no fields
func (_m *Mailbox) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Dele provides a mock function with given fields: msgNumber
func (_m *Mailbox) Dele(msgNumber int) error {
	ret := _m.Called(msgNumber)

	if len(ret) == 0 {
		panic("no return value specified for Dele")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(int) error); ok {
		r0 = rf(msgNumber)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// List provides a mock function with no fields
func (_m *Mailbox) List() ([]int, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for List")
	}

	var r0 []int
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]int, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []int); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]int)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListOne provides a mock function with given fields: msgNumber
func (_m *Mailbox) ListOne(msgNumber int) (int, error) {
	ret := _m.Called(msgNumber)

	if len(ret) == 0 {
		panic("no return value specified for ListOne")
	}

	var r0 int
	var r1 error
	if rf, ok := ret.Get(0).(func(int) (int, error)); ok {
		return rf(msgNumber)
	}
	if rf, ok := ret.Get(0).(func(int) int); ok {
		r0 = rf(msgNumber)
	} else {
		r0 = ret.Get(0).(int)
	}

	if rf, ok := ret.Get(1).(func(int) error); ok {
		r1 = rf(msgNumber)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Message provides a mock function with given fields: msgNumber
func (_m *Mailbox) Message(msgNumber int) (io.ReadCloser, error) {
	ret := _m.Called(msgNumber)

	if len(ret) == 0 {
		panic("no return value specified for Message")
	}

	var r0 io.ReadCloser
	var r1 error
	if rf, ok := ret.Get(0).(func(int) (io.ReadCloser, error)); ok {
		return rf(msgNumber)
	}
	if rf, ok := ret.Get(0).(func(int) io.ReadCloser); ok {
		r0 = rf(msgNumber)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(int) error); ok {
		r1 = rf(msgNumber)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Stat provides a mock function with no fields
func (_m *Mailbox) Stat() (int, int, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Stat")
	}

	var r0 int
	var r1 int
	var r2 error
	if rf, ok := ret.Get(0).(func() (int, int, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	if rf, ok := ret.Get(1).(func() int); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Uidl provides a mock function with no fields
func (_m *Mailbox) Uidl() ([]string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Uidl")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UidlOne provides a mock function with given fields: msgNumber
func (_m *Mailbox) UidlOne(msgNumber int) (string, error) {
	ret := _m.Called(msgNumber)

	if len(ret) == 0 {
		panic("no return value specified for UidlOne")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(int) (string, error)); ok {
		return rf(msgNumber)
	}
	if rf, ok := ret.Get(0).(func(int) string); ok {
		r0 = rf(msgNumber)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(int) error); ok {
		r1 = rf(msgNumber)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMailbox creates a new instance of Mailbox. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMailbox(t interface {
	mock.TestingT
	Cleanup(func())
}) *Mailbox {
	mock := &Mailbox{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
