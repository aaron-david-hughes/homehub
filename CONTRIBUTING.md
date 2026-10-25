# Contributing

## Pull requests

`main` is protected and changes arrive by pull request, squash-merged. The **PR title becomes the
commit message** and the **PR description becomes its body**, so the title must be a
[Conventional Commit](https://www.conventionalcommits.org/). The `Validate PR title` check enforces it.

```
type(scope): short description in lower case
```

| Type | Use for | Version bump | Appears in release notes |
|------|---------|--------------|--------------------------|
| `feat` | a new capability | minor (1.2.0 to 1.3.0) | Features |
| `fix` | a bug fix | patch (1.2.0 to 1.2.1) | Bug Fixes |
| `perf` | a performance improvement | none on its own | Performance |
| `refactor` | restructuring with no behaviour change | none on its own | Refactoring |
| `revert` | undoing an earlier change | none on its own | Reverts |
| `docs` | documentation only | none on its own | Documentation |
| `test`, `build`, `ci`, `chore` | tests, build system, pipelines, upkeep | none | hidden |

Types that don't bump the version still appear in the notes of the next release that does happen.

**Scopes** are optional. Allowed: `portal`, `shared`, `charts`, `infra`, `ci`, `deps`, `docs`.
Add a new service's name to `.github/workflows/pr-title.yaml` when you create it.

**Breaking changes** bump the major version (1.x to 2.0.0). Mark them either way:
- add `!` before the colon: `feat(portal)!: rename the registration endpoint`
- or put `BREAKING CHANGE: what broke and how to migrate` on its own line in the PR description.

Examples:

```
feat(portal): show service versions on the dashboard
fix(shared): retry registration with backoff
ci: pin the runner image
feat(shared)!: require uiPath when registering
```

## Releases

Releases are automatic: see [docs/RELEASING.md](docs/RELEASING.md).
