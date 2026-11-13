package internal

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChatSession is a locally registered API conversation, not a ChatGPT app chat.
// OpenAI does not expose the ChatGPT consumer chat history through this interface.
type ChatSession struct {
	ID         string    `json:"id"`
	ProviderID string    `json:"providerId"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ModelOption struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerId"`
	OwnedBy    string `json:"ownedBy,omitempty"`
}

type ChatMessage struct {
	Role string
	Text string
}

type ChatProvider interface {
	ID() string
	Label() string
	Models(context.Context) ([]ModelOption, error)
	CreateConversation(context.Context) (string, error)
	History(context.Context, string, int) ([]ChatMessage, error)
	Review(context.Context, string, string, string, string) (string, error)
}

// ProviderRegistry is the extension point for future Gemini, Claude and Grok adapters.
// Registering an adapter does not imply that a provider is already supported.
type ProviderRegistry struct {
	mu       sync.RWMutex
	adapters map[string]ChatProvider
}

func NewProviderRegistry(adapters ...ChatProvider) *ProviderRegistry {
	r := &ProviderRegistry{adapters: make(map[string]ChatProvider)}
	for _, p := range adapters {
		_ = r.Register(p)
	}
	return r
}

func (r *ProviderRegistry) Register(p ChatProvider) error {
	if p == nil || strings.TrimSpace(p.ID()) == "" {
		return errors.New("provider must have an ID")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id := strings.ToLower(strings.TrimSpace(p.ID()))
	if _, exists := r.adapters[id]; exists {
		return errors.New("provider already registered: " + id)
	}
	r.adapters[id] = p
	return nil
}

func (r *ProviderRegistry) Get(id string) (ChatProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.adapters[strings.ToLower(strings.TrimSpace(id))]
	return p, ok
}

func (r *ProviderRegistry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
