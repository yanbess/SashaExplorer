# Contributing

Thank you for helping. Bug reports, ideas, and pull requests are welcome.

## Reporting a problem

Open an [issue](https://github.com/yanbess/SashaExplorer/issues/new/choose) with:

- the version (Settings → Updates) and your Windows version;
- what you did, what you expected, and what happened instead;
- a screenshot if the problem is visible. Check it first for folder names or
  files you would rather not share.

## Changing the code

1. Read [docs/building.md](docs/building.md) and make sure `wails build` works.
2. Make the change on a branch, with tests where the code can be tested:
   `go test ./...` covers the backend.
3. Run `go vet ./...`, `gofmt -l .`, and `npx svelte-check` in `frontend`.
4. Add a line under **Unreleased** in [CHANGELOG.md](CHANGELOG.md).
5. Open a pull request that says what changes for the user and why.

Keep the program portable: one executable, data next to it, and no network
access apart from the optional update check.

## Style

- Go: `gofmt`, and comments that say what a thing is for.
- Frontend: Svelte 5 runes, TypeScript, and the color variables in
  `frontend/src/style.css`, so both themes work.
- Text in the interface is short, plain English.

By contributing, you agree that your work is released under the
[MIT License](LICENSE).
