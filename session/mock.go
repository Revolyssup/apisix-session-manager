package session

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"

	apisixHTTP "github.com/apache/apisix-go-plugin-runner/pkg/http"
)

//Functions which are not being used have been left empty for mock implementation

// Mock implementation of http.ResponseWriter for testing
type MockResponseWriter struct {
	writeheader mockHeader
}

// Unimplemented
func (m *MockResponseWriter) Header() http.Header {
	return nil
}

func (m *MockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *MockResponseWriter) WriteHeader(statusCode int) {

}
func (m *MockResponseWriter) ReadBody() ([]byte, error) {
	return nil, nil
}

//Mock Implementation APISIX HTTP Request for testing

type MockRequest struct {
	readheader mockHeader
}

func (m *MockRequest) ID() uint32 {
	return 0
}

func (m *MockRequest) SrcIP() net.IP {
	return nil
}

func (m *MockRequest) Method() string {
	return ""
}

func (m *MockRequest) Path() []byte {
	return nil
}
func (m *MockRequest) SetPath([]byte) {

}

func (m *MockRequest) Header() apisixHTTP.Header {
	return &m.readheader
}

func (m *MockRequest) Var(name string) ([]byte, error) {
	return nil, nil
}

func (m *MockRequest) WriteHeader(statusCode int) {

}
func (m *MockRequest) Args() url.Values {
	return nil
}
func (m *MockRequest) Body() ([]byte, error) {
	return nil, nil
}
func (m *MockRequest) Context() context.Context {
	return nil
}

func (m *MockRequest) RespHeader() http.Header {
	return nil
}

// Mock Implementation APISIX HTTP Response for testing
type MockAPISIXResponseWriter struct {
	header mockHeader
	resid  uint32
}

func (m *MockAPISIXResponseWriter) ID() uint32 {
	return m.resid
}
func (m *MockAPISIXResponseWriter) StatusCode() int {
	return 0
}
func (m *MockAPISIXResponseWriter) Var(name string) ([]byte, error) {
	return nil, nil
}
func (m *MockAPISIXResponseWriter) Header() apisixHTTP.Header {
	return &m.header
}
func (m *MockAPISIXResponseWriter) ReadBody() ([]byte, error) {
	return nil, nil
}

func (m *MockAPISIXResponseWriter) Write(b []byte) (int, error) {
	return 0, nil
}
func (m *MockAPISIXResponseWriter) WriteHeader(statusCode int) {
	return
}

type mockHeader struct {
	header map[string]string
	mx     sync.RWMutex
}

func (mh *mockHeader) Set(key, value string) {
	mh.mx.Lock()
	defer mh.mx.Unlock()
	if mh.header == nil {
		mh.header = make(map[string]string)
	}
	mh.header[key] = value
}
func (mh *mockHeader) Del(key string) {
	mh.mx.Lock()
	defer mh.mx.Unlock()
	if mh.header == nil {
		mh.header = make(map[string]string)
	}
	delete(mh.header, key)
}

func (mh *mockHeader) Get(key string) string {
	mh.mx.Lock()
	defer mh.mx.Unlock()
	if mh.header == nil {
		mh.header = make(map[string]string)
	}
	return mh.header[key]
}

func (mh *mockHeader) View() http.Header {
	return nil
}
