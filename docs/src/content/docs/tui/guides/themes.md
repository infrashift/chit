---
title: Themes
description: Built-in color themes for Chit TUI, and how the active theme is chosen.
---

Chit TUI ships with eight bundled themes: four families, each with a dark and
a light variant. You can also write your own — see
[Custom Themes](/chit/tui/guides/custom-themes/).

| Family | Dark | Light |
|--------|------|-------|
| Tokyo Night | `tokyo-night` (default) | `tokyo-night-day` |
| Catppuccin | `catppuccin` | `catppuccin-latte` |
| Kanagawa | `kanagawa` | `kanagawa-lotus` |
| Nightfox | `nightfox` | `dayfox` |

## Choosing a Theme

There are four ways to pick one:

- **In the client** — type `/theme` (or its alias `/skin`) and pick from the
  list with `↑`/`↓` (or `j`/`k`) and `Enter`. The new theme applies at once,
  with no restart, and is saved to the config file (see below).
- **For one run** — `chit-tui --theme kanagawa`.
- **In the config file** — `theme = "kanagawa"` in
  `~/.config/chit/config.toml`.
- **In the environment** — `export CHIT_THEME=kanagawa`.

To follow the terminal instead of fixing one theme, set a theme per
appearance and leave `theme` unset:

```toml
# ~/.config/chit/config.toml
theme_dark  = "kanagawa"
theme_light = "kanagawa-lotus"
```

`chit-tui --list-themes` prints every theme name you can use: the bundled
ones, then your own, marked `(local)`. The `/theme` picker lists the same set.

## How the Theme Is Chosen

The first of these that is set wins:

1. `--theme` on the command line
2. `theme` — from `CHIT_THEME` if set, otherwise from the config file
3. `theme_dark` or `theme_light`, whichever matches the appearance
4. The default: `tokyo-night` on a dark terminal, `tokyo-night-day` on a
   light one

The appearance comes from `--appearance`, then the `appearance` config key,
then `system`. With `system`, chit-tui asks the terminal whether its
background is dark, once, before the interface starts. When stdin or stdout is
not a terminal there is nobody to ask, and it assumes dark.

Appearance only matters for steps 3 and 4. If a theme is named outright by
`--theme` or `theme`, a configured appearance is ignored with a warning.

A name passed to `--theme` that does not exist is an error and the client
stops, listing the bundled themes. A configured name that does not resolve is
only a warning, and resolution falls through to the next step. Someone who
just typed a name wants to know it was wrong; a config file that has aged out
of date should not stop the client from starting.

Theme names are matched without regard to case: `--theme Kanagawa` works.

## How `/theme` Saves Your Choice

A theme picked with `/theme` is written to the config file so it survives a
restart. Only that one line is rewritten; your comments and the order of the
other keys are kept. The file is created if it does not exist.

Which key it writes depends on how you configured themes:

- If `theme_dark` or `theme_light` is set and no fixed `theme` is, the pick
  is saved to the key for the **current** appearance. Picking a theme on a
  light terminal sets `theme_light` and leaves `theme_dark` alone. Saving to
  `theme` instead would outrank both and turn appearance switching off for
  good.
- Otherwise it is saved as `theme`.

`CHIT_THEME` still overrides the file, so if it is set in your shell, the
saved choice is not what you get on the next start.

## Tokyo Night (default)

The default theme on a dark terminal. Cool blue tones with purple accents,
inspired by the [Tokyo Night](https://github.com/folke/tokyonight.nvim) color
scheme.

```bash
chit-tui --theme tokyo-night
```

| Element | Color |
|---------|-------|
| Background | `#1a1b26` |
| Foreground | `#c0caf5` |
| Accent | `#7aa2f7` |
| Username | `#bb9af7` |
| Error | `#f7768e` |
| Success | `#9ece6a` |
| Warning | `#e0af68` |

## Tokyo Night Day

The light variant, and the default on a light terminal.

```bash
chit-tui --theme tokyo-night-day
```

| Element | Color |
|---------|-------|
| Background | `#e1e2e7` |
| Foreground | `#3760bf` |
| Accent | `#2e7de9` |
| Username | `#9854f1` |
| Error | `#c64343` |
| Success | `#587539` |
| Warning | `#8c6c3e` |

## Catppuccin Mocha

A soothing pastel palette from [Catppuccin](https://github.com/catppuccin/nvim). Lavender blues and mauve purples on a warm dark base.

```bash
chit-tui --theme catppuccin
```

| Element | Color |
|---------|-------|
| Background | `#1e1e2e` |
| Foreground | `#cdd6f4` |
| Accent | `#89b4fa` |
| Username | `#cba6f7` |
| Error | `#f38ba8` |
| Success | `#a6e3a1` |
| Warning | `#f9e2af` |

## Catppuccin Latte

Catppuccin's light flavor.

```bash
chit-tui --theme catppuccin-latte
```

| Element | Color |
|---------|-------|
| Background | `#eff1f5` |
| Foreground | `#4c4f69` |
| Accent | `#1e66f5` |
| Username | `#8839ef` |
| Error | `#d20f39` |
| Success | `#40a02b` |
| Warning | `#df8e1d` |

## Kanagawa

Warm, muted tones inspired by Japanese ink paintings and the [Kanagawa.nvim](https://github.com/rebelot/kanagawa.nvim) color scheme.

```bash
chit-tui --theme kanagawa
```

| Element | Color |
|---------|-------|
| Background | `#1f1f28` |
| Foreground | `#dcd7ba` |
| Accent | `#7e9cd8` |
| Username | `#957fb8` |
| Error | `#e82424` |
| Success | `#98bb6c` |
| Warning | `#e6c384` |

## Kanagawa Lotus

Kanagawa's light variant.

```bash
chit-tui --theme kanagawa-lotus
```

| Element | Color |
|---------|-------|
| Background | `#f2ecbc` |
| Foreground | `#545464` |
| Accent | `#4d699b` |
| Username | `#624c83` |
| Error | `#c84053` |
| Success | `#6f894e` |
| Warning | `#77713f` |

## Nightfox

A deep navy palette with subtle steel blue accents from [Nightfox.nvim](https://github.com/EdenEast/nightfox.nvim).

```bash
chit-tui --theme nightfox
```

| Element | Color |
|---------|-------|
| Background | `#192330` |
| Foreground | `#cdcecf` |
| Accent | `#719cd6` |
| Username | `#9d79d6` |
| Error | `#c94f6d` |
| Success | `#81b29a` |
| Warning | `#dbc074` |

## Dayfox

The light member of the Nightfox family.

```bash
chit-tui --theme dayfox
```

| Element | Color |
|---------|-------|
| Background | `#f6f2ee` |
| Foreground | `#3d2b5a` |
| Accent | `#2848a9` |
| Username | `#6e33ce` |
| Error | `#a5222f` |
| Success | `#396847` |
| Warning | `#ac5402` |
