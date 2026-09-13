package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// LastRecord is the last build bam triggered from one repository. It holds no
// variable values.
type LastRecord struct {
	BuildKey    string    `json:"build_key"`
	Origin      string    `json:"origin"`
	PlanKey     string    `json:"plan_key"`
	Target      string    `json:"target,omitempty"`
	TriggeredAt time.Time `json:"triggered_at"`
}

type stateFile struct {
	Version int                   `json:"version"`
	Repos   map[string]LastRecord `json:"repos"`
}

// StateStore reads and writes $XDG_STATE_HOME/bam/state.json.
type StateStore struct {
	Path string
}

func (s *StateStore) load() (stateFile, error) {
	f := stateFile{Version: 1, Repos: map[string]LastRecord{}}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return stateFile{Version: 1, Repos: map[string]LastRecord{}}, nil // a corrupt state file is rebuilt
	}
	if f.Repos == nil {
		f.Repos = map[string]LastRecord{}
	}
	return f, nil
}

func (s *StateStore) save(f stateFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

// Last returns the record for a repository root.
func (s *StateStore) Last(root string) (LastRecord, bool, error) {
	f, err := s.load()
	if err != nil {
		return LastRecord{}, false, err
	}
	r, ok := f.Repos[root]
	return r, ok, nil
}

// SetLast replaces the record for a repository root.
func (s *StateStore) SetLast(root string, r LastRecord) error {
	f, err := s.load()
	if err != nil {
		return err
	}
	f.Repos[root] = r
	return s.save(f)
}

// Forget removes the record for a repository root.
func (s *StateStore) Forget(root string) error {
	f, err := s.load()
	if err != nil {
		return err
	}
	delete(f.Repos, root)
	return s.save(f)
}
