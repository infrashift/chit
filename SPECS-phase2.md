# SPECS-phase2.md: Technical Specification for Skinning

## 1. Data Structures

Define a `Skin` struct in a new `internal/tui/ui/theme` package:

```go
type Theme struct {
    Name        string
    Author      string
    Background  string // Hex
    Foreground  string // Hex
    Accent      string // For active tabs/selections
    Border      string // For component delimiters
    Success     string // For status indicators
    Error       string // For AppError states
    Dimmed      string // For timestamps and metadata
}

```

* **LazyVim Definition:** Hardcode or load a `tokyonight.cue` with these values:
* Background: `#1a1b26`
* Foreground: `#a9b1d6`
* Accent: `#7aa2f7` (Blue) or `#bb9af7` (Purple)



## 2. Component Integration

* **Global Styles:** Refactor `internal/tui/ui/styles` to be a function-based package that accepts a `Skin` object and returns a `StyleSheet`.
* **Lipgloss Adoption:** All components must use the `StyleSheet` for rendering. No hardcoded `lipgloss.Color("#XXXXXX")` values are allowed in component files.

## 3. Configuration & Loading

* **CUE Loader:** Extend the existing `internal/command/cue_loader.go` logic to detect and parse skin files from a `~/.config/chit/skins/` directory.
* **Auto-provisioning:** If no skin is selected, the TUI must default to the LazyVim skin.

## 4. Reference Repositories for Claude Code

To ensure Claude understands the styling engine and the target aesthetic, it should review:

1. **Chit Server:** `github.com/infrashift/chit` (specifically `internal/model` and `api/openapi.yaml`).
2. **Lipgloss:** `github.com/charmbracelet/lipgloss` (The underlying styling primitive).
3. **Tokyonight.nvim:** `github.com/folke/tokyonight.nvim` (Reference for the LazyVim color hex codes).