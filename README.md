# omnissiah

The Omnissiah (pronounced _omni-sigh-ah_) is a thin, opinionated wrapper around
[Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) for praying to machine spirits (a.k.a.
sandboxing AI agents).

> [!WARNING]
> This tool is deliberately playful with sci-fi references in command names. The author takes no
> responsibility for any feelings of cringe by users.

**Contents:**

- [Installation](#installation)
- [Usage](#usage)
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
3. (Optional) Set `omni` alias for Omnissiah:
   ```shell
   alias omni='omnissiah'
   ```

## Usage

Output of `omnissiah --help`:

```
USAGE:
   omnissiah [global options] [command [command options]]

COMMANDS:
   pray, p  Start a new Omnissiah session
   help, h  Shows a list of commands or help for one command
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
- Run `omnissiah --help` and update [Usage](#usage) if any new commands/options have been added
- Create commit and tag for the release (update `TAG` variable in below command):
  ```
  TAG=vX.Y.Z && git commit -m "Release ${TAG}" && git tag -a "${TAG}" -m "Release ${TAG}" && git log --oneline -2
  ```
- Push the commit and tag:
  ```
  git push && git push --tags
  ```
    - Our release workflow will then create a GitHub release with the pushed tag's changelog entry
