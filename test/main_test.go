package codeye_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var mockAgentBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "codeye-test-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: %v\n", err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "mockagent")
	_, thisFile, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Dir(filepath.Dir(thisFile))
	cmd := exec.Command("go", "build", "-o", bin, "./test/testdata/mockagent")
	cmd.Dir = moduleRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: build mock agent: %v\n%s\n", err, out)
	} else {
		mockAgentBinary = bin
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func shortHome(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "cye-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("HOME", dir)
	return dir
}
