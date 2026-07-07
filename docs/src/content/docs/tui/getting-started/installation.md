---
title: Installation
description: How to install Chit TUI from source.
---

## Prerequisites

- **Go 1.24+** — [Install Go](https://go.dev/doc/install)
- **A Chit server** — A running instance of [Chit](https://github.com/infrashift/chit)
- **A session token** — Obtained by authenticating with your Chit server

## Build from Source

The TUI lives in the Chit monorepo under `clients/chit-tui`. Clone the
repository and build:

```bash
git clone https://github.com/infrashift/chit.git
cd chit/clients/chit-tui
make build
```

The binary is output to `bin/chit-tui`.

## Verify the Build

Run the full quality suite to confirm everything is working:

```bash
make all   # lint + test + build
```

## Install to PATH

Copy the binary somewhere in your `$PATH`:

```bash
cp bin/chit-tui ~/.local/bin/
```

Or use `go install` directly:

```bash
go install github.com/infrashift/chit/clients/chit-tui/cmd/chit-tui@latest
```
