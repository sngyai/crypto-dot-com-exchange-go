// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/cshep4/go-cryptocom/internal/auth (interfaces: SignatureGenerator)

// Package signature_mocks is a generated GoMock package.
package signature_mocks

import (
	gomock "github.com/golang/mock/gomock"
	auth "github.com/sngyai/go-cryptocom/internal/auth"
	reflect "reflect"
)

// MockSignatureGenerator is a mock of SignatureGenerator interface
type MockSignatureGenerator struct {
	ctrl     *gomock.Controller
	recorder *MockSignatureGeneratorMockRecorder
}

// MockSignatureGeneratorMockRecorder is the mock recorder for MockSignatureGenerator
type MockSignatureGeneratorMockRecorder struct {
	mock *MockSignatureGenerator
}

// NewMockSignatureGenerator creates a new mock instance
func NewMockSignatureGenerator(ctrl *gomock.Controller) *MockSignatureGenerator {
	mock := &MockSignatureGenerator{ctrl: ctrl}
	mock.recorder = &MockSignatureGeneratorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockSignatureGenerator) EXPECT() *MockSignatureGeneratorMockRecorder {
	return m.recorder
}

// GenerateSignature mocks base method
func (m *MockSignatureGenerator) GenerateSignature(arg0 auth.SignatureRequest) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GenerateSignature", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GenerateSignature indicates an expected call of GenerateSignature
func (mr *MockSignatureGeneratorMockRecorder) GenerateSignature(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GenerateSignature", reflect.TypeOf((*MockSignatureGenerator)(nil).GenerateSignature), arg0)
}
