// Code generated by MockGen. DO NOT EDIT.
// Source: ./strategy.go

// Package mock_strategy is a generated GoMock package.
package mock_strategy

import (
	context "context"
	reflect "reflect"

	indicator "github.com/gobenpark/cerebro/indicator"
	order "github.com/gobenpark/cerebro/order"
	gomock "github.com/golang/mock/gomock"
)

// MockStrategy is a mock of Strategy interface.
type MockStrategy struct {
	ctrl     *gomock.Controller
	recorder *MockStrategyMockRecorder
}

// MockStrategyMockRecorder is the mock recorder for MockStrategy.
type MockStrategyMockRecorder struct {
	mock *MockStrategy
}

// NewMockStrategy creates a new mock instance.
func NewMockStrategy(ctrl *gomock.Controller) *MockStrategy {
	mock := &MockStrategy{ctrl: ctrl}
	mock.recorder = &MockStrategyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStrategy) EXPECT() *MockStrategyMockRecorder {
	return m.recorder
}

// Filter mocks base method.
func (m *MockStrategy) Filter(ctx context.Context, code string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Filter", ctx, code)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Filter indicates an expected call of Filter.
func (mr *MockStrategyMockRecorder) Filter(ctx, code interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Filter", reflect.TypeOf((*MockStrategy)(nil).Filter), ctx, code)
}

// Name mocks base method.
func (m *MockStrategy) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockStrategyMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockStrategy)(nil).Name))
}

// Next mocks base method.
func (m *MockStrategy) Next(indicator indicator.Value) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Next", indicator)
}

// Next indicates an expected call of Next.
func (mr *MockStrategyMockRecorder) Next(indicator interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockStrategy)(nil).Next), indicator)
}

// NotifyCashValue mocks base method.
func (m *MockStrategy) NotifyCashValue(before, after int64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "NotifyCashValue", before, after)
}

// NotifyCashValue indicates an expected call of NotifyCashValue.
func (mr *MockStrategyMockRecorder) NotifyCashValue(before, after interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotifyCashValue", reflect.TypeOf((*MockStrategy)(nil).NotifyCashValue), before, after)
}

// NotifyFund mocks base method.
func (m *MockStrategy) NotifyFund() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "NotifyFund")
}

// NotifyFund indicates an expected call of NotifyFund.
func (mr *MockStrategyMockRecorder) NotifyFund() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotifyFund", reflect.TypeOf((*MockStrategy)(nil).NotifyFund))
}

// NotifyOrder mocks base method.
func (m *MockStrategy) NotifyOrder(o order.Order) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "NotifyOrder", o)
}

// NotifyOrder indicates an expected call of NotifyOrder.
func (mr *MockStrategyMockRecorder) NotifyOrder(o interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotifyOrder", reflect.TypeOf((*MockStrategy)(nil).NotifyOrder), o)
}

// NotifyTrade mocks base method.
func (m *MockStrategy) NotifyTrade() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "NotifyTrade")
}

// NotifyTrade indicates an expected call of NotifyTrade.
func (mr *MockStrategyMockRecorder) NotifyTrade() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotifyTrade", reflect.TypeOf((*MockStrategy)(nil).NotifyTrade))
}
