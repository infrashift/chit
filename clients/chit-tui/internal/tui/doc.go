// Package tui is the root Bubble Tea model. It composes the panes and
// overlays, routes keys, mouse and server messages between them, and owns
// the session's state: channels, unread counts, users, the open thread.
// Update dispatches to handlers grouped by concern across the files of
// this package; session.go handles signing in and out.
package tui
