# PRD-phase1.md: Project Chit-TUI

**Status:** Draft

**Project Owner:** Ryan Craig

**Project Origin:** github.com/infrashift/chit-tui

**Core Stack:** Go, Bubbletea, Lipgloss, Bubbles, Glamour

## 1. Executive Summary

**Chit-TUI** is the primary terminal interface for the Chit messaging server; github.com/infrashift/chit. It aims to provide a high-velocity, keyboard-centric experience for sovereign communication. It leverages the Charmbracelet "Bubble" ecosystem to deliver a polished, responsive UI that integrates with Ory for identity and ZincSearch for discovery.

## 2. Functional Requirements

* **Persistent Connectivity:** Maintain a WebSocket connection for real-time events (new posts, typing indicators, channel updates).
* **Dual-Pane Navigation:** A sidebar for Team/Channel switching and a main viewport for message history.
* **Identity Integration:** Transparently handle Kratos sessions via Oathkeeper headers.
* **Threaded Conversations:** Support viewing and replying to specific message threads.
* **Markdown Rendering:** Rich text rendering in the terminal for message content.
* **Search Interface:** Dedicated modal or view for full-text search results via ZincSearch.

## 3. References for Claude Code

* **Chit Server:** `github.com/infrashift/chit` (Core API and models)
* **RocketChat-TUI:** `github.com/RocketChat/rocketchat-tui` (Architecture inspiration)
* **Bubbletea:** `github.com/charmbracelet/bubbletea` (Runtime)
* **Bubbles:** `github.com/charmbracelet/bubbles` (UI components)
* **Glamour:** `github.com/charmbracelet/glamour` (Markdown rendering)

