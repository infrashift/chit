---
title: Custom Themes
description: Create your own Chit TUI theme with a TOML file.
---

To create a theme, put a TOML file in the themes directory.

## Theme File Location

Chit TUI looks for local themes in:

```
~/.config/chit/themes/<name>.toml
```

The file name, without `.toml`, is the theme's name. Use it anywhere a
bundled theme name works:

```bash
chit-tui --theme dracula
# Loads ~/.config/chit/themes/dracula.toml
```

or `theme = "dracula"` (or `theme_dark` / `theme_light`) in
`~/.config/chit/config.toml`, or `CHIT_THEME=dracula`. Local themes also
appear in the `/theme` picker and in `chit-tui --list-themes`, marked
`(local)`.

### Edit a Theme While the Client Runs

You can edit the file of the active theme while the client runs. The client
examines the file every 2 seconds and applies each saved change. To apply a
change at once, type `/theme reload`. If the file has an error, the client
shows the error in the action bar and keeps the current theme. Set
`theme_watch = false` in the config file to stop the automatic reload.

### Naming Rules

- **Lowercase file names only.** The client changes a name to lowercase
  before it looks up the file, so the client never finds `Dracula.toml`. The
  lists do not show this file, so the client does not offer a theme that then
  fails.
- **Bundled names win.** A bundled theme hides a file with the same name, such
  as `kanagawa.toml`, and the lists do not show that file. The client checks
  the bundled themes first, so a local file cannot change what a documented
  name means.
- Names cannot contain path separators or `..`.

## Theme File Format

A theme file is TOML. Unlike the config file, it must be complete. It must
set every palette key, so a typo can never make a theme with only half of its
styles.

```toml
# ~/.config/chit/themes/dracula.toml
name   = "Dracula"
author = "Dracula Theme"

background      = "#282a36"
foreground      = "#f8f8f2"
subtle          = "#6272a4"
accent          = "#bd93f9"
error           = "#ff5555"
success         = "#50fa7b"
warning         = "#f1fa8c"
border          = "#44475a"
active_border   = "#bd93f9"
highlight       = "#44475a"
muted           = "#6272a4"
username        = "#ff79c6"
timestamp       = "#6272a4"
unread_badge    = "#8be9fd"
pin_badge       = "#f1fa8c"
channel_active  = "#bd93f9"
mention_badge   = "#ff5555"
mention_text    = "#bd93f9"
mention_self_bg = "#f1fa8c"
tag_badge       = "#50fa7b"
```

The schema is `clients/chit-tui/internal/config/schema/theme.cue`.

### Colors

A color is either a hex value (`#rrggbb` or `#rgb`) or the name of one of the
terminal's own palette colors:

`black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `gray`,
`darkgray`, `lightred`, `lightgreen`, `lightyellow`, `lightblue`,
`lightmagenta`, `lightcyan`, `white`

The `light`/`dark` names also accept an underscore (`light_blue`,
`dark_gray`), and `grey` spellings work too. A named color shows the
value that the terminal sets for that slot. As a result, a theme of names blends into
the current color scheme of the terminal, and does not override it.

Two names are not what they seem. `gray` is ANSI color 7 (the terminal's
normal white). `white` is ANSI color 15 (bright white).

## Color Fields

A theme must set `name`; `author` is optional. A theme must set every color
below, except the colors marked derived.

| Field | Used For |
|-------|----------|
| `background` | Base background: code-block backgrounds, and the color the derived fields are blended from |
| `foreground` | Default text color |
| `subtle` | De-emphasized text: status bar text, reply indents |
| `accent` | Primary accent: overlay borders, bar buttons, links |
| `error` | Error messages and the disconnected indicator |
| `success` | Success indicators |
| `warning` | Warning indicators |
| `border` | Inactive panel borders |
| `active_border` | Focused panel borders |
| `highlight` | Selected post background, status bar background |
| `muted` | Low-contrast text such as day separators |
| `username` | Username display in posts |
| `timestamp` | Timestamps and post badges (`[edited]`, `[N replies]`) |
| `unread_badge` | Unread count badge on palette rows; connected indicator |
| `pin_badge` | Pinned post badge |
| `channel_active` | Active row in lists and pickers |
| `mention_badge` | Mention count badge on palette rows |
| `mention_text` | `@mentions` in posts |
| `mention_self_bg` | Background behind mentions of you |
| `tag_badge` | `#tag` badges on posts |
| `selection` | *Derived.* Background of lines selected with the mouse |
| `search_match` | *Derived.* Background of search-term matches in the history |
| `search_match_active` | *Derived.* Background of the current search match |

The three derived fields are optional. When a file leaves them out, the client
calculates them. It blends `background` toward `accent` (selection) or
`warning` (search matches), so these colors suit the palette that the file
declares.

## Errors and Warnings

The client ignores a key that the schema does not recognize, with a warning.
Such a key probably comes from an older version.

An absent key or a bad color is an error. The client reports all problems in
the file at once, so you can fix them in one pass. The next step depends on
the source of the name:

- `--theme dracula` stops the client and shows the errors.
- `theme = "dracula"` in the config file prints a warning and falls through to
  the next theme choice (see [the theme order](/chit/tui/guides/themes/#how-the-theme-is-chosen)).

The client never adds default values to a broken theme file. With silent
defaults, the user sees a theme that is different from the requested theme.

## Legacy JSON Skins

Before theme files moved to TOML, custom themes were JSON files in
`~/.config/chit/skins/<name>.json`. These files still load. When no
`themes/<name>.toml` exists, the client looks in the skins directory next. A
warning then asks you to move the theme to the themes directory as a `.toml`
file. `/theme` and `--list-themes` show legacy skins together with TOML themes.

Legacy skins keep their old behavior. Each field that a JSON skin omits falls
back to the Tokyo Night value. The client accepts only hex colors in a skin.
To convert a skin, do these steps:

1. Copy its keys into a `.toml` file.
2. Add the mandatory keys that the skin does not have.
3. Delete the JSON file.

## Resolution Order

For any theme name, the loader checks in order:

1. **Bundled themes** — `tokyo-night`, `tokyo-night-day`, `catppuccin`,
   `catppuccin-latte`, `kanagawa`, `kanagawa-lotus`, `nightfox`, `dayfox`
2. **Local theme file** — `~/.config/chit/themes/<name>.toml`
3. **Legacy skin** — `~/.config/chit/skins/<name>.json`, with a warning

If none matches, the name is unknown: an error for `--theme`, a warning for a
configured theme.
