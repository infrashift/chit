# TODO

Open work. Completed lists are in `docs/history/`; what changed is in the
CHANGELOG.

## Smaller behaviour

- [ ] Pinning is shown only when the WebSocket echo arrives; with the socket
      down, a second `p` pins again instead of unpinning.
- [ ] Sending with no channel open, or before the user has loaded, drops the
      message without saying so.
- [ ] Choosing a search hit in another channel opens the channel but does not
      scroll to the hit.
- [ ] The wheel over the thread pane can page the hidden channel history.
- [ ] Changing the theme does not restyle the sign-in screen.
- [ ] The channel creator's slug keeps non-ASCII letters the name check then
      rejects, and truncates by byte.
- [ ] Kratos field-level errors are not parsed; the raw flow JSON can show in
      the sign-in box.
- [ ] If Kratos cannot be reached at startup, a valid stored session is
      discarded and the user is sent to sign in. A network error should not
      count as an invalid session.
- [ ] `lastWSSeq` deduplication never does anything (the server sends no
      sequence) and is never reset; remove it or make it work.

## Structure

- [ ] Cancel in-flight requests on sign-out: commands use
      `context.Background()`. Requests now time out after 30s and late
      responses are dropped, so this is tidiness rather than a bug.
- [ ] The session file lives in `~/.config/chit-tui/`, the config and themes
      in `~/.config/chit/`. Moving the session needs a migration so nobody is
      signed out.
- [ ] `update.go` is still about 800 lines; the channel, thread and tag cases
      could move beside their helpers.
- [ ] `overlays()` builds seven closures on every call.

## Tests

- [ ] Coverage under the 90% target: `tui` 87%, `thread` 77%, `viewport` 82%,
      `ws` 85%, `input` 84%, `auth` 84%.
