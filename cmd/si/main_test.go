package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	moduleVersion := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	noBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }

	tests := []struct {
		name          string
		stamped       string
		readBuildInfo func() (*debug.BuildInfo, bool)
		want          string
	}{
		{name: "release build", stamped: "0.1.0", readBuildInfo: moduleVersion("v9.9.9"), want: "0.1.0"},
		{name: "go install", readBuildInfo: moduleVersion("v0.1.0"), want: "0.1.0"},
		{name: "go run", readBuildInfo: moduleVersion("(devel)"), want: "dev"},
		{name: "no module version", readBuildInfo: moduleVersion(""), want: "dev"},
		{name: "no build info", readBuildInfo: noBuildInfo, want: "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildVersion(tt.stamped, tt.readBuildInfo); got != tt.want {
				t.Errorf("buildVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
