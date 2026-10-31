package packages

import "regexp"

// namePattern follows CocoaPods' own pod-name convention closely enough
// to avoid obvious confusion/typosquatting tricks (no leading digits,
// no unicode lookalikes, no path separators) without being stricter
// than real-world pod names require.
var namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+\-]{0,99}$`)

// versionPattern accepts semver-like versions (CocoaPods itself doesn't
// enforce strict SemVer, but the registry does — ADR 0003's version
// immutability and federation precedence rules depend on versions being
// comparably ordered).
var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.\-]+)?(\+[0-9A-Za-z.\-]+)?$`)

// ValidateName reports whether name is an acceptable package identifier.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}

// ValidateVersion reports whether version is an acceptable, orderable
// version string.
func ValidateVersion(version string) error {
	if !versionPattern.MatchString(version) {
		return ErrInvalidVersion
	}
	return nil
}
