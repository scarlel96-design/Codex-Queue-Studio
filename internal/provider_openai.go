package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// OpenAIChatProvider implements ChatProvider against the public OpenAI API.
// It does not access personal ChatGPT app conversations.
type OpenAIChatProvider struct{ Client ResponsesClient }

func NewOpenAIChatProvider() OpenAIChatProvider {
	return OpenAIChatProvider{Client: NewResponsesClientFromEnvironment()}
}
func (OpenAIChatProvider) ID() string    { return "openai" }
func (OpenAIChatProvider) Label() string { return "GPT · OpenAI API" }

func (p OpenAIChatProvider) CreateConversation(ctx context.Context) (string, error) {
	return p.Client.CreateConversation(ctx)
}
func (p OpenAIChatProvider) Review(ctx context.Context, conversationID, model, report, instructions string) (string, error) {
	return p.Client.Review(ctx, conversationID, model, report, instructions)
}
func (a ResponsesClient) fetchJSON(ctx context.Context, path string, target any) error {
	if a.Key == "" {
		return errors.New("OPENAI_API_KEY 환경 변수를 설정하세요")
	}
	root := strings.TrimRight(a.BaseURL, "/")
	u, err := url.Parse(root)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")) {
		return errors.New("API 주소는 HTTPS 또는 로컬 테스트 서버여야 합니다")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.Key)
	client := a.HTTP
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("OpenAI API HTTP %d (응답 본문은 기록하지 않음)", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Model listing is live, account-scoped, and excludes obviously incompatible
// audio, image, embedding and moderation products. Responses compatibility
// must still be confirmed by the API when the user sends a review.
func eligibleReviewModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if !(strings.HasPrefix(id, "gpt-") || (len(id) >= 2 && id[0] == 'o' && id[1] >= '0' && id[1] <= '9')) {
		return false
	}
	for _, fragment := range []string{"audio", "tts", "transcrib", "embedding", "image", "realtime", "search", "moderation", "diariz", "speech"} {
		if strings.Contains(id, fragment) {
			return false
		}
	}
	return true
}
func (p OpenAIChatProvider) Models(ctx context.Context) ([]ModelOption, error) {
	var result struct {
		Data []struct {
			ID           string  `json:"id"`
			OwnedBy      string  `json:"owned_by"`
			ShutdownDate *string `json:"shutdown_date"`
		} `json:"data"`
	}
	if err := p.Client.fetchJSON(ctx, "/models", &result); err != nil {
		return nil, err
	}
	models := make([]ModelOption, 0, len(result.Data))
	for _, v := range result.Data {
		if !eligibleReviewModel(v.ID) {
			continue
		}
		if v.ShutdownDate != nil && *v.ShutdownDate != "" {
			continue
		}
		models = append(models, ModelOption{ID: v.ID, ProviderID: p.ID(), OwnedBy: v.OwnedBy})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	if len(models) == 0 {
		return nil, errors.New("검토용 텍스트 모델을 찾지 못했습니다")
	}
	return models, nil
}
func (p OpenAIChatProvider) History(ctx context.Context, id string, limit int) ([]ChatMessage, error) {
	if !strings.HasPrefix(id, "conv_") {
		return nil, errors.New("유효한 API 대화 ID가 아닙니다")
	}
	if limit < 1 {
		limit = 40
	}
	if limit > 100 {
		limit = 100
	}
	var response struct {
		Data []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"data"`
	}
	path := "/conversations/" + url.PathEscape(id) + "/items?order=desc&limit=" + fmt.Sprint(limit)
	if err := p.Client.fetchJSON(ctx, path, &response); err != nil {
		return nil, err
	}
	messages := make([]ChatMessage, 0, len(response.Data))
	for i := len(response.Data) - 1; i >= 0; i-- {
		v := response.Data[i]
		if v.Type != "message" || (v.Role != "user" && v.Role != "assistant") {
			continue
		}
		var parts []string
		for _, c := range v.Content {
			if (c.Type == "input_text" || c.Type == "output_text" || c.Type == "text") && strings.TrimSpace(c.Text) != "" {
				parts = append(parts, c.Text)
			}
		}
		if len(parts) > 0 {
			messages = append(messages, ChatMessage{Role: v.Role, Text: strings.Join(parts, "\n")})
		}
	}
	return messages, nil
}
