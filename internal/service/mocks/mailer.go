// Code generated by mockery v2.53.4. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// Mailer is an autogenerated mock type for the Mailer type
type Mailer struct {
	mock.Mock
}

// Send provides a mock function with given fields: ctx, to, subject, body
func (_m *Mailer) Send(ctx context.Context, to string, subject string, body string) error {
	ret := _m.Called(ctx, to, subject, body)

	if len(ret) == 0 {
		panic("no return value specified for Send")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) error); ok {
		r0 = rf(ctx, to, subject, body)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMailer creates a new instance of Mailer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMailer(t interface {
	mock.TestingT
	Cleanup(func())
}) *Mailer {
	mock := &Mailer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
