# How to contribute

Contributions are welcome to this repo, but we do have a few guidelines for
contributors.

## Open an issue and pull request for changes

All submissions, including those from project members, are required to go through
review. We use [GitHub Pull Requests] for this workflow, which should be linked
with an issue for tracking purposes. A GitHub action will be run against your PR
to ensure code standards have been applied.

* [pre-commit] is used to ensure that all files have consistent formatting and to
  avoid committing secrets.

* [golangci-lint] is used to enforce Go code passes linting and formatting rules
  ([gofumpt] is the expected Go code formatter). The rules are defined in [.golangci.yml](.golangci.yml).

1. Install [pre-commit] in a virtual python environment or globally: see [instructions](https://pre-commit.com/#installation)
2. Install [golangci-lint] from a binary or from source: see [instructions](https://golangci-lint.run/usage/install/#local-installation)
3. Fork and clone this repo
4. Install pre-commit hook to git

   E.g.

   ```shell
   pip install pre-commit
   pre-commit install
   ```

5. Create a new branch for changes
6. Execute tests and linter against `v2` packages to validate changes; please
   address any issues raised before submitting the PR.

   ```shell
   go test -short -v ./v2/...
   golangci-lint run ./v2/...
   ```

7. Open an issue to describe the bug or feature
8. Commit to branch and push changes to your fork

   The hook will ensure that `pre-commit` will be run against all staged changes
   during `git commit`.

9. Open a PR, referencing the issue opened in step 7

[GitHub Pull Requests]: https://help.github.com/articles/about-pull-requests/
[pre-commit]: https://pre-commit.com/
[gofumpt]: https://github.com/mvdan/gofumpt
[golangci-lint]: https://golangci-lint.run/
