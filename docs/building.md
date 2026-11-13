# Building from source

## Requirements

- Windows 10 1809 or later. The program uses the Windows shell, so it builds and
  runs on Windows only.
- [Go](https://go.dev/dl/), the version in `go.mod` or later
- [Node.js](https://nodejs.org) 20 or later
- The [Wails v2](https://wails.io) CLI:
  `go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1`
- The Microsoft WebView2 Runtime, which comes with Windows 11

`wails doctor` checks all of this.

## Commands

```
wails dev                      # run with live reload
wails build -trimpath          # produce build/bin/SashaExplorer.exe
go test ./...                  # run backend tests
cd frontend && npx svelte-check   # check the frontend types
```

Run `wails build` once before `go test ./...`, because the root package embeds
`frontend/dist`. Do not add `-clean`: it deletes `build/bin` with
`sashaexplorer.state`.

A local build reports its version as `dev`, which turns off the update check. A
release sets it from the tag:

```
wails build -trimpath -ldflags "-X main.version=0.2.0"
```

## Layout

- `main.go`, `app.go`: the Wails application and the methods the page calls.
- `internal/files`: listing, search, and file details.
- `internal/winshell`: the Windows shell: icons, thumbnails, the context menu,
  file operations, the clipboard, and drag and drop.
- `internal/state`: the saved tabs, places, layout, and settings.
- `internal/terminals`: finding terminals and opening a folder in one.
- `internal/update`: the check for a new release.
- `frontend/src`: the page, in Svelte 5 and TypeScript.
- `frontend/wailsjs`: bindings that Wails generates from `app.go`. After changing
  a method the page calls, run `wails generate module`.
