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
# ARGV[0]: absolute path to the project directory (containing Podfile)
# ARGV[1..]: extra spec source URLs to search, in addition to whatever
#            the real Podfile already declares (empty = behaves exactly
#            like delegated `pod install`)

require 'cocoapods'

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
  augmented = extra_sources.map { |url| "source '#{url}'\n" }.join + contents
  config.podfile = Pod::Podfile.from_ruby(podfile_path, augmented)

  installer = Pod::Installer.new(config.sandbox, config.podfile, config.lockfile)
  installer.repo_update = false
  installer.update = false
  installer.install!
rescue Pod::Informative => e
  fail_with(e.message)
rescue StandardError => e
  fail_with("#{e.class}: #{e.message}")
end
