# xcframework fixture

`ZorvokFixture.xcframework` is a minimal real XCFramework (one slice:
`ios-arm64-simulator`), built without needing a full Xcode project/scheme
— just `clang` and `xcodebuild -create-xcframework`. It exists only so
`pod install` can read its structure and wire `s.vendored_frameworks`
correctly; it is never compiled against or executed by the compatibility
test, so the committed binary's architecture doesn't need to match the
machine running the test.

## Regenerating

```bash
SDK=$(xcrun --sdk iphonesimulator --show-sdk-path)
ARCH=arm64   # or x86_64, matching your machine — doesn't affect test validity

mkdir -p build/ZorvokFixture.framework/Headers
cat > build/Fixture.m <<'EOF'
#import <Foundation/Foundation.h>
@interface ZorvokXCFrameworkFixture : NSObject
@end
@implementation ZorvokXCFrameworkFixture
@end
EOF
cp build/Fixture.m build/ZorvokFixture.framework/Headers/ZorvokFixture.h  # trim to just the @interface

clang -arch $ARCH -isysroot "$SDK" -mios-simulator-version-min=13.0 \
  -dynamiclib -framework Foundation \
  -install_name @rpath/ZorvokFixture.framework/ZorvokFixture \
  -o build/ZorvokFixture.framework/ZorvokFixture build/Fixture.m

# ...then an Info.plist (CFBundlePackageType=FMWK, CFBundleExecutable=ZorvokFixture,
# CFBundleSupportedPlatforms=[iPhoneSimulator]) next to the binary, and:
xcodebuild -create-xcframework \
  -framework build/ZorvokFixture.framework \
  -output pod/ZorvokFixture.xcframework
```

Only re-run this if the fixture itself needs to change (e.g. adding a
second architecture slice to test fat-framework handling). The
`XCFrameworkPod.podspec` and `XCFrameworkApp.xcodeproj` don't need to
change when regenerating the binary.
