package initializr

import "testing"

func TestDependencySupports(t *testing.T) {
	tests := []struct {
		versionRange, bootVersion string
		want                      bool
	}{
		// No range, or no version to check against.
		{"", "4.1.1", true},
		{"[4.0.0,4.1.0-M1)", "", true},

		// A lower bound only.
		{"4.0.0", "4.0.0", true},
		{"4.0.0", "4.1.1", true},
		{"4.0.0", "3.5.9", false},
		{"4.1.0-M3", "4.1.0-M2", false},
		{"4.1.0-M3", "4.1.0-M3", true},
		{"4.1.0-M3", "4.1.0-RC1", true},

		// Both bounds.
		{"[4.0.0,4.1.0-M1)", "4.0.0", true},
		{"[4.0.0,4.1.0-M1)", "4.0.8", true},
		{"[4.0.0,4.1.0-M1)", "4.0.9-SNAPSHOT", true},
		{"[4.0.0,4.1.0-M1)", "4.1.0-M1", false},
		{"[4.0.0,4.1.0-M1)", "4.1.1", false},
		{"[3.5.0,4.0.0)", "4.0.0", false},
		{"[3.5.0,4.0.0)", "4.0.0-SNAPSHOT", true},
		{"[3.5.0,4.0.0)", "4.0.0-RC2", true},
		{"(4.0.0,4.1.0]", "4.0.0", false},
		{"(4.0.0,4.1.0]", "4.1.0", true},
		{"(4.0.0,4.1.0]", "4.1.1", false},
		{" [4.0.0, 4.1.0-M1) ", "4.0.8", true},

		// Milestones come before release candidates, those before
		// snapshots, and those before the release.
		{"[4.2.0-M2,4.2.0-RC1)", "4.2.0-M1", false},
		{"[4.2.0-M2,4.2.0-RC1)", "4.2.0-M10", true},
		{"[4.2.0-M2,4.2.0-RC1)", "4.2.0-RC1", false},
		{"[4.2.0-RC1,4.2.0)", "4.2.0-SNAPSHOT", true},
		{"[4.2.0-RC1,4.2.0)", "4.2.0", false},
		{"4.2.0", "4.2.0-SNAPSHOT", false},

		// Anything unreadable is left for Initializr to judge.
		{"[4.0.0,4.1.0-M1)", "latest", true},
		{"[4.0.0,4.1.0-M1)", "4.2", true},
		{"[4.0.0,4.1.0-M1)", "4.2.0-GA", true},
		{"newer", "4.1.1", true},
		{"[4.0.0,", "4.1.1", true},
		{"4.0.0,4.1.0", "4.1.1", true},
		{"[4.x,5.0.0)", "4.1.1", true},
	}
	for _, tt := range tests {
		d := Dependency{ID: "dep", VersionRange: tt.versionRange}
		if got := d.Supports(tt.bootVersion); got != tt.want {
			t.Errorf("range %q, Spring Boot %q: Supports() = %v, want %v", tt.versionRange, tt.bootVersion, got, tt.want)
		}
	}
}
