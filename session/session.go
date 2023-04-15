package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	apisixHTTP "github.com/apache/apisix-go-plugin-runner/pkg/http"
	"github.com/apache/apisix-go-plugin-runner/pkg/runner"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const pluginName = "session_manager"

type Instance struct {
	sessions        map[string]*session //key is a stringified UUID of the session
	requestSessions map[uint32]*session
	sessMx          sync.RWMutex
	reqSessMx       sync.RWMutex
	log             *zap.SugaredLogger
	cleanupQueue    chan string // This channel recieves sessions IDs to delete triggered by something like a timeout of the session. Never close this channel
}

type Config struct {
	SessionTimeoutInSeconds int    `json:"sessionTimeoutInSeconds"`
	CookieName              string `json:"cookie"`
	KeyAuthEnabled          bool   `json:"keyAuthEnabled"` //When using it along with the key-auth plugin, the apiKey is stored in session
}

// Each session represents a client-server sessions and stores information about the client for subsequent requests
type session struct {
	reqID     uint32 //Initial req ID from which the session was created
	sessionID string
	//Caveat: When using with key-auth plugin, until the first time a valid APIKEY is passed, session wont be created because there is no point in creating a session if the "post-resp" plugin wont be executed which is responsble for writing back sessionID in cookie
	apiKeyValue string //When used with key-auth plugin. Make sure to hook session_plugin in pre-req when using alongside key-auth
	isSticky    bool
}

// Reusing apisix's plugin logger function for reusability
func newLogger(level zapcore.Level, out zapcore.WriteSyncer) *zap.SugaredLogger {
	var atomicLevel = zap.NewAtomicLevel()
	atomicLevel.SetLevel(level)

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		out,
		atomicLevel)
	lg := zap.New(core, zap.AddStacktrace(zap.ErrorLevel), zap.AddCaller(), zap.AddCallerSkip(1))
	return lg.Sugar()
}
func New(cfg runner.RunnerConfig) *Instance {
	if cfg.LogOutput == nil {
		cfg.LogOutput = os.Stdout
	}
	i := &Instance{
		requestSessions: make(map[uint32]*session),
		sessions:        make(map[string]*session),
		cleanupQueue:    make(chan string),
	}
	i.log = newLogger(cfg.LogLevel, cfg.LogOutput)
	go i.cleanup()
	return i
}

func (i *Instance) cleanup() {
	for {
		select {
		case sid := <-i.cleanupQueue:
			i.removeSession(sid)
		}
	}
}
func (i *Instance) Name() string {
	return pluginName
}
func (i *Instance) removeSession(sid string) {
	i.sessMx.Lock()
	i.sessMx.Unlock()
	i.log.Info("Cleaned up session: ", sid)
	delete(i.sessions, sid)
}
func (i *Instance) ParseConf(in []byte) (interface{}, error) {
	cfg := Config{}
	err := json.Unmarshal(in, &cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func getKeyFromCookies(key string, cookies string) (string, bool) {
	if cookies != "" {
		cookieStrings := strings.Split(cookies, "; ")
		for _, cookieString := range cookieStrings {
			if strings.HasPrefix(cookieString, fmt.Sprintf("%s=", key)) {
				return strings.TrimPrefix(cookieString, fmt.Sprintf("%s=", key)), true
			}
		}
	}
	return "", false
}

func (i *Instance) getSession(id string) *session {
	i.sessMx.RLock()
	defer i.sessMx.RUnlock()
	return i.sessions[id]
}
func (i *Instance) getSessionFromRequestID(id string) *session {
	i.reqSessMx.RLock()
	defer i.reqSessMx.RUnlock()
	return i.sessions[id]
}
func (i *Instance) createSession(reqID uint32, id string, s *session) {
	i.reqSessMx.Lock()
	i.requestSessions[reqID] = s
	i.reqSessMx.Unlock()

	i.sessMx.Lock()
	i.sessions[id] = s
	i.sessMx.Unlock()
}

const APIKEY = "apiKey"

// RequestFilter is responsible for creating sessions if it doesn't already exist
func (i *Instance) RequestFilter(cfg interface{}, w http.ResponseWriter, r apisixHTTP.Request) {
	i.log.Info("Executing Request filter for req: ", r.ID())
	config := cfg.(Config)
	cookies := r.Header().Get("Cookie")
	sid, ok := getKeyFromCookies(config.CookieName, cookies)
	sess := i.getSession(sid)
	if !ok || sess == nil { //If no session is found or there exists an expired session then create a new Session
		sid := uuid.New().String()
		sess = &session{
			reqID:     r.ID(),
			sessionID: sid,
		}
		if config.KeyAuthEnabled {
			sess.apiKeyValue = r.Header().Get(APIKEY)
			i.log.Info("SET APIKEY IN SESSION AS: ", sess.apiKeyValue, " for session", sess.sessionID)
		}
		i.createSession(r.ID(), sid, sess)
		r.Header().Set("Cookie", fmt.Sprintf("%s=%s", config.CookieName, sid)) //This is useful for sticky sessions. When the sid key that is passed to this plugin is used for chash loadbalancing in upstream

		//It may be the case that the session was created here but before the response could come back, the session was deleted. It will look like a session was never created, since the ResponseFilter wont find any session.
		//Usually it is assumed that the Latency<SessionTimeout value
		go func(sid string, timeout int) {
			if timeout <= 0 { //Timeout less than equal to 0 is considered an infinite session
				return
			}
			<-time.After(time.Second * time.Duration(timeout))
			i.cleanupQueue <- sid
		}(sid, config.SessionTimeoutInSeconds)
	}
	if config.KeyAuthEnabled && sess != nil { //When used with key-auth plugin, re-add the apiKey in header
		if r.Header().Get(APIKEY) != "" { //If another API key is sent for subsequent request then respect the new APIKEY to refresh the store
			sess.apiKeyValue = r.Header().Get(APIKEY)
		}
		r.Header().Set(APIKEY, sess.apiKeyValue)
	}
}

// ResponseFilter handles things like:
// 1. Sticky sessions with cookies (Requires chash type loadbalancing on upstreams)
func (i *Instance) ResponseFilter(cfg interface{}, w apisixHTTP.Response) {
	config := cfg.(Config)
	i.log.Info("Executing Response filter for resp: ", w.ID())
	//For some reason the requestID detected in RequestFilter is autoincremented by 1 when detected on response.
	//IMPORTANT: It is assumed that this request ID is unique for across requests
	reqID := w.ID() - 1
	if i.requestSessions[reqID] != nil { //Attach the proper cookies on response for existing session
		w.Header().Set("Set-Cookie", fmt.Sprintf("%s=%s", config.CookieName, i.requestSessions[reqID].sessionID))
	}
}
