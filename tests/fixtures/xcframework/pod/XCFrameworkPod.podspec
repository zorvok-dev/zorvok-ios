Pod::Spec.new do |s|
  s.name         = "XCFrameworkPod"
  s.version      = "0.1.0"
  s.summary      = "Fixture: a pod distributed as a prebuilt XCFramework rather than source."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"
  s.vendored_frameworks = "ZorvokFixture.xcframework"
end
