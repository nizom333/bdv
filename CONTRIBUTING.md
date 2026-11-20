# Contributing to bdv

Thanks for helping. Bug reports, ideas and pull requests are all welcome.

## Build and run

You need Go (the version is in `go.mod`) and, for manual checks, [beads](https://github.com/steveyegge/beads) (`bd`) in your `PATH`.

```sh
make build            # ./bdv with version info
./bdv --version
cd <a beads project> && /path/to/bdv
```

Please try changes on a throwaway project (`mkdir /tmp/bdv-try && cd /tmp/bdv-try && bd init`), not on a project you care about.

## Test

```sh
gofmt -l .            # must print nothing
go vet ./...
go test -race ./...
```

Tests never call the real `bd`: `internal/testbd` puts a fake `bd` script first in `PATH` and records every call. New behaviour should come with a test that drives the app the way a user would (see `internal/app/harness_test.go`). Things tests cannot cover (real terminal, themes, clipboard, mouse, real `bd`) are listed in [docs/manual-testing.md](docs/manual-testing.md); add a line there when you change one of them.

bdv only writes through `bd` in the few places listed in the README, each one confirmed by the user. Keep it that way: every other `bd` call must run with `--readonly`.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes. The release changelog is grouped by them:

- `feat:` new behaviour (Features)
- `fix:` bug fixes (Bug fixes)
- `docs:` documentation (Documentation)
- `chore:`, `ci:`, `build:`, `test:` for everything else

Mark breaking changes with `!` (`feat!: …`) or a `BREAKING CHANGE:` footer. Keep the subject short; the body explains why.

## Releases

Releases are cut by the maintainer from `main`:

1. CI is green on `main`.
2. An annotated tag `vX.Y.Z` (semver) is pushed.
3. `.github/workflows/release.yml` runs the tests and [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`), which publishes the GitHub Release (archives, deb/rpm/apk, `checksums.txt`) and updates the Homebrew cask and Scoop manifest in [nizom333/homebrew-tap](https://github.com/nizom333/homebrew-tap).
4. `.github/workflows/install-test.yml` installs the new version with Homebrew on macOS and Linux and runs `bdv --version`.

To try the release config locally without publishing anything:

```sh
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish
rm -rf dist
```
