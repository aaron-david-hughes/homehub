# Releasing

HomeHub has **one version for everything** (for example `v1.2.0`). It is decided from the
conventional-commit PR titles that were squash-merged, using
[release-please](https://github.com/googleapis/release-please).

## How a release happens

1. PRs merge to `main`. For each one, images of the services that changed are published as **`:main`**
   (a single moving tag per service: the newest unreleased build of `main`).
2. release-please keeps one standing pull request open, titled `chore: release X.Y.Z`. It contains
   the next version, the `CHANGELOG.md` entry, `version.txt`, and the Helm chart versions.
3. **When you want a release, merge that PR.** release-please then creates the `vX.Y.Z` git tag and a
   GitHub Release whose notes are grouped by commit type (Features, Bug Fixes, ...) with breaking
   changes called out.
4. In the same run, **every** service's image is published with the tags `X.Y.Z`, `X.Y`, `X` and
   `latest`. A version therefore always refers to a complete, consistent set of images.

The version is chosen automatically: `feat` bumps the minor version, `fix` the patch, and a
breaking change (`!` or a `BREAKING CHANGE:` line) the major. If nothing releasable has merged
(only `docs`, `ci`, `chore`, ...), no release PR is opened.

## Image tags

| Tag | Moves? | Use for |
|-----|--------|---------|
| `1.2.0` | never | exact, repeatable deploys and rollbacks |
| `1.2` | on each 1.2.x release | follow fixes within a minor version |
| `1` | on each 1.x release | follow all non-breaking updates |
| `latest` | on each release | newest release |
| `main` | on each merge to main | testing unreleased changes |

The Helm chart pins the version it shipped with (`appVersion`), so `make deploy` gives a repeatable
deploy. To test unreleased code: `make deploy SERVICE=portal HELM_ARGS="--set image.tag=main --set image.pullPolicy=Always"`.
Before the first release there is no versioned image yet, so use `main`.

The first release is `1.0.0` (set by `initial-version` in `release-please-config.json`). Check the
version shown in the first release PR before merging it. If it is wrong, put a `Release-As: 1.0.0`
line in the footer of a commit message (in a PR description) and release-please will use it.

## One-time setup

1. **Personal access token for release-please.** Pull requests and tags created with the built-in
   `GITHUB_TOKEN` do not trigger other workflows, so the release PR would never run its required
   checks and could not be merged. In GitHub: Settings, Developer settings, Personal access tokens,
   Fine-grained tokens, Generate new token. Resource owner: yourself. Repository access: only
   `homehub`. Permissions: **Contents** read and write, **Pull requests** read and write. Choose an
   expiry (fine-grained tokens expire; put a reminder in your calendar).
2. Add it to the repo: Settings, Secrets and variables, Actions, New repository secret, named
   **`RELEASE_PLEASE_TOKEN`**. Until it exists the `cd` workflow's release job fails, and nothing
   is published.
3. **Branch ruleset for `main`:** require a pull request, require squash merges (with "Default to PR
   title and description" as the commit message), and require these status checks:
   `Validate PR title` and `ci-ok`. (`ci-ok` is the single gate in `ci.yaml`; the per-service
   `image (...)` jobs only exist when a service changed, so they cannot be required themselves.)
4. **Package visibility:** after a service's first image is published, set its package to Public
   (profile, Packages, `homehub/<service>`, Package settings) so the cluster can pull it. This is
   once per service.

## If a release's image build fails

The git tag and GitHub Release already exist at that point. Open the failed `cd` run and use
"Re-run failed jobs". Fix problems in a new PR rather than moving the tag.
