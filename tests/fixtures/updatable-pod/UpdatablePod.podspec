Pod::Spec.new do |s|
  s.name         = "UpdatablePod"
  s.version      = "1.0.0"
  s.summary      = "Fixture: a branch-tracking git dependency, used to test `zorvok update` vs `zorvok install` (install must stay locked; only update moves forward)."
  s.homepage     = "https://github.com/zorvok-dev/zorvok-ios"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :git => "ZORVOK_UPDATABLE_POD_GIT_URL", :branch => "main" }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
end
