---
title: Custom Themes
description: Create your own Chit TUI theme with a JSON file.
---

You can create custom themes by placing JSON files in the skins directory.

## Theme File Location

Chit TUI looks for custom themes in:

```
~/.config/chit/skins/<name>.json
```

Set `CHIT_THEME` to the filename without the `.json` extension:

```bash
export CHIT_THEME=dracula
# Loads ~/.config/chit/skins/dracula.json
```

## Theme File Format

A theme file is a JSON object with color values as hex strings:

```json
{
  "name": "Dracula",
  "author": "Dracula Theme",
  "background": "#282a36",
  "foreground": "#f8f8f2",
  "subtle": "#6272a4",
  "accent": "#bd93f9",
  "error": "#ff5555",
  "success": "#50fa7b",
  "warning": "#f1fa8c",
  "border": "#44475a",
  "active_border": "#bd93f9",
  "highlight": "#44475a",
  "muted": "#6272a4",
  "username": "#ff79c6",
  "timestamp": "#6272a4",
  "unread_badge": "#8be9fd",
  "pin_badge": "#f1fa8c",
  "channel_active": "#bd93f9"
}
```

## Color Fields

| Field | Used For |
|-------|----------|
| `background` | Main background color |
| `foreground` | Default text color |
| `subtle` | De-emphasized text |
| `accent` | Primary accent (links, active items) |
| `error` | Error messages |
| `success` | Success indicators |
| `warning` | Warning indicators |
| `border` | Inactive panel borders |
| `active_border` | Focused panel borders |
| `highlight` | Selected/highlighted row background |
| `muted` | Low-contrast text |
| `username` | Username display in posts |
| `timestamp` | Timestamp display in posts |
| `unread_badge` | Unread count badge on palette rows |
| `pin_badge` | Pinned post badge |
| `channel_active` | Active channel highlight in list rows |

## Partial Themes

You don't need to specify every field. Any omitted field falls back to the Tokyo Night default:

```json
{
  "name": "Minimal Dark",
  "author": "me",
  "background": "#000000",
  "foreground": "#ffffff",
  "accent": "#ff6600"
}
```

All other colors (border, error, success, etc.) will use the Tokyo Night values.

## Resolution Order

When `CHIT_THEME` is set, the loader checks in order:

1. **Built-in themes** — `tokyo-night`, `catppuccin`, `kanagawa`, `nightfox`
2. **Custom file** — `~/.config/chit/skins/<name>.json`
3. **Fallback** — Tokyo Night (if nothing matches)
