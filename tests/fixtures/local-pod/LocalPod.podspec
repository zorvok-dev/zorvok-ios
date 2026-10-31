Pod::Spec.new do |s|
  s.name         = "LocalPod"
  s.version      = "0.1.0"
  s.summary      = "A trivial local pod used only by Zorvok's own compatibility test suite."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
end
