Pod::Spec.new do |s|
  s.name         = "PublishablePod"
  s.version      = "1.0.0"
  s.summary      = "Fixture: a podspec with a fetchable-looking git source, used to test `zorvok trunk push`."
  s.homepage     = "https://github.com/zorvok-dev/zorvok-ios"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  # pod lib lint validates against the local checkout; it never actually
  # fetches :source, so this URL doesn't need to resolve to anything —
  # confirmed empirically, keeping this fixture's tests hermetic.
  s.source       = { :git => "https://github.com/zorvok-dev/this-repo-does-not-exist.git", :tag => "1.0.0" }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
end
