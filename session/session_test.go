package session

import (
	"fmt"
	"net/http"
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
		check           func(req *MockRequest, res *MockResponseWriter, sess map[string]*session) error
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
			reqSessionState: make(map[uint32]*session),
			sessionState:    make(map[string]*session),
			check: func(req *MockRequest, res *MockResponseWriter, sess map[string]*session) error {
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
		{
			name:        "TestCustomKeyAuth",
			description: "When session is non existent and apiKey is not passed in header then reject the calls with 401 along with a newly created session",
			cfg: Config{
				CookieName:    "test-id",
				CustomKeyAuth: "auth-one",
			},
			req: &MockRequest{
				readheader: mockHeader{header: make(map[string]string)},
			},
			res: &MockResponseWriter{
				writeheader:    mockHeader{header: make(map[string]string)},
				responseHeader: make(http.Header),
			},
			sessionState: map[string]*session{
				"abc": {
					sessionID: "abc",
				},
			},
			reqSessionState: make(map[uint32]*session),
			check: func(req *MockRequest, res *MockResponseWriter, sess map[string]*session) error {
				cookies := res.Header().Get("Set-Cookie")
				if cookies == "" {
					return fmt.Errorf("no instruction to set cookie")
				}
				key, ok := getKeyFromCookies("test-id", cookies)
				if ok && key == "" {
					return fmt.Errorf("empty key set")
				}
				if !ok {
					return fmt.Errorf("no cookie found for test-id")
				}
				if res.statuscode != http.StatusUnauthorized {
					return fmt.Errorf("expected status code:%d, found %d", http.StatusUnauthorized, res.statuscode)
				}
				return nil
			},
		}, {
			name:        "TestCustomKeyAuthWithCorrectKey",
			description: "When session is non existent and apiKey is correctly passed in header then the calls should not 401 and a new session should store the passed apiKey",
			cfg: Config{
				CookieName:    "test-id",
				CustomKeyAuth: "auth-one",
			},
			req: &MockRequest{
				readheader: mockHeader{
					header: map[string]string{
						"apiKey": "auth-one",
					},
				},
			},
			res: &MockResponseWriter{
				writeheader:    mockHeader{header: make(map[string]string)},
				responseHeader: make(http.Header),
			},
			sessionState:    map[string]*session{},
			reqSessionState: make(map[uint32]*session),
			check: func(req *MockRequest, res *MockResponseWriter, sess map[string]*session) error {
				if res.statuscode == http.StatusUnauthorized {
					return fmt.Errorf("failed to authorize")
				}
				for _, s := range sess {
					if s.customKeyValue != "" {
						if s.customKeyValue != "auth-one" {
							return fmt.Errorf("wrong value stored for apiKey: %s,expected %s", s.customKeyValue, "auth-one")
						}
						return nil
					}

				}
				return fmt.Errorf("could not find a valid session")
			},
		},
		{
			name:        "TestCustomKeyAuthExistingSessionWithCorrectKey",
			description: "When session is existent and apiKey is not passed in header then the calls should not 401 if the session contains valid key",
			cfg: Config{
				CookieName:    "test-id",
				CustomKeyAuth: "auth-one",
			},
			req: &MockRequest{
				readheader: mockHeader{
					header: map[string]string{
						"Cookie": "test-id=abc", //The client's existing session is identified by the sessionID in cookie
					},
				},
			},
			res: &MockResponseWriter{
				writeheader:    mockHeader{header: make(map[string]string)},
				responseHeader: make(http.Header),
			},
			sessionState: map[string]*session{
				"abc": {
					sessionID:      "abc",
					customKeyValue: "auth-one", //Custom key is already stored inside of session
				},
			},
			reqSessionState: make(map[uint32]*session),
			check: func(req *MockRequest, res *MockResponseWriter, sess map[string]*session) error {
				if res.statuscode == http.StatusUnauthorized {
					return fmt.Errorf("failed to authorize")
				}
				return nil
			},
		},
	}

	for _, tt := range testCases {
		i := New(runner.RunnerConfig{}) // A new instance of plugin
		i.sessions = tt.sessionState
		i.requestSessions = tt.reqSessionState
		i.RequestFilter(tt.cfg, tt.res, tt.req)
		err := tt.check(tt.req, tt.res, tt.sessionState)
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
