# Understanding

## Establishing WebSocket Connection to the Signalling Server
We need a signaling server for peer discovery. This server uses websockets using the `gorilla/websocket`
implementation which upgrades the HTTP GET request into a WebSocket request.

WebSockets are required for realtime sending of data to all connected clients, data like ICE candidates and
SDP offer and answer messages. If you used some other protocol like HTTP, you would have to constantly poll
the server for new data (ICE candidates, SDP offers and answer messages) which is inefficient and introduces 
latency.

The upgrade process doesn't create a new HTTP request. HTTP uses TCP under the hood and hence there exists a TCP
socket for each HTTP connection. When upgrading a request that socket is hijacked for a new purpose (WebSocket).
Now the new purpose of that socket is to server WebSocket frames.

The process begins by the client sending a standard HTTP/1.1 GET request to the signaling server with following
data:
```json
{
    Connection: "Upgrade",
    Upgrade: "websocket",
    Sec-WebSocket-Version: "13", // specifying the WebSocket protocol version
    Sec-WebSocket-Key: "A randomly generated, Base64-encoded string",
}
```

Inside the `HandleConnections` method, gorilla confirms the presence of the `Connection` and `Upgrade` headers
in the client's request.

To prove to the client (that the server understands the WebSocket protocol), a globally known "magic string" 
defined in the [RFC 6455 (The WebSocket Protocol)](https://datatracker.ietf.org/doc/html/rfc6455) `Sec-WebSocket-Key` 
is appended to the client's `Sec-WebSocket-Key` header value, hashes the combined result using SHA-1 and then 
encodes using Base64 to return to client for verification.

The server sends this final result in a HTTP response with the following data:
```json
{
    Connection: "Upgrade",
    Upgrade: "websocket",
    Sec-WebSocket-Key: "calculated value from Sec-WebSocket-Key", // base64( SHA-1(Sec-WebSocket-Key + magic-string) )
}
```

As this response is sent to the server, the `gorilla/websocket` library uses the `http.Hijacker` interface to hijack the
TCP socket which frees Go of the responsibility of managing this connection, essentially passing full control to the library.
Hijacking is performed so the returned object of type `websocket.Conn` can be used for sending and receiving WebSocket frames.

