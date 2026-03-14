package queue

import (
	"errors"
	"os"
	"strconv"
	"syscall"
	"time"
)

type LeaseStore struct {
	path string
}

func NewLeaseStore(path string) *LeaseStore {
	return &LeaseStore{path: path}
}

func (l *LeaseStore) Acquire(pid int) error {
	return os.WriteFile(l.path, []byte(strconv.Itoa(pid)), 0o644)
}

func (l *LeaseStore) ReadPID() (int, error) {
	b, err := os.ReadFile(l.path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(b))
}

func (l *LeaseStore) Refresh() error {
	now := []byte(time.Now().UTC().Format(time.RFC3339))
	return os.WriteFile(l.path+".heartbeat", now, 0o644)
}

func (l *LeaseStore) HeartbeatAge(now time.Time) (time.Duration, error) {
	b, err := os.ReadFile(l.path + ".heartbeat")
	if err != nil {
		return 0, err
	}
	ts, err := time.Parse(time.RFC3339, string(b))
	if err != nil {
		return 0, err
	}
	if now.Before(ts) {
		return 0, nil
	}
	return now.Sub(ts), nil
}

func (l *LeaseStore) IsStale(maxHeartbeatAge time.Duration) bool {
	pid, err := l.ReadPID()
	if err != nil {
		return true
	}
	if !processAlive(pid) {
		return true
	}
	if maxHeartbeatAge <= 0 {
		return false
	}
	age, err := l.HeartbeatAge(time.Now().UTC())
	if err != nil {
		return true
	}
	return age > maxHeartbeatAge
}

func (l *LeaseStore) Release() error {
	_ = os.Remove(l.path + ".heartbeat")
	err := os.Remove(l.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
