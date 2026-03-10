package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/storage"
)

type Service struct {
	store *storage.Store
}

type MessageWithParts struct {
	Message storage.SessionMessage
	Parts   []storage.SessionPart
}

type MessageInput struct {
	SessionID    string
	Role         string
	ParentID     string
	ProviderID   string
	ModelID      string
	FinishReason string
	Error        string
	Text         string
}

func NewService(store *storage.Store) *Service {
	return &Service{store: store}
}

func (s *Service) AppendText(input MessageInput) (*MessageWithParts, error) {
	parts := []storage.SessionPart{}
	text := strings.TrimSpace(input.Text)
	if text != "" {
		payloadBytes, err := json.Marshal(map[string]string{"text": text})
		if err != nil {
			return nil, fmt.Errorf("serialize text payload: %w", err)
		}
		parts = append(parts, storage.SessionPart{
			Type:    "text",
			Payload: string(payloadBytes),
		})
	}

	return s.AppendMessage(input, parts)
}

func (s *Service) AppendMessage(input MessageInput, parts []storage.SessionPart) (*MessageWithParts, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("session service is not initialized")
	}

	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role == "" {
		return nil, fmt.Errorf("message role is required")
	}

	createdMsg, createdParts, err := s.store.CreateMessageWithParts(storage.SessionMessage{
		SessionID:    strings.TrimSpace(input.SessionID),
		Role:         role,
		ParentID:     strings.TrimSpace(input.ParentID),
		ProviderID:   strings.TrimSpace(input.ProviderID),
		ModelID:      strings.TrimSpace(input.ModelID),
		FinishReason: strings.TrimSpace(input.FinishReason),
		Error:        strings.TrimSpace(input.Error),
		CreatedAt:    time.Now().UTC(),
	}, parts)
	if err != nil {
		return nil, err
	}

	return &MessageWithParts{Message: createdMsg, Parts: createdParts}, nil
}

func (s *Service) ListMessagesWithParts(sessionID string, limit int) ([]MessageWithParts, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("session service is not initialized")
	}

	messages, err := s.store.ListSessionMessages(sessionID, limit)
	if err != nil {
		return nil, err
	}

	result := make([]MessageWithParts, 0, len(messages))
	for _, msg := range messages {
		parts, partErr := s.store.ListSessionParts(msg.ID)
		if partErr != nil {
			return nil, partErr
		}
		result = append(result, MessageWithParts{Message: msg, Parts: parts})
	}

	return result, nil
}

func ExtractTextPart(part storage.SessionPart) string {
	if strings.ToLower(strings.TrimSpace(part.Type)) != "text" {
		return ""
	}
	if strings.TrimSpace(part.Payload) == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(part.Payload), &payload); err != nil {
		return strings.TrimSpace(part.Payload)
	}
	if text, ok := payload["text"].(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(part.Payload)
}
