# Contributing

Thanks for taking the time to contribute. This document describes how to set up your environment, the conventions this project follows, and what is expected from a pull request.

## Code of conduct

Be respectful, be constructive. Assume good intent. See [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

## Ways to contribute

- **Report a bug** — open a GitHub issue with reproduction steps, the event source (SNS topic, EventBridge rule, direct invoke), the JSON event passed to the Lambda (sanitized), expected vs. actual Slack output, and CloudWatch logs.
- **Suggest a new source parser** — open an issue first with a representative AWS event payload. Aligning on the rendered Slack message before code review keeps each parser consistent with the rest.
- **Send a pull request** — see [Pull requests](#pull-requests) below.
- **Improve docs** — `README.md`, `examples/`, and inline docs are fair game.

For security issues, do **not** open a public issue. See [`SECURITY.md`](SECURITY.md).

## Prerequisites

- **Go 1.26** — pinned in `go.mod` and CI. Match the local toolchain to avoid `go.mod` churn.
- **OpenTofu >= 1.11** (or Terraform >= 1.5) if you touch `examples/`.
- `make`, `zip`, and a POSIX shell for the build targets.

## Project layout

| Path | What lives there |
|---|---|
| `cmd/aws-to-slack/` | Lambda entry point (`bootstrap` for `provided.al2023`). |
| `internal/handler/` | Wires envelope → router → Slack sinks. |
| `internal/envelope/` | Lambda payload shape normalization (SNS, EventBridge, direct). |
| `internal/router/` | Ordered parser waterfall. |
| `internal/parser/<source>/` | Per-AWS-service parsers (one package per source). |
| `internal/slack/` | Webhook client and Block Kit message construction. |
| `internal/console/`, `internal/dedup/`, `internal/kms/`, `internal/config/` | Supporting packages. |
| `samples/` | Committed event fixtures used by parser tests — the canonical record of what each source emits. |
| `examples/lambda/` | OpenTofu / Terraform example wiring the binary into a Lambda function. |
| `vendor/` | Committed. Always build and test with `-mod=vendor`. |

## Development workflow

1. Fork the repository and create a topic branch from `main`.
2. Make focused changes — one concern per PR.
3. Run the checks listed below.
4. Open a pull request using the template.

### Build, test, lint

```sh
make test          # go test -mod=vendor -race -count=1 ./...
make vet           # go vet -mod=vendor ./...
make lint          # golangci-lint run
make fmt           # go fmt ./...
make tidy          # go mod tidy && go mod vendor
make package       # builds linux_amd64 and linux_arm64 bootstrap zips
```

For OpenTofu / Terraform changes:

```sh
tofu fmt -recursive -check examples/
tofu -chdir=examples/lambda init -backend=false
tofu -chdir=examples/lambda validate
```

### Vendoring

Dependencies live in `vendor/` and are committed. If you add or upgrade a module:

```sh
go get example.com/module@vX.Y.Z
go mod tidy
go mod vendor
```

Commit the `vendor/` changes alongside the `go.mod` / `go.sum` updates.

## Coding conventions

- **Adding a new event source** is the most common contribution. Each source lives in its own `internal/parser/<source>/` package, registers itself with the router, and ships with at least one committed sample under `samples/<source>/` plus a parser test that asserts the rendered Slack blocks.
- **Slack message shape is the user-visible contract.** Header, fields, AWS console link layout, and chart-image placement should stay consistent across sources. Cross-reference the existing parsers before introducing a new layout.
- The router is **ordered and short-circuiting**. A new parser must declare why it matches an event and must not steal events from a more specific parser registered before it.
- **Dedup is not optional for Inspector2.** Anything that emits many findings per scan must route through `internal/dedup/` or it will spam Slack.
- **KMS auto-detection is in `internal/kms/`.** Do not add bespoke decryption paths in parser code; use the existing helper so the `0x01 0x02` magic-bytes check stays in one place.
- Errors that come from AWS SDK calls are wrapped with the source name and the resource ID (alarm ARN, deployment ID, pipeline name, etc.) so failures are actionable from CloudWatch Logs.
- The `bootstrap` binary must stay CGO-free and statically linked so it runs on `provided.al2023` without extra dependencies.
- Comments explain **why**, not what. Self-documenting code with good names beats narration.
- Follow standard `gofmt` and the rules in `.golangci.yml`. CI runs `golangci-lint`.

## Tests

- Unit tests live next to the code (`*_test.go`).
- **Use committed fixtures.** Every parser test reads from `samples/<source>/...` rather than embedding JSON inline. New parsers must add at least one representative fixture.
- Cover both happy path and failure modes — malformed payloads must not panic; unknown sub-types must fall through to the generic parser.
- Tests must be deterministic and run under `-race`.
- Do not depend on real AWS or Slack calls; stub clients behind the existing interfaces. The Slack webhook client is mocked via `internal/slack/`'s `Doer` interface.

## Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

<optional body explaining the why>
```

Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `build`.

Examples from the repo's history:

- `fix: split Slack section.fields to satisfy 10-field limit`
- `feat: render CloudWatch AlarmDescription as a section block`
- `refactor: transport-neutral notifications with Slack as one renderer`
- `feat: chart URL TTL + optional SSE algorithm via env vars`

Keep the subject line under ~72 characters. Reference issues with `Fixes #N` or `Refs #N` in the body when relevant.

## Pull requests

A PR is ready for review when:

- [ ] `make test` passes locally with `-race`.
- [ ] `make vet` is clean.
- [ ] `golangci-lint run` is clean (or CI passes).
- [ ] OpenTofu examples pass `tofu fmt -check -recursive examples/` and `tofu validate`.
- [ ] `CHANGELOG.md` has an entry under `## [Unreleased]` if the change is user-visible.
- [ ] `README.md` / examples are updated when behavior changes.
- [ ] New parsers include at least one fixture under `samples/` and a parser test.
- [ ] Commits are focused and follow Conventional Commits.

CI runs the test matrix and the release workflow on tagged releases. PRs that change the env-var contract, the supported source set, or the release artifacts need maintainer sign-off before merge.

## Release process

Releases are cut by a maintainer:

1. Move the `## [Unreleased]` items in `CHANGELOG.md` into a new `## [vX.Y.Z] - YYYY-MM-DD` section in a PR; merge to `main`.
2. Create a GitHub Release with tag `vX.Y.Z`.
3. `release.yml` attaches `lambda-aws-to-slack_<version>_linux_<arch>.zip` and `.zip.sha256` for both `amd64` and `arm64`.

Pre-1.0 (current): minor bumps may include breaking changes to env vars or rendered message layout; document them prominently in the changelog.

## Hard constraints

- The `bootstrap` binary must stay CGO-free and statically linked.
- Cold-start KMS decryption of `SLACK_HOOK_URL` / `SLACK_CHANNEL` MUST fail the cold start on error, so the configured CloudWatch alarm on Lambda Errors triggers instead of silently dropping notifications.
- Inspector2 findings MUST flow through `internal/dedup/`. Skipping dedup is not an acceptable shortcut.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
