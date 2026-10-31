Pod::Spec.new do |s|
  s.name         = "ResourceBundlePod"
  s.version      = "0.1.0"
  s.summary      = "Fixture: a pod that declares a resource bundle, to verify zorvok install produces a bundle target."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"
  s.source_files = "Classes/**/*.{h,m}"
  s.resource_bundles = {
    "ResourceBundlePod" => ["Resources/*.txt"]
  }
end
