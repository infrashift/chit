---
title: Custom Themes
description: Create your own Chit TUI theme with a TOML file.
---

You can create your own themes by placing TOML files in the themes directory.

## Theme File Location

Chit TUI looks for your themes in:

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
`~/.config/chit/config.toml`, or `CHIT_THEME=dracula`. Your themes also
appear in the `/theme` picker and in `chit-tui --list-themes`, marked
`(local)`.

### Naming Rules

- **Lowercase file names only.** Names are lowercased before the file is
  looked up, so `Dracula.toml` would never be found, and it is left out of the
  lists rather than offered and then failing.
- **Bundled names win.** A file named like a bundled theme, such as
  `kanagawa.toml`, is shadowed by the bundled one and is not listed. Bundled
  themes are checked first so a local file cannot change what a documented
  name means.
- Names cannot contain path separators or `..`.

## Theme File Format

A theme file is TOML. Unlike the config file, it must be complete: every
palette key is required, so a theme can never render half-styled because of a
typo.

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
`dark_gray`), and `grey` spellings work too. A named color renders as
whatever your terminal sets that slot to, so a theme built from names blends
into your existing terminal color scheme instead of overriding it.

Two names are counter-intuitive: `gray` is ANSI color 7 (the terminal's
normal white) and `white` is ANSI color 15 (bright white).

## Color Fields

`name` is required; `author` is optional. Every color below is required
unless marked derived.

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

The three derived fields are optional. When a file leaves them out they are
computed by blending `background` toward `accent` (selection) or `warning`
(search matches), so they suit the palette you actually declared.

## Errors and Warnings

A key the schema does not recognize is ignored with a warning; it is most
likely left over from an older version.

A missing key or a bad color is an error, and every problem in the file is
reported at once so you can fix them in one pass. What happens next depends
on where the name came from:

- `--theme dracula` stops the client and shows the errors.
- `theme = "dracula"` in the config file prints a warning and falls through to
  the next theme choice (see [How the Theme Is Chosen](/chit/tui/guides/themes/#how-the-theme-is-chosen)).

Silently filling in defaults would leave you looking at a theme that is not
the one you asked for, which is why a broken theme file is never patched up.

## Legacy JSON Skins

Before theme files moved to TOML, custom themes were JSON files in
`~/.config/chit/skins/<name>.json`. These still load: when no
`themes/<name>.toml` exists, the skins directory is checked next, and a
warning asks you to move the theme to the themes directory as a `.toml` file.
Legacy skins are listed by `/theme` and `--list-themes` alongside TOML themes.

They keep their old behavior: any field a JSON skin omits falls back to the
Tokyo Night value, and only hex colors are accepted. To convert one, copy its
keys into a `.toml` file, add any of the required keys it was missing, and
delete the JSON file.

## Resolution Order

For any theme name, the loader checks in order:

1. **Bundled themes** — `tokyo-night`, `tokyo-night-day`, `catppuccin`,
   `catppuccin-latte`, `kanagawa`, `kanagawa-lotus`, `nightfox`, `dayfox`
2. **Your theme file** — `~/.config/chit/themes/<name>.toml`
3. **Legacy skin** — `~/.config/chit/skins/<name>.json`, with a warning

If none matches, the name is unknown: an error for `--theme`, a warning for a
configured theme.
