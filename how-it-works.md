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

## Why Peer Discovery Without Signalling Server is Impossible?

The core issue preventing two devices from connecting to each other on the internet is 
**Network Address Translator (NAT)** 

**NAT**: A device commonly found inside a router, it **rewrites** the source IP and Port values of an outgoing
request from the local network and maintains a _Translation Table_ for incoming values to redirect to 
the correct devices.

![](https://upload.wikimedia.org/wikipedia/commons/thumb/c/c7/NAT_Concept-en.svg/1280px-NAT_Concept-en.svg.png)

Most devices do not have a direct, public IP address, hence they sit behind a router which is assigned a private local IP address
`192.168.1.5`. The router holds the single IP address for the whole network and all the devices behind it.

If device A wants to connect to device B (assuming they are in completely separate networks), device A cannot simply send a
packet to `192.168.1.5` since many devices share this local IP address worldwide. The solution is to send a packet to the 
device B's router's public IP.

The two problems in sending packets to a router (using its public IP):
1. **Lack of Information**: Device A has no clue about what is the public IP of the router of Device B.
2. **Firewall Protection**: Even if Device A knows the public IP of the router of the Device B and manages to send
packets to it. Those packets will be dropped by the firewall, since routers are designed to drop unsolicited packets,
unless device B explicitly initiated an outbound connection to Device A.

These problems can be overcome using the method of **UDP Hole Punching** which requires a **Signalling Server**.

It is a commonly accessible middleground for both clients to communicate, since the public IP is known.
When both the devices are connected to the server the NAT for both clients create outbound rules in their
translation table allowing the server to send responses back without getting blocked by the firewall.

The firewall communicates the Source IP and Source Port of Device A (NAT of Device A) to the Device B and vice-versa.

Now they can use **UDP Hole Punching** to trick the NAT into accepting traffic from each other, completely bypassing
the signalling server for the actual data transfer.

### UDP Hole Punching

Routers which consists of NAT and stateful firewalls block incoming traffic by default if unsolicited.
UDP hole punching begins by device A sending a packet to device B when the packet leaves the NAT A, it creates
an outbound entry, thereby allowing packets from device B's public IP, if the device B sends packets at this moment
then those packets will be accepted by device A's router and since the device B has sent packets from its NAT B
it has created a similar outgoing entry which now allows packets from device A. Now we have a connection setup.

### STUN (Session Traversal Utilities for NAT)

In `client.go`, you define STUN servers in `getWebRTCConfig()` using Google provided free STUN servers (`stun:stun.1.google.com:19302`).
When a `PeerConnection` is created, it sends a UDP packet to this Google server. The Google server replies with your public IP and port
like `204.156.213.14:44921`.

### ICE (Interactive Connectivity Establishment)

STUN only shows your public IP. ICE gathers all the possible paths to reach your machine.
Each path is called a _ICE Candidate_.
