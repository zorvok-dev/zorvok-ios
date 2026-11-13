# Security policy

## Reporting a vulnerability

Please report security issues privately, not in a public issue or discussion:

1. Go to this repository's **Security** tab.
2. Click **Report a vulnerability** and describe the issue, how to reproduce it, and its impact.

We'll acknowledge your report, keep you updated while we fix it, and credit you in the release notes if you'd like. Please give us a reasonable chance to release a fix before disclosing the issue publicly.

## Supported versions

Security fixes go into the latest release. Upgrade with `brew upgrade zorvok`.

## Scope

- **The `zorvok` CLI** (this repository).
- **The public registry and its publish workflow** ([zorvok-specs](https://github.com/zorvok-dev/zorvok-specs)), for example a way to publish or change a pod you don't own, or to run code in the publish workflow.
- **The Homebrew formula** ([homebrew-tap](https://github.com/zorvok-dev/homebrew-tap)).

Vulnerabilities in CocoaPods itself, or in a pod's own code, should go to those projects.

## Security review

The most recent review, covering what was checked, how, and what was fixed, is in [SECURITY-REVIEW.md](SECURITY-REVIEW.md).
