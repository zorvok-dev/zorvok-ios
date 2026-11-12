# ADR 0010: `trunk push` publishes to the public registry, and the registry merges it itself

## Status
Accepted

## Context
After ADR 0009, `zorvok trunk push` only published to a live `zorvok-registry-ios` server, which nobody runs publicly. The free public registry, `zorvok-specs`, could only be published to by hand: convert the podspec with `pod ipc spec`, fork the repository, add `Specs/<name>/<version>/<name>.podspec.json` (plus `OWNERS` for a new pod), open a pull request, and wait for a maintainer to merge it.

That's far from what CocoaPods users expect. With CocoaPods, a pod's code, frameworks and xcframeworks stay wherever its podspec's `source` points; only the podspec goes to the registry, and `pod trunk push` puts it there in one command. trunk.cocoapods.org can do that because it's a server holding write access to the Specs repo: it checks the session, checks ownership in its database, and commits the podspec as a bot. CocoaPods trunk also becomes read-only on December 2, 2026, so "keep publishing new versions of an existing pod" is exactly what pod authors will come to Zorvok for.

The public registry deliberately has no server (ADR 0009), so something else has to hold write access and make that decision.

## Decision
GitHub plays trunk.cocoapods.org's part:

- **Identity is the publisher's GitHub account**, in place of trunk's email session: `gh auth login` once (or `GH_TOKEN`/`GITHUB_TOKEN`), the way `pod trunk register` is a one-time step. OWNERS files already list GitHub usernames (ADR 0009).
- **`zorvok trunk push` makes the pull request itself** (`internal/publicregistry`). It lints and converts the podspec exactly as before (`publisher.Prepare`), then checks the registry first: the version must be new, and an existing pod's OWNERS must list the publisher. It forks the registry if needed, commits the JSON (plus an OWNERS file listing the publisher, for a brand-new pod) as one commit on top of the registry's tip, moves a branch to it in one step, and opens the pull request, or reuses the one an earlier attempt opened. One commit and one branch move mean the registry's workflow runs once, on the finished change, and the branch never points at a commit the registry already has, which GitHub can treat as the pull request having been merged.
- **The registry merges valid publishes itself** (`zorvok-specs/.github/workflows/publish.yml`, on `pull_request_target`). It runs `validate-pr.sh` (immutability, structure, ownership; unchanged) and a new `publish-policy.sh` (is this a change a machine should merge on its own?). It merges with `--match-head-commit`, so exactly the judged commit lands, regenerates the index in the same job, and comments a verdict whose first line is machine-readable: `<!-- zorvok-publish sha=<commit> verdict=published|needs-review|rejected -->`.
- **`trunk push` waits for that verdict** — on its own commit, posted by `github-actions[bot]`, nothing else counts — and reports it the way `pod trunk push` would: `Published MyPod 1.0.0`, or why not.
- **Routing**: with no `registry_url`, or with `registry_url` set to any of the public registry's addresses, `trunk push` publishes there. A `registry_url` naming anything else is a registry server, published to as before, with `zorvok login`.

What still needs a person (`publish-policy.sh`), so the automation can't be turned against anyone:

- **A name that's already on CocoaPods trunk**, on its first Zorvok publish. Otherwise anyone could claim `Alamofire` here first. A maintainer confirms it once; that pod's later versions publish automatically, held to its OWNERS. Trunk is checked through the same sharded index files CocoaPods' own CDN source reads.
- **Anything other than adding podspecs** (plus a new pod's OWNERS): an OWNERS edit, a change to the registry's scripts or workflows, an OWNERS file with no version.
- **`prepare_command`**, which runs shell commands on every machine that installs the pod; CocoaPods trunk stopped accepting new podspecs that use it in May 2025.

## Consequences
- Publishing to the public registry is one command after a one-time `gh auth login`, and it usually finishes in about a minute.
- `trunk push` with no `registry_url` no longer targets the local development server (`http://localhost:8080`); developing against a local `zorvok-registry` now needs `registry_url` set explicitly. Every other registry command still falls back to that default.
- `pull_request_target` runs with the registry's write token, so the workflow never runs anything from a pull request. It checks out the base branch, copies its scripts aside first, fetches the pull request's commits only as data for `git show`/`git diff`/`jq`, and passes every value through `env` rather than interpolating it into the script.
- A merge made with a workflow's token doesn't trigger other workflows, so the publish workflow regenerates the index itself; `regenerate-index.yml` still covers merges a maintainer makes by hand. Both go through `push-index.sh`, which retries when they race.
- `trunk add-owner`/`remove-owner`/`deprecate` still only talk to a registry server. On the public registry, co-owners are added with a pull request editing OWNERS, which a maintainer reviews.
- The workflow YAML hasn't run on GitHub Actions yet. Its scripts are tested locally (`test-validate-pr.sh`, `test-publish-policy.sh`), and its script block was run end to end against a local git origin with a stand-in for `gh`; the first real publish is the remaining check.
