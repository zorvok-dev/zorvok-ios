Pod::Spec.new do |s|
  s.name         = "PodB"
  s.version      = "0.1.0"
  s.summary      = "Fixture: depends on PodA — the Podfile only declares PodB, PodA must resolve transitively."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
  s.dependency "PodA"
end
