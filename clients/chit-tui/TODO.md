# TODO

Open work. Completed lists are in `docs/history/`; what changed is in the
CHANGELOG.

## Found running against the UAT stack

- [ ] Live reply counts need the server's `thread_updated` event, which
      `main`'s server does not send (`fix/server-review`'s does). The client
      stopped adding one per `posted` reply because, against that server,
      it double-counted. Against `main`, counts update only when a thread
      is opened, until the two branches meet.
- [ ] Tags on someone else's new post do not show until the channel
      reloads: tags are applied after the post is created, and no event
      announces them.

## Tests

- [ ] `auth` is at 88%. What is left are failures of a just-created temp
      file (Chmod, Write, Close) and of building a request; reaching them
      needs fault-injection hooks, which may not be worth adding.
