package store

import (
	"sort"
	"time"

	"ai-workbench-api/internal/model"
)

// SaveChatSession persists a chat session to memory and MySQL.
func SaveChatSession(s *model.ChatSession) {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	s.UpdatedAt = time.Now()
	mu.Lock()
	chatSessions[s.ID] = s
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`REPLACE INTO chat_sessions (id,title,model,target_ip,created_at,updated_at) VALUES (?,?,?,?,?,?)`, s.ID, s.Title, s.Model, s.TargetIP, s.CreatedAt, s.UpdatedAt)
	} else {
		persistFallbackSnapshot()
	}
}

// ListChatSessions returns all chat sessions ordered by updated_at desc.
func ListChatSessions() []model.ChatSession {
	if mysqlOK {
		rows, err := db.Query(`SELECT id,title,model,target_ip,created_at,updated_at FROM chat_sessions ORDER BY updated_at DESC LIMIT 200`)
		if err == nil {
			defer rows.Close()
			out := []model.ChatSession{}
			for rows.Next() {
				s := model.ChatSession{}
				_ = rows.Scan(&s.ID, &s.Title, &s.Model, &s.TargetIP, &s.CreatedAt, &s.UpdatedAt)
				out = append(out, s)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	out := []model.ChatSession{}
	for _, s := range chatSessions {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

// GetChatSession retrieves a chat session with its messages.
func GetChatSession(id string) (*model.ChatSession, bool) {
	sessions := ListChatSessions()
	for _, s := range sessions {
		if s.ID == id {
			msgs := ListChatMessages(id)
			s.Messages = msgs
			return &s, true
		}
	}
	return nil, false
}

// DeleteChatSession removes a chat session and its messages.
func DeleteChatSession(id string) {
	mu.Lock()
	delete(chatSessions, id)
	delete(chatMessages, id)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM chat_messages WHERE session_id=?`, id)
		_, _ = db.Exec(`DELETE FROM chat_sessions WHERE id=?`, id)
	} else {
		persistFallbackSnapshot()
	}
}

// AddChatMessage appends a message to a chat session.
func AddChatMessage(m model.ChatMessage) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	mu.Lock()
	chatMessages[m.SessionID] = append(chatMessages[m.SessionID], m)
	if s := chatSessions[m.SessionID]; s != nil {
		s.UpdatedAt = m.CreatedAt
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`REPLACE INTO chat_messages (id,session_id,role,content,model,target_ip,created_at) VALUES (?,?,?,?,?,?,?)`, m.ID, m.SessionID, m.Role, m.Content, m.Model, m.TargetIP, m.CreatedAt)
		_, _ = db.Exec(`UPDATE chat_sessions SET updated_at=? WHERE id=?`, m.CreatedAt, m.SessionID)
	} else {
		persistFallbackSnapshot()
	}
}

// ListChatMessages returns messages for a session ordered by created_at.
func ListChatMessages(sessionID string) []model.ChatMessage {
	if mysqlOK {
		rows, err := db.Query(`SELECT id,session_id,role,content,model,target_ip,created_at FROM chat_messages WHERE session_id=? ORDER BY created_at`, sessionID)
		if err == nil {
			defer rows.Close()
			out := []model.ChatMessage{}
			for rows.Next() {
				m := model.ChatMessage{}
				_ = rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Model, &m.TargetIP, &m.CreatedAt)
				out = append(out, m)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	return append([]model.ChatMessage{}, chatMessages[sessionID]...)
}
