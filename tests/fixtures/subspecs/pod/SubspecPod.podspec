Pod::Spec.new do |s|
  s.name         = "SubspecPod"
  s.version      = "0.1.0"
  s.summary      = "Fixture: a pod with two subspecs, to verify only the declared one is installed."
  s.homepage     = "https://github.com/zorvok-dev/zorvok"
  s.license      = "MIT"
  s.author       = { "Zorvok" => "dev@example.com" }
  s.source       = { :path => "." }
  s.ios.deployment_target = "13.0"

  s.subspec "Core" do |core|
    core.source_files = "Core/**/*.{h,m}"
  end

  s.subspec "Extra" do |extra|
    extra.source_files = "Extra/**/*.{h,m}"
  end
end
