Pod::Spec.new do |s|
  s.name         = "WrapperPod"
  s.version      = "1.0.0"
  s.summary      = "Fixture: a local pod whose only dependency, SharedPod, is ambiguous."
  s.homepage     = "https://github.com/zorvok-dev/zorvok-ios"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
  s.dependency "SharedPod"
end
