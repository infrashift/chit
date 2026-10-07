package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// tagByName finds a known tag, ignoring case.
func (m Model) tagByName(name string) *model.Tag {
	for _, t := range m.allTags {
		if strings.EqualFold(t.Name, name) {
			return t
		}
	}
	return nil
}

// tagPost applies tags by name to a post, creating any that do not exist yet.
func (m Model) tagPost(postID string, names []string) []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(names))
	for _, name := range names {
		if t := m.tagByName(name); t != nil {
			cmds = append(cmds, AddTagToPostCmd(m.reqCtx(), m.client, postID, t.ID))
		} else {
			cmds = append(cmds, CreateTagAndApplyCmd(m.reqCtx(), m.client, name, postID))
		}
	}
	return cmds
}
