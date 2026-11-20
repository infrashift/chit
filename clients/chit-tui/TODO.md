# TODO

Open work. The lists of completed work are in `docs/history/`. The
CHANGELOG records what changed.

## Tests

- [ ] `auth` has 88% coverage. The tests do not reach two types of failure.
      The first is a failure of a temp file that the code created a moment
      before (Chmod, Write, Close). The second is a failure to build a
      request. To reach them, the
      tests need hooks that inject faults. These hooks may not be worth the
      work.
