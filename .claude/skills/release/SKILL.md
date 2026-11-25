---
name: release
description: Use when cutting, tagging, publishing or shipping a new bdv version, when someone asks to "release", "make a release", "bump the version", "push a tag", or when a bdv release, the Homebrew cask, the Scoop manifest or the install test needs checking or rolling back.
---

# Releasing bdv

## Overview

A bdv release is a pushed annotated tag `vX.Y.Z` on `main`. Everything else is automatic: `.github/workflows/release.yml` runs the tests and GoReleaser (`.goreleaser.yaml`), which publishes the GitHub Release and pushes the Homebrew cask and the Scoop manifest to `nizom333/homebrew-tap`; `.github/workflows/install-test.yml` then installs it with brew on macOS and Linux.

**A published version is immutable.** Tags reach the Go module proxy, Homebrew and Scoop users within minutes, and `proxy.golang.org` caches a version forever. Never force-push, never move or recreate a tag that was pushed. A broken release is fixed by the next patch version.

| Thing | Value |
|---|---|
| Repo | `nizom333/bdv`, branch `main` |
| Tap | `nizom333/homebrew-tap`: cask `Casks/bdv.rb`, Scoop `bucket/bdv.json` |
| Workflows | `ci.yml`, `release.yml` (on tag `v*`), `install-test.yml` (after Release) |
| Secrets | `TAP_DEPLOY_KEY` (SSH deploy key, write access to the tap), `AUR_KEY` (optional; AUR upload is skipped when empty), `WINGET_GITHUB_TOKEN` (optional classic PAT with `public_repo`; GoReleaser pushes manifests to the fork `nizom333/winget-pkgs` and opens a PR to `microsoft/winget-pkgs`; skipped when empty) |
| Assets | `bdv_X.Y.Z_{darwin,linux}_{amd64,arm64}.tar.gz`, `bdv_X.Y.Z_windows_{amd64,arm64}.zip`, `bdv_X.Y.Z_{amd64,arm64}.deb`, `bdv-X.Y.Z-1.{x86_64,aarch64}.rpm`, `bdv_X.Y.Z_{x86_64,aarch64}.apk`, `checksums.txt` |

Homebrew ships bdv as a **cask** (`Casks/bdv.rb`), not a formula: GoReleaser v2.16+ deprecates `brews`, and `goreleaser check` fails on it. Do not "fix" this by switching back to `brews`.

## 1. Preflight

Stop at the first failure and report it; do not tag around a red check.

```sh
cd "$(git rev-parse --show-toplevel)"
test "$(git branch --show-current)" = main || echo "STOP: not on main"
test -z "$(git status --porcelain)"        || echo "STOP: working tree not clean"
git fetch origin --tags --prune
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" || echo "STOP: main is not in sync with origin/main"

test -z "$(gofmt -l .)" || echo "STOP: gofmt"
go vet ./...
go test -race ./...
go run github.com/goreleaser/goreleaser/v2@latest check

gh run list --repo nizom333/bdv --workflow ci.yml --branch main --limit 1 \
  --json conclusion,headSha --jq '.[0]'   # conclusion must be "success" for HEAD
```

`origin/main` ahead or behind: pull or push the branch normally first (never `--force`), then start over.

## 2. Choose the version

```sh
last="$(git describe --tags --abbrev=0 2>/dev/null)"
echo "last tag: ${last:-none}"
git log --format='%h %s' ${last:+$last..}HEAD
git log --format='%B' ${last:+$last..}HEAD | grep -n 'BREAKING CHANGE' || true
```

Commit subjects are in Russian with English prefixes (`feat: …`, `fix: …`). Classify by the prefix only:

| Commits since last tag | Before 1.0 (`0.y.z`) | From 1.0 |
|---|---|---|
| any `type!:` or `BREAKING CHANGE:` | minor `0.(y+1).0` | major `(x+1).0.0` |
| any `feat:` | minor `0.(y+1).0` | minor |
| only `fix:` | patch | patch |
| only `docs:`, `chore:`, `ci:`, `build:`, `test:` | ask whether a release is wanted at all; if yes, patch | same |

No tag yet: the first release is `v0.1.0`. Never reuse a version that exists on GitHub, even a deleted one (`gh release list --repo nizom333/bdv`, `git ls-remote --tags origin`). State the chosen version and the reason (for example "feat commits since v0.1.0, pre-1.0 → v0.2.0") before tagging.

## 3. Tag and push

```sh
V=v0.2.0   # the version chosen above
git tag -a "$V" -m "bdv $V"
git push origin "$V"
```

Push only the tag. Never `git push --tags` (it pushes stray local tags) and never `--force`.

## 4. Monitor

```sh
sleep 10
RUN=$(gh run list --repo nizom333/bdv --workflow release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
gh run watch "$RUN" --repo nizom333/bdv --exit-status
```

On failure: `gh run view "$RUN" --repo nizom333/bdv --log-failed`. If it failed before anything was published (tests, build), see Rollback. If it failed after the GitHub Release was created (typically a tap push with a wrong `TAP_DEPLOY_KEY`), fix the cause and re-run the same run with `gh run rerun "$RUN" --repo nizom333/bdv --failed`; `release.mode: replace` lets GoReleaser reuse the release. Do not retag.

## 5. Verify

```sh
V=v0.2.0; N=${V#v}

# Release assets and checksums
gh release view "$V" --repo nizom333/bdv --json assets --jq '.assets[].name' | sort
d=$(mktemp -d)
gh release download "$V" --repo nizom333/bdv -D "$d" -p checksums.txt -p "bdv_${N}_darwin_arm64.tar.gz" -p "bdv_${N}_amd64.deb"
(cd "$d" && shasum -a 256 --ignore-missing -c checksums.txt)

# Tap: cask and Scoop manifest carry the new version
gh api repos/nizom333/homebrew-tap/contents/Casks/bdv.rb --jq .content | base64 -d | grep "version \"$N\""
gh api repos/nizom333/homebrew-tap/contents/bucket/bdv.json --jq .content | base64 -d | grep "\"version\": \"$N\""

# Install test (starts when Release finishes)
IT=$(gh run list --repo nizom333/bdv --workflow install-test.yml --limit 1 --json databaseId --jq '.[0].databaseId')
gh run watch "$IT" --repo nizom333/bdv --exit-status

# Go module proxy
curl -fsS "https://proxy.golang.org/github.com/nizom333/bdv/@v/$V.info"
```

Expect 6 archives, 2 deb, 2 rpm, 2 apk and `checksums.txt`. Report the release URL (`gh release view "$V" --repo nizom333/bdv --json url --jq .url`) and every check result.

## 6. Rollback

Use only when the release is broken. Rollback removes it from users' reach; the fix ships as the next patch version.

```sh
V=v0.2.0
gh release delete "$V" --repo nizom333/bdv --yes --cleanup-tag   # release + remote tag
git tag -d "$V"                                                  # local tag

# Revert the tap commits that GoReleaser pushed (cask, scoop), with a normal push
t=$(mktemp -d) && git clone git@github.com:nizom333/homebrew-tap.git "$t" && cd "$t"
git log --oneline -5 -- Casks/bdv.rb bucket/bdv.json   # commits named "bdv vX.Y.Z"
git revert --no-edit <cask-commit> <scoop-commit>
git push origin main
```

Then fix the problem on `main` and release `vX.Y.(Z+1)`. The deleted version number is burned (the Go proxy may already have it); never tag it again.

## Common mistakes

| Mistake | Fix |
|---|---|
| Tagging with a dirty tree or unpushed commits | Preflight; the tag must point at `origin/main` |
| Moving a tag after a bad release (`git tag -f`, `git push -f`) | Never. Roll back and release the next patch |
| Lightweight tag (`git tag v0.2.0`) | Always `git tag -a` |
| Looking for `Formula/bdv.rb` in the tap | bdv is a cask: `Casks/bdv.rb` |
| Treating a skipped AUR step as a failure | Expected while `AUR_KEY` is not set |
| Treating a skipped winget step as a failure | Expected while `WINGET_GITHUB_TOKEN` is not set; without it, update winget by hand: a PR with the three manifests under `manifests/n/nizom333/bdv/<version>/` |
| Bumping major for a breaking change before 1.0 | Pre-1.0 breaking changes bump the minor |
