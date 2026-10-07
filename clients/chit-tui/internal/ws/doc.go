// Package ws is the WebSocket client for real-time events. Each Connect
// starts a session that reads until it fails and then redials with capped,
// jittered backoff; pings detect a half-open link. Events and connection
// state arrive on separate channels that outlive a session, so listeners
// started once serve every sign-in.
package ws
