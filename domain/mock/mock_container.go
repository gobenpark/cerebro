// Code generated by MockGen. DO NOT EDIT.
// Source: ./container.go

// Package mock_domain is a generated GoMock package.
package mock_domain

import (
	domain "github.com/gobenpark/trader/domain"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
	time "time"
)

// MockContainer is a mock of Container interface
type MockContainer struct {
	ctrl     *gomock.Controller
	recorder *MockContainerMockRecorder
}

// MockContainerMockRecorder is the mock recorder for MockContainer
type MockContainerMockRecorder struct {
	mock *MockContainer
}

// NewMockContainer creates a new mock instance
func NewMockContainer(ctrl *gomock.Controller) *MockContainer {
	mock := &MockContainer{ctrl: ctrl}
	mock.recorder = &MockContainerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockContainer) EXPECT() *MockContainerMockRecorder {
	return m.recorder
}

// Empty mocks base method
func (m *MockContainer) Empty() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Empty")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Empty indicates an expected call of Empty
func (mr *MockContainerMockRecorder) Empty() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Empty", reflect.TypeOf((*MockContainer)(nil).Empty))
}

// Size mocks base method
func (m *MockContainer) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size
func (mr *MockContainerMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockContainer)(nil).Size))
}

// Clear mocks base method
func (m *MockContainer) Clear() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Clear")
}

// Clear indicates an expected call of Clear
func (mr *MockContainerMockRecorder) Clear() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clear", reflect.TypeOf((*MockContainer)(nil).Clear))
}

// Values mocks base method
func (m *MockContainer) Values() []domain.Candle {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Values")
	ret0, _ := ret[0].([]domain.Candle)
	return ret0
}

// Values indicates an expected call of Values
func (mr *MockContainerMockRecorder) Values() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Values", reflect.TypeOf((*MockContainer)(nil).Values))
}

// Add mocks base method
func (m *MockContainer) Add(candle domain.Candle) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Add", candle)
}

// Add indicates an expected call of Add
func (mr *MockContainerMockRecorder) Add(candle interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockContainer)(nil).Add), candle)
}

// Code mocks base method
func (m *MockContainer) Code() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Code")
	ret0, _ := ret[0].(string)
	return ret0
}

// Code indicates an expected call of Code
func (mr *MockContainerMockRecorder) Code() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Code", reflect.TypeOf((*MockContainer)(nil).Code))
}

// Level mocks base method
func (m *MockContainer) Level() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Level")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// Level indicates an expected call of Level
func (mr *MockContainerMockRecorder) Level() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Level", reflect.TypeOf((*MockContainer)(nil).Level))
}