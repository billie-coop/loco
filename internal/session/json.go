package session

import (
	"encoding/json"
	"time"

	"github.com/billie-coop/loco/internal/csync"
	"github.com/billie-coop/loco/internal/llm"
)

// sessionJSON is a helper struct for JSON marshaling/unmarshaling
type sessionJSON struct {
	Created     time.Time     `json:"created"`
	LastUpdated time.Time     `json:"last_updated"`
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Team        *ModelTeam    `json:"team"`
	Messages    []llm.Message `json:"messages"`
}

// MarshalJSON implements json.Marshaler for Session
func (s *Session) MarshalJSON() ([]byte, error) {
	return json.Marshal(sessionJSON{
		Created:     s.Created,
		LastUpdated: s.LastUpdated,
		ID:          s.ID,
		Title:       s.Title,
		Team:        s.Team,
		Messages:    s.Messages.ToSlice(),
	})
}

// UnmarshalJSON implements json.Unmarshaler for Session
func (s *Session) UnmarshalJSON(data []byte) error {
	var temp sessionJSON
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	s.Created = temp.Created
	s.LastUpdated = temp.LastUpdated
	s.ID = temp.ID
	s.Title = temp.Title
	s.Team = temp.Team
	s.Messages = csync.NewSliceFrom(temp.Messages)
	
	return nil
}