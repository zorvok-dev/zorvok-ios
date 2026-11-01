# Zorvok's native installer shim (ADR 0002 Phase 3 / Milestone 6).
#
# Drives Pod::Installer directly — the same class the real `pod install`
# CLI command uses internally (see Pod::Command::Install#run in the
# `cocoapods` gem) — instead of shelling out to the `pod` executable.
# This is still "delegating to CocoaPods' real implementation," not a
# reimplementation: resolution still goes through Molinillo, Xcode
# integration still goes through xcodeproj, downloads still go through
# cocoapods-downloader. The only thing Zorvok does differently here is
# supply the Podfile object itself.
#
# That difference is the whole point: Pod::Config#podfile is normally
# loaded by reading the literal Podfile at disk (Podfile.from_file),
# which only ever sees `source` lines the user actually wrote — this is
# exactly why Milestone 5 (see ADR 0004) found no way to add a Zorvok
# registry source without writing to that file. Pod::Podfile.from_ruby
# accepts explicit `contents`, used in place of reading the path again,
# and Pod::Config#podfile has a public writer. So: Zorvok reads the
# user's real Podfile exactly once, itself, prepends any federated
# `source` lines in memory, and hands the augmented content to
# Pod::Podfile.from_ruby — the file on disk is never written to, and
# with no extra sources this produces byte-identical results to a plain
# `pod install` (verified in the Milestone 6 differential tests).
#
# Two correctness/security fixes layered on top of that base mechanism,
# both found by reading CocoaPods' own Analyzer/Source code before
# shipping (ADR 0007), not assumed:
#
# 1. Pod::Installer::Analyzer#sources only falls back to the implicit
#    default CDN (`https://cdn.cocoapods.org/`) when the Podfile's own
#    `source` lines are EMPTY. The instant Zorvok prepends its own
#    `source` line, that fallback is bypassed — silently dropping the
#    public CDN and breaking every ordinary CocoaPods pod in the same
#    Podfile. Fix: if the ORIGINAL (unaugmented) Podfile declared no
#    sources of its own, Zorvok explicitly re-adds the trunk CDN
#    alongside whatever it injects, preserving default behavior exactly.
#    If the original Podfile already declared its own sources, Zorvok
#    adds nothing extra beyond what was asked for via --source.
#
# 2. Confirmed empirically: when two sources both have a spec for the
#    same (pod, version), CocoaPods' own Specification::Set picks one by
#    *source declaration order* — printing a warning ("Found multiple
#    specifications for ...") but still exiting 0. That is not "never a
#    silent pick" (ADR 0003); it is a real, exploitable dependency-
#    confusion / package-takeover vector the instant more than one
#    source is in play, which federation inherently requires. Fix: a
#    preflight check, using the exact same Pod::Installer::Analyzer#sources
#    computation `install!` itself would use, that aborts loudly —
#    before anything is downloaded or installed — if any dependency
#    without an explicit `:source =>` pin resolves from more than one
#    source.
#
# ARGV[0]: absolute path to the project directory (containing Podfile)
# ARGV[1..]: extra spec source URLs to search, in addition to whatever
#            the real Podfile already declares (empty = behaves exactly
#            like delegated `pod install`)

require 'cocoapods'

# Overridable via ZORVOK_DEFAULT_SOURCE_URL so the Milestone 6 test
# suite can verify this fallback hermetically, against a local fixture,
# instead of depending on real network access to cdn.cocoapods.org.
TRUNK_CDN_URL = (ENV['ZORVOK_DEFAULT_SOURCE_URL'] || 'https://cdn.cocoapods.org/').freeze

def fail_with(message)
  warn "zorvok-native-install: #{message}"
  exit 1
end

project_dir = ARGV[0]
fail_with('missing project directory argument') if project_dir.nil? || project_dir.empty?
extra_sources = ARGV[1..] || []

config = Pod::Config.instance
config.installation_root = Pathname.new(project_dir)

podfile_path = config.podfile_path
fail_with("no Podfile found in #{project_dir}") unless podfile_path && podfile_path.exist?

begin
  contents = File.read(podfile_path)

  # Fix 1: preserve the implicit default CDN when the original Podfile
  # declared no sources of its own — see the file header.
  original_podfile = Pod::Podfile.from_ruby(podfile_path, contents)
  source_lines = extra_sources.dup
  source_lines << TRUNK_CDN_URL if original_podfile.sources.empty? && !extra_sources.empty?

  augmented = source_lines.map { |url| "source #{url.inspect}\n" }.join + contents
  augmented_podfile = Pod::Podfile.from_ruby(podfile_path, augmented)
  config.podfile = augmented_podfile

  # Fix 2: refuse to silently disambiguate a pod that resolves from more
  # than one source. Uses the real Analyzer#sources computation — the
  # exact same one `installer.install!` is about to use internally — so
  # this can never drift from what's actually about to be installed.
  if augmented_podfile.sources.size > 1
    analyzer = Pod::Installer::Analyzer.new(config.sandbox, augmented_podfile, config.lockfile)
    sources = analyzer.sources

    # Podfile#dependencies (public, cocoapods-core) rather than
    # Analyzer#podfile_dependencies (private) — same root dependencies,
    # a stable public API.
    ambiguous = augmented_podfile.dependencies.each_with_object({}) do |dep, found|
      next if dep.podspec_repo # explicit `:source =>` pin on this pod: never ambiguous, it's pinned

      matches = sources.select { |s| s.search(dep) }
      found[dep.root_name] = matches if matches.size > 1
    end

    unless ambiguous.empty?
      details = ambiguous.map do |name, matches|
        "  #{name}: found in #{matches.map(&:url).join(', ')}"
      end.join("\n")
      fail_with(
        "refusing to install — the following pods resolve from more than one " \
        "source and have no explicit `:source =>` pin in the Podfile:\n#{details}\n" \
        'Pin each one explicitly (e.g. `pod "Name", :source => "https://..."`) to proceed.',
      )
    end
  end

  installer = Pod::Installer.new(config.sandbox, config.podfile, config.lockfile)
  installer.repo_update = false
  installer.update = false
  installer.install!
rescue Pod::Informative => e
  fail_with(e.message)
rescue StandardError => e
  fail_with("#{e.class}: #{e.message}")
end
