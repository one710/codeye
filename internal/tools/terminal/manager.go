package terminal

import (
	"bytes"
	"errors"
	"os/exec"
	"sync"
	"syscall"
)

type proc struct {
	cmd    *exec.Cmd
	buffer *bytes.Buffer
	done   chan error
	exit   WaitResult
	waited bool
}

type WaitResult struct {
	ExitCode int
	Signal   string
}

type Manager struct {
	mu    sync.Mutex
	next  int
	procs map[int]*proc
}

func New() *Manager {
	return &Manager{procs: map[int]*proc{}}
}

func (m *Manager) Create(command string, args ...string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.next + 1
	m.next = id
	buf := &bytes.Buffer{}
	cmd := exec.Command(command, args...)
	cmd.Stdout = buf
	cmd.Stderr = buf
	p := &proc{cmd: cmd, buffer: buf, done: make(chan error, 1)}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	m.procs[id] = p
	go func() {
		err := cmd.Wait()
		p.exit = extractWaitResult(cmd, err)
		p.waited = true
		p.done <- err
	}()
	return id, nil
}

func (m *Manager) Output(id int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.procs[id]
	if !ok {
		return "", errors.New("terminal not found")
	}
	return p.buffer.String(), nil
}

func (m *Manager) Wait(id int) (WaitResult, error) {
	m.mu.Lock()
	p, ok := m.procs[id]
	m.mu.Unlock()
	if !ok {
		return WaitResult{}, errors.New("terminal not found")
	}
	err := <-p.done
	return p.exit, err
}

func (m *Manager) Kill(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.procs[id]
	if !ok {
		return errors.New("terminal not found")
	}
	return p.cmd.Process.Signal(syscall.SIGTERM)
}

func (m *Manager) Release(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.procs, id)
}

func extractWaitResult(cmd *exec.Cmd, waitErr error) WaitResult {
	result := WaitResult{ExitCode: 0}
	if cmd.ProcessState == nil {
		if waitErr != nil {
			result.ExitCode = 1
		}
		return result
	}
	result.ExitCode = cmd.ProcessState.ExitCode()
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
		if ws.Signaled() {
			result.Signal = ws.Signal().String()
		}
	}
	return result
}
