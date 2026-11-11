package internal

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Session struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Project     string    `json:"project"`
	Path        string    `json:"path"`
	Source      string    `json:"source"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Archived    bool      `json:"archived"`
	File        string    `json:"file"`
	TitleSource string    `json:"titleSource"`
}

type HistoryLine struct {
	Role string
	Text string
}

type indexEntry struct {
	ID         string `json:"id"`
	SessionID  string `json:"session_id"`
	ThreadName string `json:"thread_name"`
	Title      string `json:"title"`
	Name       string `json:"name"`
}
type rolloutEvent struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}
type sessionMeta struct {
	ID         string          `json:"id"`
	CWD        string          `json:"cwd"`
	Source     json.RawMessage `json:"source"`
	Originator string          `json:"originator"`
}
type payloadMessage struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Message string          `json:"message"`
	Text    string          `json:"text"`
	Content json.RawMessage `json:"content"`
}

func clipLine(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return s
}

// Generated review and worker titles are not meaningful session titles.
func genericTitle(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "untitled" || s == "제목 없음" || s == "new thread" || s == "new conversation" {
		return true
	}
	for _, v := range []string{"guardian review", "code review", "review agent", "guardian", "background review"} {
		if s == v {
			return true
		}
	}
	return false
}

func extractText(p payloadMessage) string {
	if v := strings.TrimSpace(p.Message); v != "" {
		return v
	}
	if v := strings.TrimSpace(p.Text); v != "" {
		return v
	}
	var content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(p.Content, &content) == nil {
		var texts []string
		for _, item := range content {
			if item.Type == "input_text" || item.Type == "output_text" || item.Type == "text" {
				texts = append(texts, item.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func parseConversation(event rolloutEvent) HistoryLine {
	if event.Type != "event_msg" && event.Type != "response_item" {
		return HistoryLine{}
	}
	var p payloadMessage
	if json.Unmarshal(event.Payload, &p) != nil {
		return HistoryLine{}
	}
	var role string
	switch {
	case p.Role == "user", p.Type == "user_message":
		role = "사용자"
	case p.Role == "assistant", p.Type == "agent_message":
		role = "Codex"
	default:
		return HistoryLine{}
	}
	msg := strings.TrimSpace(extractText(p))
	if msg == "" {
		return HistoryLine{}
	}
	return HistoryLine{Role: role, Text: msg}
}

func candidateTitle(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, token := range []string{"<environment_context>", "<system>", "# agents.md", "you are codex", "guardian review", "[agent note]", "<developer_instructions>"} {
		if strings.HasPrefix(lower, token) {
			return ""
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = clipLine(strings.TrimSpace(line), 90)
		if line != "" && len([]rune(line)) >= 3 && !genericTitle(line) && !strings.HasPrefix(line, "<") {
			return line
		}
	}
	return ""
}

func scanJSONLines(path string, maxBytes int64, maxLines int, visit func([]byte) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := bufio.NewReader(io.LimitReader(f, maxBytes))
	for i := 0; i < maxLines; i++ {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 && !visit(line) {
			break
		}
		if err != nil {
			break
		}
	}
	return nil
}

func ScanSessions(home string, maxFiles int) ([]Session, error) {
	if maxFiles <= 0 {
		maxFiles = 20000
	}
	if home == "" {
		return nil, errors.New("Codex 홈 경로를 지정하세요")
	}
	home, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	titles := map[string]string{}
	_ = scanJSONLines(filepath.Join(home, "session_index.jsonl"), 32<<20, 150000, func(line []byte) bool {
		var e indexEntry
		if json.Unmarshal(line, &e) != nil {
			return true
		}
		id := e.ID
		if id == "" {
			id = e.SessionID
		}
		n := e.ThreadName
		if n == "" {
			n = e.Name
		}
		if n == "" {
			n = e.Title
		}
		if id != "" && n != "" {
			titles[id] = clipLine(n, 100)
		}
		return true
	})
	roots := []struct {
		dir      string
		archived bool
	}{{filepath.Join(home, "sessions"), false}, {filepath.Join(home, "archived_sessions"), true}}
	found := map[string]Session{}
	scanned := 0
	var firstError error
	for _, root := range roots {
		if _, err := os.Stat(root.dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root.dir, func(path string, entry fs.DirEntry, e error) error {
			if e != nil {
				if firstError == nil {
					firstError = e
				}
				return nil
			}
			if entry.IsDir() {
				if strings.Count(strings.TrimPrefix(path, root.dir), string(os.PathSeparator)) > 7 {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
				return nil
			}
			scanned++
			if scanned > maxFiles {
				return filepath.SkipAll
			}
			st, e := entry.Info()
			if e != nil {
				return nil
			}
			s := Session{File: path, UpdatedAt: st.ModTime(), Archived: root.archived, Source: "로컬", Title: "제목 없음", TitleSource: "missing"}
			idxTitle := ""
			firstUser := ""
			var gotMeta bool
			_ = scanJSONLines(path, 12<<20, 2800, func(line []byte) bool {
				var event rolloutEvent
				if json.Unmarshal(line, &event) != nil {
					return true
				}
				if event.Type == "session_meta" {
					var meta sessionMeta
					if json.Unmarshal(event.Payload, &meta) == nil && meta.ID != "" {
						s.ID = meta.ID
						s.Path = meta.CWD
						s.Project = filepath.Base(meta.CWD)
						s.Source = sourceText(meta.Source, meta.Originator)
						gotMeta = true
					}
				}
				if firstUser == "" {
					if h := parseConversation(event); h.Role == "사용자" {
						firstUser = candidateTitle(h.Text)
					}
				}
				return !gotMeta || firstUser == ""
			})
			if s.ID == "" {
				return nil
			}
			idxTitle = titles[s.ID]
			if idxTitle != "" && !genericTitle(idxTitle) {
				s.Title = idxTitle
				s.TitleSource = "index"
			} else if firstUser != "" {
				s.Title = firstUser
				s.TitleSource = "user"
			} else if idxTitle != "" {
				s.Title = "제목 없음 · " + idxTitle
				s.TitleSource = "unresolved"
			}
			if s.Project == "" || s.Project == "." {
				s.Project = "프로젝트 없음"
			}
			if old, ok := found[s.ID]; !ok || s.UpdatedAt.After(old.UpdatedAt) {
				found[s.ID] = s
			}
			return nil
		})
		if err != nil && firstError == nil {
			firstError = err
		}
	}
	result := make([]Session, 0, len(found))
	for _, s := range found {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	if len(result) == 0 && firstError != nil {
		return result, firstError
	}
	return result, nil
}

func ReadHistory(path string, maxItems int) ([]HistoryLine, error) {
	if maxItems <= 0 {
		maxItems = 200
	}
	result := make([]HistoryLine, 0, maxItems)
	err := scanJSONLines(path, 32<<20, 50000, func(line []byte) bool {
		var ev rolloutEvent
		if json.Unmarshal(line, &ev) != nil {
			return true
		}
		h := parseConversation(ev)
		if h.Role == "" {
			return true
		}
		// The same input appears in event_msg and response_item on some Codex versions.
		if len(result) > 0 && result[len(result)-1].Role == h.Role && result[len(result)-1].Text == h.Text {
			return true
		}
		if len([]rune(h.Text)) > 5000 {
			h.Text = string([]rune(h.Text)[:5000]) + "…"
		}
		result = append(result, h)
		if len(result) > maxItems {
			result = result[1:]
		}
		return true
	})
	return result, err
}

func sourceText(raw json.RawMessage, originator string) string {
	var v string
	if json.Unmarshal(raw, &v) == nil {
		switch strings.ToLower(v) {
		case "app":
			return "Codex 앱"
		case "cli":
			return "CLI"
		case "vscode":
			return "IDE"
		case "exec":
			return "자동 실행"
		}
		if v != "" {
			return v
		}
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) == nil {
		for k := range m {
			switch strings.ToLower(k) {
			case "app":
				return "Codex 앱"
			case "cli":
				return "CLI"
			case "vscode":
				return "IDE"
			case "exec":
				return "자동 실행"
			}
		}
	}
	if strings.Contains(strings.ToLower(originator), "app") {
		return "Codex 앱"
	}
	return "로컬"
}
