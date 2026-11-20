---
title: Themes
description: Built-in color themes for Chit TUI, and how the active theme is chosen.
---

Chit TUI ships with eight bundled themes: four families, each with a dark and
a light variant. You can also write a theme — see
[Custom Themes](/chit/tui/guides/custom-themes/).

| Family | Dark | Light |
|--------|------|-------|
| Tokyo Night | `tokyo-night` (default) | `tokyo-night-day` |
| Catppuccin | `catppuccin` | `catppuccin-latte` |
| Kanagawa | `kanagawa` | `kanagawa-lotus` |
| Nightfox | `nightfox` | `dayfox` |

## Choosing a Theme

There are four ways to pick one:

- **In the client** — see [Change the Theme While the Client Runs](#change-the-theme-while-the-client-runs).
  The client also saves the theme to the config file (see below).
- **For one run** — `chit-tui --theme kanagawa`.
- **In the config file** — `theme = "kanagawa"` in
  `~/.config/chit/config.toml`.
- **In the environment** — `export CHIT_THEME=kanagawa`.

To follow the terminal instead of one fixed theme, set a theme for each
appearance and leave `theme` unset:

```toml
# ~/.config/chit/config.toml
theme_dark  = "kanagawa"
theme_light = "kanagawa-lotus"
```

`chit-tui --list-themes` prints every theme name you can use: the bundled
ones, then the local ones, marked `(local)`. The `/theme` picker lists the same set.

## Change the Theme While the Client Runs

A theme change applies at once. You do not restart the client.

- **`Ctrl+T` or `/theme`** opens the theme picker on the active theme.
  `↑`/`↓` (or `k`/`j`) shows each theme on the full screen as the cursor
  moves to it. `Enter` keeps the theme and saves it. `Esc` puts the theme
  from before the picker back, and saves nothing.
- **`/theme <name>`** applies and saves one theme, for example
  `/theme kanagawa`. For a name that does not exist, the client shows the
  names that are near it.
- **`/theme reload`** reads the active theme file again. Use it after you
  edit a [custom theme](/chit/tui/guides/custom-themes/).

The client also reads an edited local theme file again by itself. Every 2
seconds, it examines the time of the last change of the active theme file.
When the time changes, the client loads the file and applies it. If the
file has an error, the client shows the error and keeps the current theme.
To stop this, set `theme_watch = false` in the config file.

### Colors of the Terminal

The client sets the default foreground and background of the terminal to
the colors of the theme. It uses the OSC 10 and OSC 11 control sequences.
So the cells that the client does not draw on, such as the empty part of a
short pane, also show the theme. When the client stops, it gives the
terminal its own colors back with OSC 110 and OSC 111.

Most terminals accept these sequences: xterm, kitty, WezTerm, Alacritty,
iTerm2, the VTE terminals and tmux 3.3 or later. A terminal that does not
accept them shows its own colors. To keep the colors of the terminal, set
`theme_paint = false` in the config file.

The message input and the search boxes also take the colors of the theme.

## How the Theme Is Chosen

The first of these settings that has a value wins:

1. `--theme` on the command line
2. `theme` — from `CHIT_THEME` if set, otherwise from the config file
3. `theme_dark` or `theme_light`, whichever matches the appearance
4. The default: `tokyo-night` on a dark terminal, `tokyo-night-day` on a
   light one

The appearance comes from `--appearance`, then the `appearance` config key,
then `system`. With `system`, chit-tui asks the terminal whether its
background is dark, once, before the interface starts. When stdin or stdout is
not a terminal there is nobody to ask, and it assumes dark.

Appearance only matters for steps 3 and 4. If `--theme` or `theme` names a
theme outright, the client ignores a configured appearance with a warning.

A name passed to `--theme` that does not exist is an error. The client stops
and lists the bundled themes. A configured name that does not resolve is
only a warning. In that case, resolution falls through to the next step. A
person who typed a name a moment before wants to know that the name is wrong.
A config file that is out of date must not stop the client when it starts.

The client matches theme names without regard to case: `--theme Kanagawa`
works.

## How `/theme` Saves Your Choice

The client writes a theme picked with `/theme` to the config file, so the
choice survives a restart. The client changes only that one line. It keeps the
comments and the order of the other keys. If the file does not exist, the
client creates it.

The key that the client writes depends on the theme settings:

- If `theme_dark` or `theme_light` has a value and `theme` has no value, the
  client saves the pick to the key for the **current** appearance. A pick on
  a light terminal sets `theme_light` and does not change `theme_dark`. A
  value in `theme` outranks both keys. It can turn off the change between
  appearances permanently, so the client does not write `theme` in this case.
- If not, the client saves the pick as `theme`.

`CHIT_THEME` still overrides the file. If the shell sets `CHIT_THEME`, the
next start does not use the saved choice.

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

A soft pastel palette from [Catppuccin](https://github.com/catppuccin/nvim). Lavender blues and mauve purples on a warm dark base.

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
