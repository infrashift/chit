# TODO

Open work. Completed lists are in `docs/history/`; what changed is in the
CHANGELOG.

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
