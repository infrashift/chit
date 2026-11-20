---
title: Tags
description: Private tags, channel tags, the rules for a tag name, and how to rename, delete and find tags in Chit TUI.
---

A tag marks a post, or a channel, with a word such as `#deploy`. Chit has
two kinds of tags:

| Kind | Who sees it | Where you can use it |
|------|-------------|----------------------|
| Channel tag | Each member of the channel | On the posts of that channel, and on the channel |
| Private tag | Only you | On each post that you can read |

The user who makes a tag owns it. Only the owner can rename or delete it.

## Tag a Post

- Write `#name` in a message. When you send it, the TUI applies the channel
  tag of that name. If the channel has no such tag, the TUI makes one.
- Or press `t` on a post to open the tag picker. Type to filter, and push
  `Enter` to apply or remove the tag under the cursor. `Ctrl+N` makes a
  new tag from the text. `Ctrl+T` changes the kind of the new tag from a
  channel tag to a private tag.

## Tag Names

A tag name has 1 to 50 characters. Each character is a letter, a digit, a
space, `-`, `_` or `.`. Letters and digits of each script are correct,
such as `résumé` or `日本語`. The server compares names without regard to
case. The prefix `profile/` is for machines only.

A `#tag` in a message stops at a space or at a `.`. So a period at the
end of a sentence is not part of the tag. Use the tag picker for a name with a
space or a `.`.

The TUI examines each name before it sends it. If a name does not obey these
rules, the action bar tells you why.

## Rename or Delete a Tag

In the tag picker, move the cursor to a tag that you own:

- `Ctrl+R` opens a prompt with the name. Type the new name and push
  `Enter`. If the name is in use, the prompt tells you so. `Esc` cancels.
- `Ctrl+X` asks first. Push `Ctrl+X` again to delete the tag from each post
  and each channel. Any other key cancels.

When another member renames or deletes a channel tag, the TUI shows the
change at once.

## Channel Tags

The tags of the open channel show on the upper border of the main pane:

```
╭── #deploy #release ───────────────────────────────╮
```

Use `/chtag` to change them:

| Command | Action |
|---------|--------|
| `/chtag add <name>` | Apply the tag to the open channel. A name with no tag makes a channel tag first. |
| `/chtag rm <name>` | Remove the tag from the open channel. The TUI asks first; push `y` to confirm. |
| `/chtag find <name> …` | List the channels of which you are a member and that carry each of the tags (1 to 8 tags). Push the digit of a channel to open it. |

A `#` before a name is optional: `/chtag add #deploy` and
`/chtag add deploy` do the same thing.
