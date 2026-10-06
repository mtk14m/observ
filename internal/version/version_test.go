package version

import "testing"

func TestString(t *testing.T) {
	t.Cleanup(func() { Version, Commit = "dev", "unknown" })

	Version, Commit = "v0.1.0", "abc1234"
	if got, want := String(), "obsrv v0.1.0 (abc1234)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
