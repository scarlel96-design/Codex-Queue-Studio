package internal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Settings struct {
	CodexHome string `json:"codexHome"`
	CodexPath string `json:"codexPath"`
	Theme     string `json:"theme"`
	AutoStart bool   `json:"autoStart"`
}

type Job struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"sessionId"`
	SessionTitle string    `json:"sessionTitle"`
	Cipher       string    `json:"cipher"`
	Preview      string    `json:"preview"`
	When         time.Time `json:"when"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	State        string    `json:"state"`
	Mode         string    `json:"mode"`
	Error        string    `json:"error,omitempty"`
	Detail       string    `json:"detail,omitempty"`
}

type JobPublic struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"sessionId"`
	SessionTitle string    `json:"sessionTitle"`
	Preview      string    `json:"preview"`
	When         time.Time `json:"when"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	State        string    `json:"state"`
	Mode         string    `json:"mode"`
	Error        string    `json:"error,omitempty"`
	Detail       string    `json:"detail,omitempty"`
}

func (j Job) Public() JobPublic {
	// The UI only needs the already persisted preview. Never decrypt on paint/timer.
	return JobPublic{j.ID, j.SessionID, j.SessionTitle, j.Preview, j.When, j.CreatedAt, j.UpdatedAt, j.State, j.Mode, j.Error, j.Detail}
}

type Event struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	JobID   string    `json:"jobId"`
	Message string    `json:"message"`
}

type State struct {
	Version      int           `json:"version"`
	Settings     Settings      `json:"settings"`
	Jobs         []Job         `json:"jobs"`
	Events       []Event       `json:"events"`
	Relays       []RelayRule   `json:"relays,omitempty"`
	ChatSessions []ChatSession `json:"chatSessions,omitempty"`
}

type Store struct {
	mu    sync.Mutex
	path  string
	state State
}

func NewStore(path string, initial Settings) (*Store, error) {
	st := &Store{path: path, state: State{Version: 3, Settings: initial, Jobs: []Job{}, Events: []Event{}}}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &st.state); err != nil {
			return nil, fmt.Errorf("예약 저장소 손상: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	changed := false
	for i := range st.state.Jobs {
		if st.state.Jobs[i].State == "sending" {
			st.state.Jobs[i].State = "uncertain"
			st.state.Jobs[i].Error = "실행 도중 종료되었습니다. 중복 방지를 위해 자동 재전송하지 않습니다."
			changed = true
		}
	}
	for i := range st.state.Relays {
		if st.state.Relays[i].State == "reviewing" || st.state.Relays[i].State == "reply-sending" {
			st.state.Relays[i].State = "uncertain"
			st.state.Relays[i].LastError = "프로그램이 처리 도중 종료되었습니다. 중복 방지를 위해 자동 재실행하지 않습니다."
			changed = true
		}
	}
	if changed {
		if err := st.writeLocked(); err != nil {
			return nil, err
		}
	}
	return st, nil
}

func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.state
	out.Jobs = append([]Job(nil), s.state.Jobs...)
	out.Events = append([]Event(nil), s.state.Events...)
	out.Relays = append([]RelayRule(nil), s.state.Relays...)
	out.ChatSessions = append([]ChatSession(nil), s.state.ChatSessions...)
	return out
}

func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := s.state
	next := before
	next.Jobs = append([]Job(nil), before.Jobs...)
	next.Events = append([]Event(nil), before.Events...)
	next.Relays = append([]RelayRule(nil), before.Relays...)
	next.ChatSessions = append([]ChatSession(nil), before.ChatSessions...)
	if err := fn(&next); err != nil {
		return err
	}
	s.state = next
	if err := s.writeLocked(); err != nil {
		s.state = before
		return err
	}
	return nil
}

func (s *Store) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	temp := s.path + ".tmp"
	f, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(temp, s.path); err != nil {
		return err
	}
	return nil
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func AddEvent(st *State, kind, id, message string) {
	st.Events = append(st.Events, Event{Time: time.Now(), Kind: kind, JobID: id, Message: message})
	if len(st.Events) > 400 {
		st.Events = append([]Event(nil), st.Events[len(st.Events)-400:]...)
	}
}
func SortPublicJobs(jobs []Job) []JobPublic {
	a := make([]JobPublic, 0, len(jobs))
	for _, j := range jobs {
		a = append(a, j.Public())
	}
	sort.Slice(a, func(i, j int) bool { return a[i].When.After(a[j].When) })
	return a
}
