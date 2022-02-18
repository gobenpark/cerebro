// Code generated by MockGen. DO NOT EDIT.
// Source: ./event.go

// Package mock_event is a generated GoMock package.
package mock_event

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockListener is a mock of Listener interface.
type MockListener struct {
	ctrl     *gomock.Controller
	recorder *MockListenerMockRecorder
}

// MockListenerMockRecorder is the mock recorder for MockListener.
type MockListenerMockRecorder struct {
	mock *MockListener
}

// NewMockListener creates a new mock instance.
func NewMockListener(ctrl *gomock.Controller) *MockListener {
	mock := &MockListener{ctrl: ctrl}
	mock.recorder = &MockListenerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockListener) EXPECT() *MockListenerMockRecorder {
	return m.recorder
}

// Listen mocks base method.
func (m *MockListener) Listen(e interface{}) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Listen", e)
}

// Listen indicates an expected call of Listen.
func (mr *MockListenerMockRecorder) Listen(e interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Listen", reflect.TypeOf((*MockListener)(nil).Listen), e)
}

// MockBroadcaster is a mock of Broadcaster interface.
type MockBroadcaster struct {
	ctrl     *gomock.Controller
	recorder *MockBroadcasterMockRecorder
}

// MockBroadcasterMockRecorder is the mock recorder for MockBroadcaster.
type MockBroadcasterMockRecorder struct {
	mock *MockBroadcaster
}

// NewMockBroadcaster creates a new mock instance.
func NewMockBroadcaster(ctrl *gomock.Controller) *MockBroadcaster {
	mock := &MockBroadcaster{ctrl: ctrl}
	mock.recorder = &MockBroadcasterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBroadcaster) EXPECT() *MockBroadcasterMockRecorder {
	return m.recorder
}

// BroadCast mocks base method.
func (m *MockBroadcaster) BroadCast(e interface{}) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "BroadCast", e)
}

// BroadCast indicates an expected call of BroadCast.
func (mr *MockBroadcasterMockRecorder) BroadCast(e interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BroadCast", reflect.TypeOf((*MockBroadcaster)(nil).BroadCast), e)
}
