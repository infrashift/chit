# TODO

Open work. Completed lists are in `docs/history/`; what changed is in the
CHANGELOG.

## Tests

- [ ] `auth` is at 88%. What is left are failures of a just-created temp
      file (Chmod, Write, Close) and of building a request; reaching them
      needs fault-injection hooks, which may not be worth adding.
