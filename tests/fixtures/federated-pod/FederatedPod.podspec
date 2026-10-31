Pod::Spec.new do |s|
  s.name         = "FederatedPod"
  s.version      = "1.0.0"
  s.summary      = "Fixture: a pod only resolvable via a federated source injected by Zorvok's native installer (Milestone 6), never declared in the consuming project's Podfile."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
end
