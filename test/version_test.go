package codeye_test

import (
	"testing"

	"github.com/one710/codeye/internal/version"
)

func TestVersionString(t *testing.T) {
	v := version.String()
	if v == "" {
		t.Fatal("version string should not be empty")
	}
	if v != version.Version {
		t.Fatalf("expected %s, got %s", version.Version, v)
	}
}
