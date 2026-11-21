---
title: Attachments
description: Attach images to a post in Chit TUI, and save the files of a post to disk.
---

A post can have up to four image files. The server accepts PNG, JPEG, WebP
and GIF images of 10 MB or less. It makes each image again: a JPEG stays a
JPEG, and each other type becomes a PNG.

## Attach an Image

1. Type `/attach` and the path of the file, for example
   `/attach ~/charts/sales.png`. To open a file picker, type `/attach`
   with no path.
2. Write the alt text: what the image shows, for people who cannot see
   it. Then push `Enter`. You can leave the alt text empty.
3. The TUI sends the file to the server. The lower border of the main pane
   shows `uploading sales.png…` until the server answers.
4. The file waits in a tray on the upper border of the input:

   ```
   ╭── [file] sales.png  · backspace removes · /detach ──╮
   ```

5. Write a message.
6. Push `Enter`. The post takes the files of the tray.

To send the files with no text, push `Enter` in the empty input. To cancel
an attachment, push `Esc` in the alt text prompt.

In the file picker, `↑`/`↓` move. `→` or `Enter` opens a directory or picks
a file. `←` goes back, and `Esc` closes the picker.

Before it sends a file, the TUI examines the type and the size. If the
server cannot accept the file, the action bar tells you why.

### Remove a File from the Tray

- `Backspace` in the empty input removes the last file of the tray.
- `/detach` removes each file of the tray.
- A switch to another channel empties the tray, because a file belongs to
  the channel that you sent it to.

The server deletes a file that no post takes after 24 hours.

## Save the Files of a Post

1. Move the cursor to the post in the history pane.
2. Press `i` to open the post details.
3. Press `s`. The TUI saves each file of the post to the download
   directory.

The action bar tells how many files the TUI saved.

The download directory is the `download_dir` setting. When it has no
value, the TUI uses the XDG download directory, or `~/Downloads`. The
`CHIT_DOWNLOAD_DIR` variable overrides both.

The TUI does not write over a file. If the directory already has a file of
that name, the TUI adds a number: `sales (1).png`. The TUI uses only the
name of the file, never a path that the name holds.
