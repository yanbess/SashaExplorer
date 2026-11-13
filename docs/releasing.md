# Releasing

Releases are built by GitHub Actions from a version tag. Nothing is built on a
personal computer, so every release can be traced to its commit.

## Versions

Versions follow [Semantic Versioning](https://semver.org): `MAJOR.MINOR.PATCH`.
Until 1.0.0, a new feature raises MINOR and a fix raises PATCH. A tag is the
version with a `v`, such as `v0.2.0`.

A tag with a suffix, such as `v0.3.0-beta.1`, makes a pre-release. The update
check in the program skips pre-releases.

## Steps

1. On `main`, move the entries under **Unreleased** in `CHANGELOG.md` into a new
   section `## [0.2.0] - YYYY-MM-DD`, and set `productVersion` in `wails.json` to
   the same version. Commit: `Release 0.2.0`.
2. Tag and push:

   ```
   git tag v0.2.0
   git push origin main v0.2.0
   ```

3. The **Release** workflow tests the code, builds
   `SashaExplorer-0.2.0-windows-amd64.zip` with its `.sha256`, and publishes a
   GitHub release with the notes of that version from `CHANGELOG.md`.

If the workflow fails, fix the problem, delete the tag
(`git push --delete origin v0.2.0` and `git tag -d v0.2.0`), and tag again.

## Update check

Once a day, if the user allows it, the program asks
`https://api.github.com/repos/yanbess/SashaExplorer/releases/latest` for the
latest release and compares its tag with its own version. A newer one is shown
in a note with a link to its page. No token or personal data is sent; GitHub
allows 60 such requests an hour from one address.

## Signing

The executable is not signed. Windows SmartScreen warns about an unsigned
program from the internet until it has a reputation. Signing with a code
signing certificate, for example through Azure Trusted Signing, would end that
warning; the workflow is the place to add it.
