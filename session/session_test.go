package session

import (
	"fmt"
	"testing"

	"github.com/apache/apisix-go-plugin-runner/pkg/runner"
)

func TestRequestFilter(t *testing.T) {
	type testCase struct {
		name            string
		description     string
		cfg             Config
		sessionState    map[string]*session //To be used for mocking a session state before Filter handling
		reqSessionState map[uint32]*session //To be used for mocking a session state before Filter handling
		req             *MockRequest
		res             *MockResponseWriter
		check           func(req *MockRequest, res *MockResponseWriter) error
	}
	testCases := []testCase{
		{
			name:        "TestSessionSetting",
			description: "Very basic capability of Filter function to set a new session in cookie for a new request",
			cfg: Config{
				CookieName: "test-id",
			},
			req: &MockRequest{
				readheader: mockHeader{header: make(map[string]string)},
			},
			res: &MockResponseWriter{
				writeheader: mockHeader{header: make(map[string]string)},
			},
			check: func(req *MockRequest, res *MockResponseWriter) error {
				key, ok := getKeyFromCookies("test-id", req.Header().Get("Cookie"))
				if ok && key == "" {
					return fmt.Errorf("empty key set")
				}
				if !ok {
					return fmt.Errorf("no cookie found for test-id")
				}
				return nil
			},
		},
	}

	for _, tt := range testCases {
		i := New(runner.RunnerConfig{}) // A new instance of plugin
		i.RequestFilter(tt.cfg, tt.res, tt.req)
		err := tt.check(tt.req, tt.res)
		if err != nil {
			t.Fatal(fmt.Printf("Name: %s\nDescription:%s\nReason:%s\n", tt.name, tt.description, err.Error()))
		}
	}
}

func TestResponseFilter(t *testing.T) {
	type testCase struct {
		name            string
		description     string
		sessionState    map[string]*session //To be used for mocking a session state before Filter handling
		reqSessionState map[uint32]*session //To be used for mocking a session state before Filter handling
		cfg             Config
		res             *MockAPISIXResponseWriter
		check           func(res *MockAPISIXResponseWriter) error
	}
	testCases := []testCase{
		{
			name:        "TestSessionSetting",
			description: "Very basic capability of Filter function to set a new session in cookie for returned responses",
			cfg: Config{
				CookieName: "test-id",
			},
			res: &MockAPISIXResponseWriter{
				header: mockHeader{header: map[string]string{
					"Cookie": "test-id=xyz",
				}},
				resid: 124, //As you can see the response ID is 1+reqID
			},
			sessionState: map[string]*session{ //Emulating session creation of Request Filter
				"xyz": {},
			},
			reqSessionState: map[uint32]*session{
				123: {
					sessionID: "xyz",
				},
			},
			check: func(res *MockAPISIXResponseWriter) error {
				cookies := res.Header().Get("Set-Cookie")
				if len(cookies) == 0 {
					return fmt.Errorf("no instruction to set cookie")
				}
				key, ok := getKeyFromCookies("test-id", cookies)
				if ok && key == "" {
					return fmt.Errorf("empty key set")
				}
				if !ok {
					return fmt.Errorf("no cookie found for test-id")
				}
				return nil
			},
		},
	}

	for _, tt := range testCases {
		i := New(runner.RunnerConfig{}) // A new instance of plugin
		i.sessions = tt.sessionState
		i.requestSessions = tt.reqSessionState
		i.ResponseFilter(tt.cfg, tt.res)
		err := tt.check(tt.res)
		if err != nil {
			t.Fatal(fmt.Printf("Name: %s\nDescription:%s\nReason:%s\n", tt.name, tt.description, err.Error()))
		}
	}
}
