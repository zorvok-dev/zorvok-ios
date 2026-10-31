Pod::Spec.new do |s|
  s.name         = "GitPod"
  s.version      = "1.0.0"
  s.summary      = "Fixture: a pod fetched via a git dependency rather than :path."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :git => "ZORVOK_GIT_REPO_URL", :tag => s.version.to_s }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
end
