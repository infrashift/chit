package model

// Command mirrors the server's Command model.
type Command struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
