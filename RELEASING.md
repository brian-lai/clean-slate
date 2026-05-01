# Releasing clean-slate

Releases are cut by tagging `main` with a semver version. A GitHub Actions
workflow (`.github/workflows/release.yml`) handles the rest: creating the
GitHub release and bumping the Homebrew formula in
[`brian-lai/homebrew-tap`](https://github.com/brian-lai/homebrew-tap).

## Cut a release

```sh
# From the tip of main
git checkout main
git pull

# Tag and push
git tag -a vX.Y.Z -m "vX.Y.Z: brief release note"
git push origin vX.Y.Z
```

That's it. Within ~2 minutes:

1. A GitHub release appears at
   `https://github.com/brian-lai/clean-slate/releases/tag/vX.Y.Z`
   with auto-generated notes from the commits since the previous tag.
2. The `clean-slate` formula in `brian-lai/homebrew-tap` gets a commit
   updating `url` and `sha256` to the new version.
3. `brew update && brew upgrade clean-slate` pulls the new version.

## What the workflow does

The `Release` workflow triggers only on tags matching `v[0-9]+.[0-9]+.[0-9]+`
(strict semver). Pre-release tags like `v0.1.0-rc1` are intentionally
excluded so experimental tags don't publish releases or bump the formula.

Two jobs:

- `release` — uses the built-in `GITHUB_TOKEN` (no setup) to call
  `gh release create --generate-notes --verify-tag`.
- `update-tap` — uses `dawidd6/action-homebrew-bump-formula@v4` with the
  `TAP_REPO_TOKEN` secret (a fine-grained PAT scoped to the tap repo) to
  commit the formula bump directly.

## Pre-requisites (one-time)

The `TAP_REPO_TOKEN` secret must be set on this repo. It's a fine-grained
Personal Access Token with:

- **Resource owner:** `brian-lai`
- **Repositories:** `brian-lai/homebrew-tap` only
- **Permissions:** `Contents: Read and write`, `Metadata: Read-only`

Rotate the token annually — the workflow will start failing with a 401
when it expires.

## If a release goes wrong

- **The tag was pushed but the workflow didn't fire.** Check the trigger
  glob matched (must be exactly `vX.Y.Z`, no suffix — pre-release tags
  like `v1.0.0-rc1` are intentionally excluded).
- **GitHub release was created but tap update failed.** Most common cause:
  `TAP_REPO_TOKEN` expired or is missing. Re-run the `update-tap` job after
  fixing the secret, or bump the formula manually with:
  ```sh
  brew bump-formula-pr \
    --url "https://github.com/brian-lai/clean-slate/archive/refs/tags/vX.Y.Z.tar.gz" \
    brian-lai/tap/clean-slate
  ```
- **Need to retract a release.** Delete the GitHub release (`gh release
  delete vX.Y.Z`), delete the tag locally and remotely (`git tag -d vX.Y.Z
  && git push --delete origin vX.Y.Z`), and revert the tap's formula
  commit. Downstream users who already installed are not affected until
  they `brew upgrade`.

## Versioning guidance

- Patch (`v0.1.0` → `v0.1.1`) — bug fixes, docs, no behavior change.
- Minor (`v0.1.x` → `v0.2.0`) — new features, additive only.
- Major (`v0.x.y` → `v1.0.0`) — breaking CLI changes.

Pre-1.0 the bar for minor bumps is looser; breaking changes can still land
in minor versions if called out clearly in the release notes.

## Changelog

Human-written one-liners per release. GitHub's auto-generated release notes
cover the full commit list; these are just the headline.

- **v0.1.4** — Show actionable errors on wrong arg counts (human + JSON); delete `ws/` branches when cleaning tasks so source repos don't accumulate abandoned branches.
- **v0.1.3** — First fully-sanitized public release.
- **v0.1.2** — Sanitized repo author metadata via history rewrite.
- **v0.1.1** — Release automation verified end-to-end.
- **v0.1.0** — Initial public release via `brew install brian-lai/tap/clean-slate`.
