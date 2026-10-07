# TODO

Open work. Completed lists are in `docs/history/`; what changed is in the
CHANGELOG.

## Structure

- [ ] `update.go` is still about 800 lines; the channel, thread and tag cases
      could move beside their helpers.
- [ ] `overlays()` builds seven closures on every call.

## Tests

- [ ] Coverage under the 90% target: `tui` 87%, `thread` 77%, `viewport` 82%,
      `ws` 85%, `input` 84%, `auth` 84%.
