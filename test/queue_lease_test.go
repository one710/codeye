package codeye_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/one710/codeye/internal/queue"
)

func TestLeaseStaleWhenHeartbeatOld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	if err := store.Acquire(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339)
	if err := os.WriteFile(path+".heartbeat", []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if !store.IsStale(2 * time.Second) {
		t.Fatalf("expected stale lease with old heartbeat")
	}
}

func TestLeaseNotStaleWhenAliveAndFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	if err := store.Acquire(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if err := store.Refresh(); err != nil {
		t.Fatal(err)
	}
	if store.IsStale(5 * time.Second) {
		t.Fatalf("expected non-stale lease")
	}
}

func TestLeaseStaleWhenNoFile(t *testing.T) {
	store := queue.NewLeaseStore(filepath.Join(t.TempDir(), "missing.lease"))
	if !store.IsStale(2 * time.Second) {
		t.Fatal("expected stale when lease file does not exist")
	}
}

func TestLeaseRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	if err := store.Acquire(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if err := store.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("lease file should exist after acquire")
	}
	if err := store.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("lease file should be removed after release")
	}
	if _, err := os.Stat(path + ".heartbeat"); !os.IsNotExist(err) {
		t.Fatal("heartbeat file should be removed after release")
	}
}

func TestLeaseReleaseNonexistent(t *testing.T) {
	store := queue.NewLeaseStore(filepath.Join(t.TempDir(), "missing.lease"))
	if err := store.Release(); err != nil {
		t.Fatalf("releasing nonexistent lease should not error: %v", err)
	}
}

func TestReadPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	if err := store.Acquire(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	pid, err := store.ReadPID()
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Fatalf("expected %d, got %d", os.Getpid(), pid)
	}
}

func TestReadPIDMissing(t *testing.T) {
	store := queue.NewLeaseStore(filepath.Join(t.TempDir(), "missing.lease"))
	_, err := store.ReadPID()
	if err == nil {
		t.Fatal("expected error for missing lease")
	}
}

func TestHeartbeatAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	ts := time.Now().UTC().Add(-5 * time.Second).Format(time.RFC3339)
	os.WriteFile(path+".heartbeat", []byte(ts), 0o644)
	age, err := store.HeartbeatAge(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if age < 4*time.Second || age > 7*time.Second {
		t.Fatalf("expected ~5s age, got %v", age)
	}
}

func TestHeartbeatAgeFuture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	ts := time.Now().UTC().Add(10 * time.Second).Format(time.RFC3339)
	os.WriteFile(path+".heartbeat", []byte(ts), 0o644)
	age, err := store.HeartbeatAge(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if age != 0 {
		t.Fatalf("future heartbeat should return 0 age, got %v", age)
	}
}

func TestHeartbeatAgeMissing(t *testing.T) {
	store := queue.NewLeaseStore(filepath.Join(t.TempDir(), "missing.lease"))
	_, err := store.HeartbeatAge(time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for missing heartbeat")
	}
}

func TestHeartbeatAgeInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	os.WriteFile(path+".heartbeat", []byte("not-a-timestamp"), 0o644)
	_, err := store.HeartbeatAge(time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestIsStaleWithZeroMaxAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.lease")
	store := queue.NewLeaseStore(path)
	if err := store.Acquire(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if store.IsStale(0) {
		t.Fatal("zero maxHeartbeatAge should mean 'don't check heartbeat'")
	}
}
