# Zorvok security review, November 2026

**Result:** two rounds of review found 27 issues. Every High and Medium one is fixed and covered by a test. Scans of all four repositories found no known-vulnerable dependencies and no real secrets in any commit. The risks that remain are listed under [What's left](#whats-left) instead of being left out.

**What kind of review this is:** the Zorvok team reviewed its own code with standard open-source security scanners, plus an independent AI-assisted review that was given only the code and asked to break it. After the fixes, a second, fresh review attacked them and found more, which are fixed too. **This is not a paid third-party audit.** We publish it so you can check our work: every tool and test below can be re-run with the commands at the end.

Date: 2026-11-13 · Reviewed by: the Zorvok team · Report a vulnerability: [SECURITY.md](SECURITY.md)

## What was reviewed

| Repository | What it is | Reviewed at | Fixes in |
|---|---|---|---|
| [zorvok-ios](https://github.com/zorvok-dev/zorvok-ios) | The `zorvok` CLI (Go) | `247602d` | v0.1.4 |
| [zorvok-specs](https://github.com/zorvok-dev/zorvok-specs) | The free public pod registry and its publish workflow | `f175447` | `main` |
| [homebrew-tap](https://github.com/zorvok-dev/homebrew-tap) | The Homebrew formula and release script | `d99e564` | `main` |
| zorvok-registry-ios | The self-hostable registry server (Go, not public yet) | `2d8c235` | `main` |

No pull request had ever been opened on the public registry when this review was done, so none of the issues below was ever exploitable in practice: no pod had been published through the old workflow.

**Threat model.** Most of the attention went to the public registry, because it is the one place where strangers can act. Anyone can open a pull request, and a GitHub Actions workflow holding the registry's write token decides on its own whether to publish it. The questions were:

1. Can a pull request run code in that workflow, or steal its token?
2. Can anyone change or delete a version that's already published?
3. Can anyone publish a new version of a pod they don't own, or take over a pod's ownership?
4. Can anyone claim a pod name that already exists on CocoaPods trunk without being one of its owners there?
5. Can a publish trick someone who installs from the registry into installing the wrong pod, or sneak anything else into the registry?
6. Can the CLI leak a token, or be driven by what a server sends back?

## How it was checked

| Tool | Version | Checks |
|---|---|---|
| [govulncheck](https://go.dev/blog/vuln) | 1.9.0 | Go dependencies and standard library against the Go vulnerability database, only reporting vulnerable code that is actually called |
| [gosec](https://github.com/securego/gosec) | 2.30.0 | Go code for injection, unsafe file and process handling, weak crypto |
| [gitleaks](https://github.com/gitleaks/gitleaks) | 8.30.1 | Every commit in all four repositories, plus uncommitted files, for keys and tokens |
| [shellcheck](https://www.shellcheck.net/) | 0.11.0 | The registry's shell scripts and the release script |
| [actionlint](https://github.com/rhysd/actionlint) | 1.7.12 | GitHub Actions workflows, including shellcheck on every `run:` step |
| [zizmor](https://github.com/zizmorcore/zizmor) | 1.32.0 | GitHub Actions security: dangerous triggers, injection, permissions, credential handling (auditor mode, all findings shown) |
| Manual and AI-assisted review, two rounds | | The publish workflow, every script it runs, and the CLI's publishing and login code, read line by line against the threat model above. Suspected bypasses were confirmed with local experiments before being counted. |

Go 1.27.4 on macOS. The registry's scripts were tested on bash 5.3, the shell GitHub's runners use, and on macOS's bash 3.2.

## Scan results

| Scan | zorvok-ios | zorvok-specs | homebrew-tap | zorvok-registry-ios |
|---|---|---|---|---|
| govulncheck | No vulnerabilities | n/a | n/a | No vulnerabilities |
| gitleaks, full history | No real secrets: 2 fake test tokens (below) | No leaks | No leaks | No leaks |
| gitleaks, working tree | No leaks | No leaks | No leaks | No leaks |
| shellcheck | n/a | Clean | Clean | n/a |
| actionlint | n/a | Clean | n/a | n/a |
| zizmor | n/a | No findings (3 deliberate, explained below) | n/a | n/a |
| gosec | 25 reports, none exploitable (below) | n/a | n/a | 3 reports, 2 fixed (below) |

Two lines in the CLI's tests hold the fake token `abcdef123456` (a fixture for token handling), which gitleaks's generic key rule flags. They're marked `gitleaks:allow` in the current code, and the two older commits that had them unmarked are listed in `.gitleaksignore` with the reason, so `gitleaks git .` passes cleanly.

**gosec, zorvok-ios (25).** Each was read and none is exploitable.
- 6 × G204, a subprocess run with variable arguments. The CLI's job is running `pod`, `ruby` and `gh`. It runs them with argument lists, never through a shell, so arguments can't be split or reinterpreted.
- 4 × G304 and 3 × G703, a file path from a variable. The paths are Zorvok's own config and credentials files, and the `pod` launcher found on your `PATH`. These are the user's own files, not attacker input.
- 1 × G302. The config directory is set to `0700`. A directory needs the execute bit to be usable, and `0700` already locks out other users.
- 1 × G306, a test helper writing a fixture file.
- 10 × G104, an unchecked error. These are writes to the terminal, best-effort cleanup on a path that's already failing, and test servers.

**gosec, zorvok-registry-ios (3).**
- A request path written to the log could forge log lines. **Fixed:** it's now logged quoted (`%q`). gosec's data-flow check still reports the line.
- A podspec served back could be read by a browser as HTML (G705). It's sent as JSON. **Hardened:** it's also sent with `X-Content-Type-Options: nosniff`, so browsers won't guess another type.
- An unchecked error writing a JSON response to a client that has gone away (G104). Nothing can be done about it.

**zizmor's three deliberate exceptions.** Each one is explained in a comment beside it in the workflow.
- `pull_request_target` is the only way for a pull request from a fork to get a token that can publish. That is exactly why nothing from the pull request ever runs (finding 8 and the workflow's Security notes).
- Both workflows keep the checkout's token, because they `git push` with it. Neither uploads artifacts, and the publish job also holds the same token as `GH_TOKEN` for `gh`.

## Round 1: findings and fixes

Severity reflects what an attacker could do to the registry or its users.

| # | Severity | Finding | Status |
|---|---|---|---|
| 1 | **High** | A removed owner could still publish, from a branch made before the removal | Fixed |
| 2 | **High** | A pod already on CocoaPods trunk could be claimed by taking over its old domain, organization, or repository name | Fixed (round 2 completed the fix) |
| 3 | Medium | Owners were listed by GitHub username, which can be renamed and re-registered by someone else | Fixed (v0.1.4) |
| 4 | Medium | A case variant of a trunk pod's name (`alamofire`) could be claimed by anyone | Fixed |
| 5 | Medium | Docs suggested adding the registry as a global Podfile `source`, which allows dependency confusion | Fixed (docs) |
| 6 | Low | Pod names and versions weren't restricted to a safe character set | Fixed |
| 7 | Low | Renames, symlinks, executables and multi-pod pull requests weren't refused | Fixed |
| 8 | Low | Text from a pull request was echoed into the verdict comment and the user's terminal unfiltered | Fixed |
| 9 | Low | `git` and `curl` calls with attacker-influenced URLs weren't fully hardened | Fixed |
| 10 | Low | Supply chain: unpinned action, a release script that could hash an error page or tag unpushed code, broad workflow permissions | Fixed; two recommendations open |
| 11 | Low | The CLI would send a GitHub token or registry session over plain `http` if configured to | Fixed (v0.1.4) |
| 12 | Low | Nothing limits how often someone can open publish pull requests | Open (see What's left) |

### 1. Removed owners could still publish (High)

**Problem.** The workflow checked whether the pull request's author was in the pod's `OWNERS` file as of the commit the pull request was based on. The author chooses that commit. So after an owner was removed, they could branch from the commit before their removal and publish a new version.

**Fix.** The workflow now merges the pull request onto the registry's current tip itself, with `git merge-tree`, which never touches the working tree. It checks the result against the tip and publishes it with a plain `git push`, which only succeeds as a fast-forward. If anything else landed in the meantime, the push fails and the checks run again on the new tip.

**Test.** `test-publish-workflow.sh` runs this attack: the removed owner's pull request is rejected and nothing of it reaches `main`.

### 2. Taking over trunk pods through lapsed names (High)

**Problem.** Several ways of proving you own a trunk pod could be passed by someone who wasn't an owner:
- serving a file from the pod's homepage, which anyone can do after registering a lapsed domain;
- being a public member of the organization that owns the repository;
- owning a repository at the podspec's URL, which can be re-created after a rename or deletion.

**Fix.** Round 1 removed the homepage and organization proofs, and added age checks to the repository-based ones. Round 2 then showed the age checks could be beaten (R1 below), so **the repository-based proofs are gone too**. A trunk pod can now be claimed only by proving control of one of its trunk owner emails, the same thing CocoaPods trunk itself relies on, through what GitHub has verified about the account:
- the email is the account's public GitHub email, which GitHub only allows for a verified address; or
- GitHub credits the account with commits to the pod's repository made with that email. GitHub credits a commit only to the account that verified its email, so commits forged with someone else's email are credited to them, not to the forger.

**Test.** `test-publish-policy.sh` has 25 cases. They cover each remaining proof, and show each removed one is now refused: repository owner, previous repository owner, re-registered username, organization member, a file in the repository, a file on the homepage.

### 3. Owners by username (Medium)

**Problem.** `OWNERS` listed GitHub usernames. If an owner renames their account, anyone can register the old name and would then be treated as an owner.

**Fix.**
- Each `OWNERS` line is now `<numeric GitHub user ID> <username>`, and only the ID counts. GitHub never changes or reuses an ID.
- The workflow takes the author's ID from GitHub's event, not from anything in the pull request.
- `zorvok trunk push` (v0.1.4) writes the new format.
- The workflow rejects old-format lines with a message saying how to upgrade.

### 4. Case variants of trunk names (Medium)

**Problem.** `alamofire` isn't `Alamofire` to git, so it passed the "already on trunk?" check and anyone could claim it. On case-insensitive disks (the macOS default), it could then shadow the real pod.

**Fix.**
- A name that matches a trunk pod's name case-insensitively is refused, checked against trunk's complete name list.
- A new pod whose name differs from one already in this registry only in case is refused too.

### 5. Dependency confusion through a global `source` (Medium, docs)

**Problem.** The registry's README suggested adding it as a `source` line in a Podfile. With several sources, CocoaPods merges every source's versions of a pod and picks the highest. We checked this in CocoaPods 1.16.2's `Specification::Set#versions`. Anyone can publish a name here that isn't on trunk. So for a company whose private spec repo has a pod that isn't on trunk either, an attacker could publish a higher version of that name here, and a Podfile listing both sources would install it.

**Fix.**
- The docs now pin each pod with `:source =>`, which makes CocoaPods use exactly one source for it, and explain why.
- `zorvok install --native` already refuses to install any pod found in more than one source unless it's pinned.

### 6–9. Smaller hardening (Low)

- **Names and versions** must match a safe character set. A pod named `-n` no longer confuses `echo`, and names can't hide path or option tricks.
- **One pod per pull request, plain files only.** Every file must have mode `100644`, so symlinks, submodules and executables are refused. Diffs are read with renames off, so moving a published version shows up as a deletion and is refused.
- **Untrusted text.** The verdict comment quotes its reasons as an indented code block with control characters removed, so nothing in it renders as a link or `@mention`. The CLI strips control and bidirectional-text characters from anything a server sends before printing it, so it can't send terminal escape codes or display differently from what it says.
- **git and curl.** The registry's scripts no longer clone anything. curl is restricted to https, including on redirects (`--proto`, `--proto-redir`), with `--globoff`, a timeout and a size limit.

### 10. Supply chain (Low)

- `actions/checkout` is pinned to a full commit SHA (v4.4.0), so a moved tag can't change what runs with the registry's token. Dependabot keeps the pin current.
- Each workflow job now asks for only the permissions it uses, with none at the workflow level, and each permission says why it's needed.
- The release script refuses to tag uncommitted or unpushed code, and fails on any HTTP error instead of hashing an error page into the formula.

Still recommended (see What's left): signed release tags, and release archives built by CI.

### 11. Tokens over plain http (Low)

**Problem.** If `registry_url` or `ZORVOK_GITHUB_API_URL` were set to an `http://` address, the CLI would send the session or GitHub token unencrypted.

**Fix.** v0.1.4 refuses with a clear error unless the URL is `https`, or `http` to this machine (`localhost`, `127.0.0.0/8`, `::1`) for local development. Look-alikes such as `http://localhost.example.com` are refused too. Round 2 closed the remaining gap through redirects (R9).

## Round 2: re-review of the fixes

A fresh reviewer that hadn't seen round 1's conclusions attacked the fixed code. It confirmed most bypasses with local experiments; the rest are marked "plausible". It confirmed these properties hold:
- nothing from a pull request runs in the workflow;
- published versions can't change;
- the stale-branch attack (finding 1) is closed;
- the CLI only trusts the workflow bot's comment about the exact commit it pushed.

| # | Severity | Finding | Status |
|---|---|---|---|
| R1 | **High** | The repository age checks could be beaten. After an owner renamed or deleted their account, an attacker could rename an *old* account and an *old* repository of theirs into the freed names, and both would pass. | Fixed: repository-based proofs removed (finding 2) |
| R2 | Medium | `.zorvok-owners` also matched usernames, and never checked the publisher's account age | Fixed: proof removed |
| R3 | Medium | A pull request's whole history was added to the registry, since its head became a parent of the published commit. Anything hidden in earlier commits (a large or illegal file added and later removed, a forged author) would have become permanent, and could get the registry taken down. | Fixed: the published commit's only parent is the tip, so only checked files enter |
| R4 | Medium | Anyone who takes over a trunk owner's email domain could verify that email on GitHub and pass the email proofs | Accepted: CocoaPods trunk trusts email the same way (What's left) |
| R5 | Low | "Latest version" was chosen by version sort, which ranks an old `2.0.0-beta.1` above `2.0.0` and could point at a repository the pod has left | Fixed: chosen by trunk's own publish dates |
| R6 | Low | A podspec file holding two JSON documents was checked on the second only, which could hide `prepare_command` in the first | Fixed: each podspec must be exactly one JSON object |
| R7 | Low | Carriage returns survived the comment filter and could end the quoted block, letting a pull request inject links or text into the bot's comment | Fixed: carriage returns removed too |
| R8 | Low | The CLI treated a pull request GitHub marked "merged" as published, even with no verdict | Fixed: only the workflow's verdict counts |
| R9 | Low | An https server redirecting to plain http on the same host would have received the token unencrypted (Go resends `Authorization` on same-host redirects) | Fixed: the CLI refuses redirects off https |
| R10 | Low | A push to a pull request by someone other than its author (a collaborator on the fork) was judged as the author's | Fixed: rejected unless the author made it |
| R11 | Low | An empty or truncated trunk pod list would have let anyone claim a trunk name | Fixed: a short list counts as trunk unreachable |
| R12 | Low | The workflow ran for pull requests into any branch, not only `main` | Fixed |
| R13 | Low | The CLI printed a registry server's error text unfiltered | Fixed: filtered like the verdict |
| R14 | Low–Medium | Pull requests are public before they land, so a brand-new name can be raced. Mass pull requests can also use up the workflow's GitHub API quota, which makes ownership checks fail. | Open (What's left) |
| R15 | Low | An owner can publish two spellings of one version of their own pod (`1.0` and `1.0.0`) | Accepted: only affects the owner's own pod |

**Tests.** R3, R7, R10 and the stale-branch attack run end to end in `test-publish-workflow.sh`:
- R3: a pull request hides a file in an earlier commit and removes it. It's published, and that file is not reachable anywhere in the registry's history.
- R7: an `OWNERS` line full of carriage returns. We confirmed the raw rejection text contains a carriage return and the published comment doesn't.

R5, R6 and R11 are cases in `test-publish-policy.sh`, and R6 is also in `test-validate-pr.sh`. R8, R9 and R13 are Go tests in `internal/publicregistry` and `internal/safehttp`.

## Verification

| Suite | Result |
|---|---|
| zorvok-ios `go test -race ./...` | All packages pass |
| zorvok-registry-ios `go test -race ./...` | All packages pass |
| zorvok-specs `scripts/test-validate-pr.sh` | 12 / 12 (bash 5.3 and 3.2) |
| zorvok-specs `scripts/test-publish-policy.sh` | 25 / 25 (bash 5.3 and 3.2) |
| zorvok-specs `scripts/test-publish-workflow.sh` | 9 / 9 (bash 5.3 and 3.2), live CocoaPods trunk and GitHub data |

`test-publish-workflow.sh` runs the publish workflow's own script, taken from the YAML, on nine simulated pull requests against a real git repository:
1. a new pod;
2. an owner removing a co-owner;
3. the removed co-owner's stale-branch attack;
4. a stranger claiming Kingfisher;
5. Kingfisher's real trunk owner claiming it;
6. the removed co-owner re-adding themselves;
7. hidden history;
8. a push by someone other than the author;
9. carriage returns.

For every pull request it checks the verdict and its reason. It also checks that `main` has the pull request's files exactly when it was published, and that none of the pull request's commits ever reach `main`.

## What's left

- **The workflow hasn't run on GitHub yet.** Its script has run end to end locally, but not on GitHub's runners. The first real publish should be watched in the Actions tab.
- **Email is the root of trunk ownership (R4).** If a trunk owner's email domain lapses and someone re-registers it, they can verify that address on GitHub and pass the email proofs. CocoaPods trunk itself trusts email the same way. The same goes for a compromise of GitHub, CocoaPods trunk, or an owner's account.
- **Racing new names, and spam (finding 12, R14).** A brand-new name (one not on trunk) belongs to whoever publishes it first, as on trunk. Here a pending pull request is public, so someone could race it. Nothing limits how many publish pull requests one account opens. Each is judged on its own and can only affect one pod, so the worst case is noise, wasted Actions minutes, and ownership checks failing closed (rejected with "run it again") until GitHub's API quota resets. GitHub's own abuse limits apply.
- **A pod's code is not reviewed.** Like CocoaPods trunk, the registry checks who may publish, not what a pod's code does. Pin versions and review dependencies as you would with any package manager.
- **Release signing.** Release tags aren't signed yet, and the Homebrew formula builds from GitHub's tag archive, checked against a fixed sha256. Signed tags, and archives built by CI with build provenance, are the next steps.
- **No third-party audit yet.** Once Zorvok has users, we'll apply to programs that fund independent audits of open-source projects, such as OSTIF and the GitHub Secure Open Source Fund.

## Re-run it yourself

```bash
brew install govulncheck gosec gitleaks shellcheck actionlint zizmor

# Go repos: known vulnerabilities and static analysis
govulncheck ./...
gosec ./...
gitleaks git .            # every commit
gitleaks dir .            # working tree

# zorvok-specs: workflow audit and tests
actionlint .github/workflows/*.yml
zizmor --persona=auditor .github/workflows/
scripts/test-validate-pr.sh
scripts/test-publish-policy.sh
scripts/test-publish-workflow.sh   # needs network: uses live CocoaPods trunk data
```

## Reporting a vulnerability

Please don't open a public issue. Use **Security → Report a vulnerability** on the repository; [SECURITY.md](SECURITY.md) has the details and what's in scope.
