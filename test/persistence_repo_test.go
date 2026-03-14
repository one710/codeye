package codeye_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/one710/codeye/internal/session/persistence"
)

func TestRepoSaveAndFind(t *testing.T) {
	repo := persistence.New(t.TempDir())
	rec := persistence.Record{RecordID: "r1", ACPSession: "acp-1", Agent: "codex", Cwd: "/tmp"}
	if err := repo.Save(rec); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Find("codex", "/tmp", "")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.RecordID != "r1" {
		t.Fatalf("unexpected record id: %s", got.RecordID)
	}
	if got.Schema == "" {
		t.Fatal("schema should be set")
	}
	if got.CreatedAt == "" {
		t.Fatal("createdAt should be set")
	}
	if got.UpdatedAt == "" {
		t.Fatal("updatedAt should be set")
	}
}

func TestRepoFindMissing(t *testing.T) {
	repo := persistence.New(t.TempDir())
	_, err := repo.Find("codex", "/tmp", "")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestRepoLoadByID(t *testing.T) {
	repo := persistence.New(t.TempDir())
	rec := persistence.Record{RecordID: "r2", ACPSession: "acp-2", Agent: "claude", Cwd: "/tmp"}
	repo.Save(rec)
	got, err := repo.Load("r2")
	if err != nil {
		t.Fatal(err)
	}
	if got.Agent != "claude" {
		t.Fatalf("expected claude, got %s", got.Agent)
	}
}

func TestRepoLoadMissing(t *testing.T) {
	repo := persistence.New(t.TempDir())
	_, err := repo.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing record")
	}
}

func TestRepoSaveRequiresRecordID(t *testing.T) {
	repo := persistence.New(t.TempDir())
	err := repo.Save(persistence.Record{Agent: "codex", Cwd: "/tmp"})
	if err == nil {
		t.Fatal("expected error for empty recordId")
	}
}

func TestRepoSaveOverwrites(t *testing.T) {
	repo := persistence.New(t.TempDir())
	rec := persistence.Record{RecordID: "r3", ACPSession: "acp-3", Agent: "codex", Cwd: "/tmp"}
	repo.Save(rec)
	rec.ACPSession = "acp-3-updated"
	repo.Save(rec)
	got, err := repo.Load("r3")
	if err != nil {
		t.Fatal(err)
	}
	if got.ACPSession != "acp-3-updated" {
		t.Fatalf("expected updated session, got %s", got.ACPSession)
	}
}

func TestRepoNamedSessions(t *testing.T) {
	repo := persistence.New(t.TempDir())
	repo.Save(persistence.Record{RecordID: "r-api", ACPSession: "s1", Agent: "codex", Cwd: "/tmp", Name: "api"})
	repo.Save(persistence.Record{RecordID: "r-docs", ACPSession: "s2", Agent: "codex", Cwd: "/tmp", Name: "docs"})
	api, err := repo.Find("codex", "/tmp", "api")
	if err != nil {
		t.Fatal(err)
	}
	if api.RecordID != "r-api" {
		t.Fatalf("expected r-api, got %s", api.RecordID)
	}
	docs, err := repo.Find("codex", "/tmp", "docs")
	if err != nil {
		t.Fatal(err)
	}
	if docs.RecordID != "r-docs" {
		t.Fatalf("expected r-docs, got %s", docs.RecordID)
	}
}

func TestRepoRebuildCorruptIndex(t *testing.T) {
	root := t.TempDir()
	repo := persistence.New(root)
	rec := persistence.Record{RecordID: "r4", ACPSession: "acp-4", Agent: "codex", Cwd: "/tmp"}
	repo.Save(rec)

	indexPath := filepath.Join(root, "sessions", "index.json")
	os.WriteFile(indexPath, []byte("{corrupt}"), 0o644)

	got, err := repo.Find("codex", "/tmp", "")
	if err != nil {
		t.Fatalf("Find after corrupt index: %v", err)
	}
	if got.RecordID != "r4" {
		t.Fatalf("expected r4 after rebuild, got %s", got.RecordID)
	}
}
