// Code generated by mockery v2.53.4. DO NOT EDIT.

package mocks

import (
	context "context"

	gorm "gorm.io/gorm"

	mock "github.com/stretchr/testify/mock"

	model "go_4_vocab_keep/internal/model"

	uuid "github.com/google/uuid"
)

// TenantRepository is an autogenerated mock type for the TenantRepository type
type TenantRepository struct {
	mock.Mock
}

// Create provides a mock function with given fields: ctx, db, tenant
func (_m *TenantRepository) Create(ctx context.Context, db *gorm.DB, tenant *model.Tenant) error {
	ret := _m.Called(ctx, db, tenant)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, *model.Tenant) error); ok {
		r0 = rf(ctx, db, tenant)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Delete provides a mock function with given fields: ctx, db, tenantID
func (_m *TenantRepository) Delete(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) error {
	ret := _m.Called(ctx, db, tenantID)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, uuid.UUID) error); ok {
		r0 = rf(ctx, db, tenantID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FindByEmail provides a mock function with given fields: ctx, db, email
func (_m *TenantRepository) FindByEmail(ctx context.Context, db *gorm.DB, email string) (*model.Tenant, error) {
	ret := _m.Called(ctx, db, email)

	if len(ret) == 0 {
		panic("no return value specified for FindByEmail")
	}

	var r0 *model.Tenant
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, string) (*model.Tenant, error)); ok {
		return rf(ctx, db, email)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, string) *model.Tenant); ok {
		r0 = rf(ctx, db, email)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Tenant)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *gorm.DB, string) error); ok {
		r1 = rf(ctx, db, email)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindByID provides a mock function with given fields: ctx, db, tenantID
func (_m *TenantRepository) FindByID(ctx context.Context, db *gorm.DB, tenantID uuid.UUID) (*model.Tenant, error) {
	ret := _m.Called(ctx, db, tenantID)

	if len(ret) == 0 {
		panic("no return value specified for FindByID")
	}

	var r0 *model.Tenant
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, uuid.UUID) (*model.Tenant, error)); ok {
		return rf(ctx, db, tenantID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, uuid.UUID) *model.Tenant); ok {
		r0 = rf(ctx, db, tenantID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Tenant)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *gorm.DB, uuid.UUID) error); ok {
		r1 = rf(ctx, db, tenantID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// FindByName provides a mock function with given fields: ctx, db, name
func (_m *TenantRepository) FindByName(ctx context.Context, db *gorm.DB, name string) (*model.Tenant, error) {
	ret := _m.Called(ctx, db, name)

	if len(ret) == 0 {
		panic("no return value specified for FindByName")
	}

	var r0 *model.Tenant
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, string) (*model.Tenant, error)); ok {
		return rf(ctx, db, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *gorm.DB, string) *model.Tenant); ok {
		r0 = rf(ctx, db, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Tenant)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *gorm.DB, string) error); ok {
		r1 = rf(ctx, db, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewTenantRepository creates a new instance of TenantRepository. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTenantRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *TenantRepository {
	mock := &TenantRepository{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
