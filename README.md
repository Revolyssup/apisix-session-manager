![Tests](https://github.com/revolyssup/apisix-session-manager/actions/workflows/ci.yaml/badge.svg)
# Session Management in APISIX


## Introduction
Session management is an important part of many web applications, including those built on top of APISIX. 

In particular, there is currently no built-in support or plugin for session management in APISIX, which means that developers must either implement their own session management solution or use a third-party library.

## Problem Statement

As APIs become more complex, it becomes challenging to manage user authentication and session handling. There is a need for a solution that can securely store user credentials and manage sessions for better performance and user experience. Additionally, with underlying support in APISIX for chash based loadbalancing in a given upstream, it is important to handle the creation and deletion of specific cookies to facilitate sticky sessions. Moreover, sessions should have a timeout as well as a limit to a certain number of request failures after which a new session will be created.



## Existing Solution by Kong
Kong has a built in plugin support for [session management](https://github.com/Kong/kong/tree/master/kong/plugins/session) which is built using [lua-resty-session](https://github.com/bungle/lua-resty-session). Currently APISIX does not have any such plugin. Ideally a lua plugin should be created using lua-resty-session and added in-tree with the other plugins. 


## Proposed Solution 
As a working solution, APISIX’s go plugin runner is used to build a go based plugin which will be hooked to “ext-plugin-pre-req” and “ext-plugin-post-resp” filters on APISIX which allow plugin execution with RPCs on unix sockets.

The proposed solution is a session manager plugin for APISIX, written in Go. The plugin addresses the challenges mentioned above and provides the following core features:

Session backed key-auth: Each session exists across the lifecycle of multiple request responses. The plugin persists the authentication data in the session, so that users do not have to keep passing it through headers or URI.

Facilitation of sticky sessions when used along with chash loadbalancer strategy in APISIX upstream: The plugin acts as a cookie manager and associates each session with cookies. When chash type loadbalancing is enabled, clients do not have to manually set cookies and keep track.

Store, auto-expire, and creation of sessions: The plugin handles the creation and deletion of sessions, and using the timeout and failureLimit values passed in the plugin configuration, sessions are invalidated and new sessions are created accordingly. Moreover, the plugin has extensive unit tests in Go, covering both RequestFilter and ResponseFilter methods given by apisix-go-plugin-runner, ensuring its robustness and reliability.


## Caveats, Limitations and other Implementation details 

1. The request ID is auto-incremented by 1 when detected in the ResponseFilter on its way back. The intention of this behavior is unclear as of now so a hacky logic is applied that decrements the requestID by 1 on ResponseFilter to match.

2. The plugin might have issues for using it with other built in plugins due to one reason that both RequestFilter and ResponseFilter need to be executed to reliably manage a session. In cases where the built in filters block or respond to the calls themselves, the lifecycle of the request never enters ResponseFilter, therefore sessions cannot be guaranteed in such scenarios. 

3. To avoid the above issue and still provide key-auth plugin features, a custom key-auth functionality is added which works exactly like the already present key-auth.

4. Currently sessions are stored in go maps, like lua-resty-sessions multiple backends like redis or postgres can be integrated to persist the sessions much more reliably.

5. This one is not limited to this plugin but an in general limitation of sticky sessions inside APISIX. When the upstream nodes are DNS names instead of IPs, the chash loadbalancing does not work therefore sticky sessions cannot be guaranteed. Refer to this github issue (#9305) where my doubt regarding why the DNS name doesn’t work was clarified.

6. Notice the APISIX version in the docker-compose.yaml in repository because some previous versions did not have support for “ext-plugin-post-resp” which is required for this plugin to operate.

7. For consistency pass the same config in both “ext-plugin-pre-req” and “ext-plugin-post-resp”. Example configs are given in configs directory

## Demo/Screenshots
The configs passed to admin API for testing each of these features is given in .configs/


1. A session being created and persisted in the cookie with simple round-robin. Note a new cookie being generated after 10 seconds as that was the timeout for a session provided.

[![Demo]()](https://user-images.githubusercontent.com/43276904/232322685-9274a862-e07b-434a-847c-04ab8723a7b9.mp4)


![timeout](https://user-images.githubusercontent.com/43276904/232323525-4be6cdd1-f6f6-41f9-b4db-330c13887d90.png)

2. After user key-auth is passed once for a session, the authentication information is stored with the session and used throughout the session. User doesn't need to keep passing it in a header which imprpves security.
[![Demo]()](https://user-images.githubusercontent.com/43276904/232322322-b60bbed5-3122-4f6c-b7ed-6cc047081a18.mp4)



3. When used with chash type load balancing, you can see the sticky session behaviour for a given session.
[![Demo]()](https://user-images.githubusercontent.com/43276904/232322935-8a43f90e-e2ef-455f-8483-aa159690333f.mp4)


4. An example of a session being refreshed after a limit(3) failed requests.

[![Demo]()](https://user-images.githubusercontent.com/43276904/232323281-b42a5112-6008-436f-a34b-a1005ed5505e.mp4)


![limitReq](https://user-images.githubusercontent.com/43276904/232323348-04efab09-4125-4c89-91b0-e7e51817c7c6.png)






