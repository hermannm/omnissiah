# omnissiah

Omnissiah (pronounced _omni-sigh-ah_) is a thin, opinionated wrapper around
[Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) for sandboxing AI agents.

- [Installation](#installation)
- [Maintainer's guide](#maintainers-guide)

## Installation

1. [Install the Docker Sandboxes `sbx` CLI](https://docs.docker.com/ai/sandboxes/install)
    - On MacOS:
      ```
      brew trust docker/tap
      brew install docker/tap/sbx
      ```
2. Then, install `omnissiah`:
    - With [Go](https://go.dev/):
      ```
      go install hermannm.dev/omnissiah
      ```
    - With [Mise](https://mise.jdx.dev/), using the
      [Go backend](https://mise.jdx.dev/dev-tools/backends/go.html):
      ```
      mise use -g go:hermannm.dev/omnissiah
      ```

## Maintainer's guide

### Publishing a new release

- Run tests and linter ([`golangci-lint`](https://golangci-lint.run/)):
  ```
  go test ./... && golangci-lint run
  ```
- Add an entry to `CHANGELOG.md` (with the current date)
    - Remember to update the link section, and bump the version for the `[Unreleased]` link
- Bump `Command.Version` in `main.go`
- Create commit and tag for the release (update `TAG` variable in below command):
  ```
  TAG=vX.Y.Z && git commit -m "Release ${TAG}" && git tag -a "${TAG}" -m "Release ${TAG}" && git log --oneline -2
  ```
- Push the commit and tag:
  ```
  git push && git push --tags
  ```
    - Our release workflow will then create a GitHub release with the pushed tag's changelog entry
