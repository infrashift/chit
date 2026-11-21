package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/post"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/infobox"
)

// The post detail overlay (ui/infobox): what a post holds beyond its text.
// "i" opens it on the post under the cursor, or on the root of the open
// thread.

// subjectPostInfo prefixes the post ID in the subject of the overlay.
const subjectPostInfo = "post:"

// selectedPost returns the post that a key in the focused pane acts on.
func (m Model) selectedPost() *model.Post {
	assert.That(m.focus >= FocusViewport && m.focus <= FocusThreadInbox, "selectedPost: unknown focus")
	assert.That(m.appState == AppStateRunning, "selectedPost: not signed in")
	switch m.focus {
	case FocusViewport:
		return m.viewport.SelectedPost()
	case FocusThread:
		return m.thread.RootPost()
	}
	return nil
}

// openPostInfo opens the overlay on the selected post. It reports false,
// and the key goes on, when no post is selected.
func (m *Model) openPostInfo() (tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "openPostInfo: not signed in")
	p := m.selectedPost()
	if p == nil {
		return nil, false
	}
	m.infoPost = p
	m.infoBox.Open(subjectPostInfo+p.ID, "Post details", m.postInfoRows(p), m.postInfoActions(p))
	assert.That(m.infoBox.Visible(), "openPostInfo: overlay not open")
	return nil, true
}

// postInfoActions are the keys that the overlay offers for p.
func (m Model) postInfoActions(p *model.Post) []infobox.Action {
	assert.That(p != nil, "postInfoActions: nil post")
	assert.That(len(p.FileIDs) <= model.MaxPostFiles, "postInfoActions: more files than a post can carry")
	if len(p.FileIDs) == 0 {
		return nil
	}
	return []infobox.Action{{Key: "s", Label: "save the files to " + m.downloadDir}}
}

// postInfoRows lists the facts of p: the author, the client that posted
// for the author, the times, the provenance and the files.
func (m Model) postInfoRows(p *model.Post) []infobox.Row {
	assert.That(p != nil, "postInfoRows: nil post")
	dir := post.Directory{Names: m.usernameMap(), Kinds: m.kindMap(), Files: m.files}
	author := "@" + dir.Username(p.UserID)
	if kind := dir.Kinds[p.UserID]; kind != "" {
		author += " (" + kind + ")"
	}
	rows := []infobox.Row{{Label: "Author", Value: author}}
	if actor := p.OnBehalfOf(); actor != "" {
		rows = append(rows, infobox.Row{Label: "Posted by", Value: "@" + dir.Username(actor) + " on behalf of the author"})
	}
	rows = append(rows, infobox.Row{Label: "Time", Value: model.MillisToTime(p.CreateAt).Format(time.DateTime)})
	if p.EditAt > 0 {
		rows = append(rows, infobox.Row{Label: "Edited", Value: model.MillisToTime(p.EditAt).Format(time.DateTime)})
	}
	rows = append(rows, provenanceRows(p)...)
	for i, id := range p.FileIDs {
		label := ""
		if i == 0 {
			label = "Files"
		}
		rows = append(rows, infobox.Row{Label: label, Value: post.FileLine(id, m.files[id])})
	}
	assert.That(len(rows) >= 2, "postInfoRows: author or time missing")
	return rows
}

// provenanceRows lists what produced an agent's message.
func provenanceRows(p *model.Post) []infobox.Row {
	assert.That(p != nil, "provenanceRows: nil post")
	pv, ok := p.Provenance()
	if !ok {
		return nil
	}
	rows := []infobox.Row{{Label: "Provenance", Value: "set by the agent"}}
	if pv.Model != "" {
		rows = append(rows, infobox.Row{Label: "Model", Value: pv.Model})
	}
	rows = append(rows, listRows("Tools", pv.ToolIDs)...)
	rows = append(rows, listRows("OKF namespaces", pv.OKFNamespaceIDs)...)
	if pv.OKFBlobSHA256 != "" {
		rows = append(rows, infobox.Row{Label: "OKF blob", Value: pv.OKFBlobSHA256})
	}
	rows = append(rows, listRows("Attachments", pv.AttachmentIDs)...)
	assert.That(len(rows) >= 1, "provenanceRows: header row missing")
	return rows
}

// listRows gives one row for each value, with the label on the first.
func listRows(label string, values []string) []infobox.Row {
	assert.That(label != "", "listRows: empty label")
	rows := make([]infobox.Row, 0, len(values))
	for i, v := range values {
		l := ""
		if i == 0 {
			l = label
		}
		rows = append(rows, infobox.Row{Label: l, Value: v})
	}
	assert.That(len(rows) == len(values), "listRows: a row for each value")
	return rows
}
