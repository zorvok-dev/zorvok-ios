# ADR 0001: CLI implemented in Go, not Ruby or Swift

## Status
Proposed (Milestone 0)

## Context
Zorvok needs a CLI distributed via Homebrew, invoked frequently (including in CI), that must integrate with CocoaPods' existing Ruby implementation without becoming a fork of CocoaPods.

## Decision
Build the CLI in Go. Treat the CocoaPods Ruby toolchain as an external subprocess dependency, integrated via `pod ipc` (JSON over stdout) and direct subprocess calls to `pod install`/`pod update`, never as a linked library.

## Alternatives considered
- **Ruby**: natural fit for calling CocoaPods-Core directly as a library, but inherits CocoaPods' own biggest UX liability — a Ruby/gem toolchain as a distribution and version-management problem — and works against the goal of an independent, Homebrew-friendly binary.
- **Swift**: idiomatic for iOS tooling, but weaker cross-compilation/static-binary story and a worse fit for the registry-server backend, which won't run on macOS.

## Consequences
- Single static, cross-compiled (arm64 + x86_64) binary; trivial Homebrew formula.
- Requires an explicit, well-tested IPC boundary (`engines/cocoapods/adapter`) between Go and Ruby — this is real integration work, not avoided by the language choice, only isolated to one component.
- `zorvok doctor` must detect/diagnose the Ruby + CocoaPods runtime dependency during Phases 1-2.

See [ARCHITECTURE.md](../../ARCHITECTURE.md) §2.1-2.2.
