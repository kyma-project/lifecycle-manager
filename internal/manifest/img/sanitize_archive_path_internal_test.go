package img

import (
	"testing"
)

// TestSanitizeArchivePath_PathTraversal verifies that a TAR entry whose resolved
// path lands in a sibling directory sharing a string prefix with the intended dir
// is rejected. This test currently FAILS because sanitizeArchivePath uses
// strings.HasPrefix without a trailing separator, allowing the bypass. It will
// pass once the fix (appending os.PathSeparator to cleanDir) is applied.
//
// Attack scenario:
//   - Malicious module version: existing-1.0.1  → dir: /tmp/existing-1.0.1
//   - Victim   module version:  existing-1.0.10 → dir: /tmp/existing-1.0.10
//
// "1.0.1" is a string prefix of "1.0.10", so the bypass succeeds.
// Both are valid semver versions (major.minor.patch, no leading "v").
func TestSanitizeArchivePath_PathTraversal(t *testing.T) {
	dir := "/tmp/existing-1.0.1"
	maliciousEntry := "../existing-1.0.10/raw-manifest.yaml"

	_, err := sanitizeArchivePath(dir, maliciousEntry)
	if err == nil {
		t.Errorf("sanitizeArchivePath(%q, %q) returned nil error; expected ErrTaintedArchive", dir, maliciousEntry)
	}
}

// TestSanitizeArchivePath_CrossModuleTraversalRejected verifies that a TAR entry
// targeting a completely different module's cache directory is rejected. This test
// PASSES with the current implementation — cross-module traversal does not bypass
// the strings.HasPrefix check because the two directory names share no common prefix.
func TestSanitizeArchivePath_CrossModuleTraversalRejected(t *testing.T) {
	dir := "/tmp/existing-1.0.1"
	crossModuleEntry := "../another-1.0.1/raw-manifest.yaml"

	_, err := sanitizeArchivePath(dir, crossModuleEntry)
	if err == nil {
		t.Errorf("sanitizeArchivePath(%q, %q) returned nil error; expected ErrTaintedArchive", dir, crossModuleEntry)
	}
}
