#!/usr/bin/env ruby
# Regenerates tests/fixtures/simple-project/SimpleApp.xcodeproj.
#
# `pod install` requires a real Xcode project to integrate into — it
# won't synthesize one. This script creates the minimal valid project
# (via the `xcodeproj` gem, a CocoaPods dependency) so the fixture can be
# regenerated without Xcode.app itself. The generated project.pbxproj is
# checked into the repo; re-run this only if the fixture needs to change.
#
# Usage: ruby tests/fixtures/generate_simple_project.rb

require "xcodeproj"

project_path = File.join(__dir__, "simple-project", "SimpleApp.xcodeproj")
project = Xcodeproj::Project.new(project_path)

target = project.new_target(:application, "SimpleApp", :ios, "13.0")

group = project.main_group.new_group("SimpleApp")
main = group.new_file("main.m")
target.source_build_phase.add_file_reference(main)

project.save
puts "wrote #{project_path}"
