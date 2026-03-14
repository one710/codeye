package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	recordSchema = "codeye.session.v1"
	indexSchema  = "codeye.session-index.v1"
)

type Repo struct {
	root string
}

func New(root string) *Repo {
	return &Repo{root: root}
}

func (r *Repo) sessionsDir() string { return filepath.Join(r.root, "sessions") }
func (r *Repo) indexPath() string   { return filepath.Join(r.sessionsDir(), "index.json") }

func (r *Repo) Init() error {
	return os.MkdirAll(r.sessionsDir(), 0o755)
}

func (r *Repo) Save(record Record) error {
	if err := r.Init(); err != nil {
		return err
	}
	if record.RecordID == "" {
		return errors.New("recordId is required")
	}
	record.Schema = recordSchema
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	path := filepath.Join(r.sessionsDir(), record.RecordID+".json")
	if err := atomicWriteJSON(path, record); err != nil {
		return err
	}
	idx, err := r.loadIndex()
	if err != nil {
		return err
	}
	if idx.ByScope == nil {
		idx.ByScope = map[string]string{}
	}
	idx.ByScope[scopeKey(record.Agent, record.Cwd, record.Name)] = record.RecordID
	return atomicWriteJSON(r.indexPath(), idx)
}

func (r *Repo) Load(recordID string) (Record, error) {
	path := filepath.Join(r.sessionsDir(), recordID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// List returns all session records (excluding index) from the sessions directory.
func (r *Repo) List() ([]Record, error) {
	if err := r.Init(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(r.sessionsDir())
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "index.json" {
			continue
		}
		rec, err := r.Load(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		if rec.RecordID != "" {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *Repo) Find(agent, cwd, name string) (Record, error) {
	idx, err := r.loadIndex()
	if err != nil {
		return Record{}, err
	}
	id := idx.ByScope[scopeKey(agent, cwd, name)]
	if id == "" {
		return Record{}, os.ErrNotExist
	}
	return r.Load(id)
}

func (r *Repo) loadIndex() (Index, error) {
	if err := r.Init(); err != nil {
		return Index{}, err
	}
	b, err := os.ReadFile(r.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return Index{Schema: indexSchema, ByScope: map[string]string{}}, nil
		}
		return Index{}, err
	}
	var idx Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return r.rebuildIndex()
	}
	if idx.ByScope == nil {
		idx.ByScope = map[string]string{}
	}
	return idx, nil
}

func (r *Repo) rebuildIndex() (Index, error) {
	entries, err := os.ReadDir(r.sessionsDir())
	if err != nil {
		return Index{}, err
	}
	idx := Index{Schema: indexSchema, ByScope: map[string]string{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "index.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.sessionsDir(), entry.Name()))
		if err != nil {
			continue
		}
		var rec Record
		if json.Unmarshal(b, &rec) == nil && rec.RecordID != "" {
			idx.ByScope[scopeKey(rec.Agent, rec.Cwd, rec.Name)] = rec.RecordID
		}
	}
	_ = atomicWriteJSON(r.indexPath(), idx)
	return idx, nil
}

func scopeKey(agent, cwd, name string) string {
	return fmt.Sprintf("%s|%s|%s", agent, filepath.Clean(cwd), name)
}

func atomicWriteJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
