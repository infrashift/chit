// Package auth signs in through Kratos and keeps the session: TokenStore
// holds the token for the running client, and SessionStore persists it,
// readable by the owner alone, so a restart stays signed in.
package auth
