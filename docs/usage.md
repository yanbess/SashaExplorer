# Using SashaExplorer

SashaExplorer is a file manager for Windows, made for fans of Cyberpunk:
Edgerunners. This page describes everything it does.

`SashaExplorer.exe D:\some\folder` opens the folder in a tab. Only one window
runs, so a second start passes its folders to it. A folder that is already open
in a tab, in either pane, is shown there instead of opening again; with several
such tabs, the last one is shown. A path to a file opens its folder with the
file selected.

## Window

- **Sidebar:** This PC, then places: the desktop and the drives at first. Drag a
  folder from the list to the edge of a place to add it, drag places to reorder
  them, and remove them with the cross, which asks first. A drive taken out stays
  out, while a new drive, such as a USB stick, joins on its own.
- **Recycle Bin** closes the list of places: its icon fills up with its content,
  its button or right click empties it, and items dropped on it are deleted
  there. A click opens it in a tab with every drive's deleted items, where they
  were, and when they were deleted. **Restore** puts items back, and undo
  deletes them again; Del deletes them for good.
- **Recent** lists opened folders by their path, with the middle left out when
  it is long. As in VS Code, it sits at the bottom of the sidebar and starts
  folded; click its title to unfold it, and drag its top edge to change its
  height. A double click on the edge gives it its first height back. The
  program remembers both.
- **Tabs** come back when the program opens again, with their pins. Right click a
  tab to pin it, duplicate it, or close others or those to the right. Group
  commands leave pinned tabs alone. A middle click closes a tab, or opens a
  folder in a new tab.
- **Split:** drag a tab to the right side of the list, or press the split button
  or Ctrl+\\, to see two folders side by side. Each side has its own tabs; drag a
  tab onto the other side to move it there. The button again puts the sides back
  together.
- **Splitters** between the sidebar, the panes, and the preview panel resize them
  by dragging. A double click puts them back to their first size.
- **Address bar:** click a part of the path to go there, or an empty spot to type
  a path. Environment variables such as `%APPDATA%` work, and a path to a file
  opens its folder with the file selected.
- **Views:** details, medium icons, and gallery. In the gallery, Ctrl+wheel
  changes the size. Each folder keeps its own view and sort order, by name, date
  modified, date created, type, or size. A folder with pictures shows the first
  of them in its icon.
- **Folder sizes:** the Size column adds up folders in the background, a few at a
  time, and remembers the totals for ten minutes.
- **Search:** type in the filter to filter the folder; press Enter to search its
  subfolders too. `*` and `?` work as in `*.jpg`. Escape goes back to the folder.
- **Thumbnails and icons** come from Windows, so they match Explorer, videos and
  documents included. They are saved in
  `%LOCALAPPDATA%\SashaExplorer\Thumbnails2`, up to 1 GB, encrypted for the
  current Windows user.
- **Preview panel:** images, videos and audio with playback, the start of text
  files, and file details. Videos and songs list their container, and each
  stream with its codec, resolution, frame rate, bitrate, bit depth, HDR,
  channels, sample rate, and language, plus subtitles and tags; photos list
  their camera settings. This comes from [ffprobe](https://ffmpeg.org) when it is
  on the PATH or next to the program, and from Windows otherwise, which knows
  fewer formats.
- **Context menu:** the program's own commands, and **More options** for the full
  Windows menu with the commands of other programs.

## Files

Dragging items is a Windows drag, so they can go to Explorer, a mail, a chat, or
any other program, and into other tabs and the other side of a split. Holding
them over a tab for a moment opens it.

Copy, cut, paste, delete, and drops run through Windows, with its progress and
conflict dialogs and the Recycle Bin. The clipboard is shared with Explorer both
ways. Within one drive a drop moves, and to another drive it copies; hold Ctrl to
copy or Shift to move. Files can be dropped in from other programs too.

**Undo and redo** (Ctrl+Z, Ctrl+Y, and the arrows) cover renames, new items,
moves, copies, and deletions to the Recycle Bin, up to 50 steps. Undoing a
deletion brings the items back from the Recycle Bin; undoing a copy moves the
copies there. Permanent deletion and commands from the Windows menu cannot be
undone, and replacing a file in a copy keeps the new one.

## Terminals

The terminal button opens the current folder in the default terminal; its arrow
lists every terminal on the computer:

- LucyShell, when `LucyShell.exe` is next to `SashaExplorer.exe`
- Windows PowerShell, and PowerShell 7 when it is installed
- Command Prompt
- Git Bash
- Windows Terminal
- each WSL distribution

**Settings → Terminal** chooses the default and adds terminals of your own: a
program and its arguments, where `{folder}` becomes the folder, quoted. The
program also starts in the folder, so many shells need no arguments at all.

Any terminal can be deleted from the list with its bin button, after a
question. A deleted terminal that was found on the computer is not found again,
but can be added by hand. Terminals added by hand can also be changed.

Without a choice, the default is LucyShell when it is there, and Windows
PowerShell otherwise.

## Keyboard and mouse

- **Ctrl+T, Ctrl+W, Ctrl+Tab, Ctrl+1…9:** tabs.
- **Alt+Left, Alt+Right, Alt+Up, Backspace,** mouse side buttons: back, forward, up.
- **Ctrl+L or Alt+D:** type a path. **Ctrl+F**, or just start typing: filter.
- **Enter, Ctrl+Enter:** open, open in a new tab. **F2:** rename.
- **Del, Shift+Del:** Recycle Bin, delete permanently (asks first).
- **Ctrl+C, Ctrl+X, Ctrl+V, Ctrl+Shift+C:** copy, cut, paste, copy path.
- **Ctrl+Z, Ctrl+Y:** undo, redo. **Ctrl+\\:** split.
- **Ctrl+Shift+N:** new folder. **F5:** refresh. **Alt+Enter:** properties.

## Settings

The gear at the bottom of the sidebar opens Settings.

- **Appearance:** the theme (System, Light, or Dark; System follows the colors
  Windows uses for apps), hidden files, Sasha icons, and the preview panel.
  **Sasha icons** draws files and folders in the style of the app; programs and
  shortcuts keep their own icons, and pictures and videos show themselves.
- **Window:** whether the window opens where it was closed and as large, and
  whether it opens maximized when it was closed so. **Reset** puts it back to its
  first size in the middle of the screen.
- **Terminal:** the default terminal, and terminals of your own.
- **Explorer:**
  - **"Open in SashaExplorer" in Explorer** adds the item to the menu of folders
    and drives. It writes only to the current user's part of the registry; on
    Windows 11 the item is under "Show more options". Moving the program means
    turning it on again.
  - **Open folders and Win+E here** makes SashaExplorer the program for folders:
    a folder opened from the desktop, or by a program that asks Windows to open
    it, opens here, and so does Win+E. It changes only the default action of
    folders and drives and the Win+E launcher in the current user's part of the
    registry, keeps what was there before, and puts it back when turned off.
    Explorer itself keeps working. "Show in folder" in many programs, and
    "Reveal in File Explorer" in VS Code, ask Windows for Explorer by name, so
    they still open Explorer; for VS Code there is an extension instead.
  - **Saved thumbnails** can be cleared.
- **Updates:** a daily check for a new release on GitHub, and **Check now**. When
  there is one, a note offers its page or to skip that version. Nothing is
  downloaded or installed by itself.

## VS Code

`vscode-extension` adds **Reveal in SashaExplorer** to the menus of files,
folders, and editor tabs, on Shift+Alt+R. Pack it with
`python vscode-extension/build_vsix.py` and install the `.vsix` with
`code --install-extension`. It finds the program through the path SashaExplorer
records each time it starts.

## Data

`sashaexplorer.state` next to the executable holds tabs, places, recent folders,
layout, and settings. It is protected with Windows DPAPI for the current user,
because a list of folders says a lot about someone. Delete it to start over.

## LucyShell

LucyShell, a terminal from the same author, can also show a folder here: the
folder button in its Folders list, or "Open folder in SashaExplorer" in a tab's
menu. SashaExplorer looks for `LucyShell.exe` next to `SashaExplorer.exe`, or in
`../LucyShell/build/bin` during development.
