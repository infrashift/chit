# PRD-phase2.md: Project Chit-TUI Skinning System

**Version:** 1.0

**Status:** Draft

**Project Owner:** Ryan Craig

**Core Stack:** Go, Lipgloss, CUE (for configuration)

## 1. Executive Summary

**Chit-TUI Themes** is a feature allowing users to customize the visual appearance of the `chit` terminal client. The system must support a "LazyVim" (Tokyonight) default theme and provide a mechanism for users to define and switch between custom color palettes without modifying the core Go logic.

## 2. Functional Requirements

* **Themed Components:** Every UI element (Sidebar, Viewport, Textarea, Modals) must inherit colors from a central `Theme` object.
* **Default Theme (Tokyonight):** The initial release must include a skin that mimics the LazyVim/Tokyonight color palette (Deep blues, purples, and vibrant accents).
* **Hot-Swappable:** The TUI should ideally reflect theme changes upon restart or via a `/skin` slash command.
* **External Definition:** Themes should be definable in CUE or JSON to align with the project's existing configuration patterns.

#### 3. Success Criteria

* The TUI successfully renders using the LazyVim palette.
* Adding a new skin file results in a functional UI change without refactoring `lipgloss.Style` definitions in the component files.



