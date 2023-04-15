package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	apisixHTTP "github.com/apache/apisix-go-plugin-runner/pkg/http"
	"github.com/google/uuid"
)

const pluginName = "session_manager"

// type CookieState string

// const ()

type Instance struct {
	sessions        map[string]*session //key is a stringified UUID of the session
	requestSessions map[uint32]*session
	sessMx          sync.RWMutex
	reqSessMx       sync.RWMutex
	cleanupQueue    chan string // This channel recieves sessions IDs to delete triggered by something like a timeout of the session. Never close this channel
}

type Config struct {
	SessionTimeoutInSeconds int    `json:"sessionTimeoutInSeconds"`
	CookieName              string `json:"cookie"`
}

// Each session represents a client-server sessions and stores information about the client for subsequent requests
type session struct {
	reqID     uint32 //Initial req ID from which the session was created
	sessionID string
	isSticky  bool
}

func New() *Instance {
	i := &Instance{
		requestSessions: make(map[uint32]*session),
		sessions:        make(map[string]*session),
		cleanupQueue:    make(chan string),
	}
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
	fmt.Println("Cleaned up session: ", sid)
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
	fmt.Println("NO COOKIE FOUND")
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

// RequestFilter is responsible for creating sessions if it doesn't already exist
func (i *Instance) RequestFilter(cfg interface{}, w http.ResponseWriter, r apisixHTTP.Request) {
	config := cfg.(Config)
	cookies := r.Header().Get("Cookie")
	sid, ok := getKeyFromCookies(config.CookieName, cookies)
	if !ok || i.getSession(sid) == nil { //If no cookie is found or there exists an expired session then create a new Session
		sid := uuid.New().String()
		i.createSession(r.ID(), sid, &session{
			reqID:     r.ID(),
			sessionID: sid,
		})
		r.Header().Set("Cookie", fmt.Sprintf("%s=%s", config.CookieName, sid)) //This is useful for sticky sessions. When the sid key that is passed to this plugin is used for chash loadbalancing in upstream

		//It may be the case that the session was created here but before the response could come back, the session was deleted. It will look like a session was never created, since the ResponseFilter wont find any session.
		//Usually it is assumed that the Latency<SessionTimeout value
		go func(sid string) {
			<-time.After(time.Second * time.Duration(config.SessionTimeoutInSeconds))
			i.cleanupQueue <- sid
		}(sid)
	}
}

// ResponseFilter handles things like:
// 1. Sticky sessions with cookies (Requires chash type loadbalancing on upstreams)
func (i *Instance) ResponseFilter(cfg interface{}, w apisixHTTP.Response) {
	config := cfg.(Config)
	//For some reason the requestID detected in RequestFilter is autoincremented by 1 when detected on response.
	//IMPORTANT: It is assumed that this request ID is unique for across requests
	reqID := w.ID() - 1
	if i.requestSessions[reqID] != nil { //Attach the proper cookies on response for existing session
		w.Header().Set("Set-Cookie", fmt.Sprintf("%s=%s", config.CookieName, i.requestSessions[reqID].sessionID))
	}
}
