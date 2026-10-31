Pod::Spec.new do |s|
  s.name         = "PodA"
  s.version      = "0.1.0"
  s.summary      = "Fixture: the base of a transitive dependency chain (B depends on A)."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
end
