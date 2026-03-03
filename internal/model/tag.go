package model

import "net/http"

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (t *Tag) IsValid() *AppError {
	if !IsValidID(t.ID) {
		return NewAppError("Tag.IsValid", "invalid tag id", "", http.StatusBadRequest)
	}
	if len(t.Name) == 0 || len(t.Name) > 50 {
		return NewAppError("Tag.IsValid", "name must be 1–50 characters", "", http.StatusBadRequest)
	}
	return nil
}

func (t *Tag) PreSave() {
	if t.ID == "" {
		t.ID = NewID()
	}
}

type MessageTag struct {
	MessageID string `json:"message_id"`
	TagID     string `json:"tag_id"`
}
