#!/usr/bin/env ruby
# Regenerates the Xcode projects used by the CocoaPods compatibility
# fixtures under tests/fixtures/.
#
# `pod install` requires a real Xcode project to integrate into — it
# won't synthesize one. This script creates the minimal valid projects
# (via the `xcodeproj` gem, a CocoaPods dependency) so every fixture can
# be regenerated without Xcode.app itself. The generated project.pbxproj
# files are checked into the repo; re-run this only if a fixture needs to
# change.
#
# Usage: ruby tests/fixtures/generate_projects.rb

require "xcodeproj"

# [relative project dir, project filename (no extension), [target names]]
PROJECTS = [
  ["simple-project", "SimpleApp", ["SimpleApp"]],
  ["transitive/project", "TransitiveApp", ["TransitiveApp"]],
  ["transitive/project-missing-dependency", "TransitiveApp", ["TransitiveApp"]],
  ["subspecs/project", "SubspecApp", ["SubspecApp"]],
  ["resource-bundle/project", "ResourceBundleApp", ["ResourceBundleApp"]],
  ["multi-target/project", "MultiTargetApp", ["AppOne", "AppTwo"]],
  ["git-dependency/project", "GitApp", ["GitApp"]],
  ["xcframework/project", "XCFrameworkApp", ["XCFrameworkApp"]],
  ["federated-project", "FederatedApp", ["FederatedApp"]],
]

PROJECTS.each do |dir, name, targets|
  project_path = File.join(__dir__, dir, "#{name}.xcodeproj")
  project = Xcodeproj::Project.new(project_path)

  targets.each do |target_name|
    target = project.new_target(:application, target_name, :ios, "13.0")
    group = project.main_group.new_group(target_name)
    main = group.new_file("main.m")
    target.source_build_phase.add_file_reference(main)
  end

  project.save
  puts "wrote #{project_path}"
end
