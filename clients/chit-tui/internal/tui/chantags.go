package tui

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/chips"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/infobox"
)

// Channel tags (ISSUE-022): a strip of chips on the upper border of the
// main pane, /chtag add, rm and find, and the list of found channels.

const (
	// maxChannelTags is the most tags that one channel carries.
	maxChannelTags = 32
	// subjectRemoveChannelTag prefixes "<channel ID>/<tag ID>".
	subjectRemoveChannelTag = "remove-chtag:"
	// subjectFoundChannels names the list of found channels.
	subjectFoundChannels = "found-channels"
	chtagUsage           = "use /chtag add <name>, /chtag rm <name> or /chtag find <name>…"
)

// routeChannelTagMsg routes the messages of the channel tags.
func (m Model) routeChannelTagMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "routeChannelTagMsg: not signed in")
	assert.That(m.client != nil && m.scope != nil, "routeChannelTagMsg: no client or request scope")
	switch msg := msg.(type) {
	case ChannelTagsLoadedMsg:
		if msg.Err == nil && msg.ChannelID == m.activeChannelID() {
			m.setChannelTags(msg.Tags)
		}
		return m, nil, true
	case ChannelTagListLoadedMsg:
		m.learnTags(msg.Tags)
		return m, nil, true
	case ChannelTagChangedMsg:
		return handled(m.handleChannelTagChanged(msg))
	case ChannelsFoundMsg:
		return handled(m.handleChannelsFound(msg))
	case infobox.ActionMsg:
		return handled(m.handleInfoAction(msg))
	}
	return m, nil, false
}

// setChannelTags shows tags as the tags of the open channel.
func (m *Model) setChannelTags(tags []*model.Tag) {
	assert.That(m.chanChips.Len() <= maxChannelTags, "setChannelTags: chips over the cap before the set")
	m.channelTags = tags[:min(len(tags), maxChannelTags)]
	cs := make([]chips.Chip, 0, len(m.channelTags))
	for _, t := range m.channelTags {
		cs = append(cs, chips.Chip{ID: t.ID, Label: "#" + t.Name})
	}
	m.chanChips.Set(cs)
	assert.That(m.chanChips.Len() <= maxChannelTags, "setChannelTags: chips over the cap")
}

// learnTags adds the tags that the user did not know yet, such as the
// channel tags that other members made.
func (m *Model) learnTags(tags []*model.Tag) {
	assert.That(m.appState == AppStateRunning, "learnTags: not signed in")
	for _, t := range tags {
		if t != nil && m.tagByID(t.ID) == nil {
			m.allTags = append(m.allTags, t)
		}
	}
	assert.That(len(m.allTags) >= len(tags) || len(tags) > 0, "learnTags: tags lost")
}

// onChannelTagsUpdated shows the new tags of the open channel.
func (m *Model) onChannelTagsUpdated(evt model.WebSocketEvent) {
	assert.That(m.appState == AppStateRunning, "onChannelTagsUpdated: not signed in")
	assert.That(evt.Event == model.WebSocketEventChannelTagsUpdated, "onChannelTagsUpdated: another event")
	if dataString(evt.Data, "channel_id") != m.activeChannelID() || m.activeChan == nil {
		return
	}
	var tags []*model.Tag
	if raw, ok := evt.Data["tags"]; ok && raw != nil {
		if err := reDecode(raw, &tags); err != nil {
			slog.Warn("could not decode a channel_tags_updated payload", "error", err)
			return
		}
	}
	m.learnTags(tags)
	m.setChannelTags(tags)
}

// placeChannelTags draws the tags of the open channel on the upper border of
// the main pane.
func (m Model) placeChannelTags(layout string) string {
	assert.That(layout != "", "placeChannelTags: empty layout")
	if m.chanChips.Len() == 0 || m.width < 12 {
		return layout
	}
	line := ansi.Truncate(m.chanChips.View(), m.width-8, "…")
	assert.That(!strings.Contains(line, "\n"), "placeChannelTags: chips on more than one line")
	return placeOverlay(3, 0, " "+line+" ", layout)
}

// chtagCommand runs /chtag add, rm and find.
func (m Model) chtagCommand(args string) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "chtagCommand: not signed in")
	assert.That(m.client != nil && m.scope != nil, "chtagCommand: no client or request scope")
	verb, rest, hasArgs := strings.Cut(strings.TrimSpace(args), " ")
	names := strings.Fields(rest)
	for i, n := range names {
		names[i] = strings.TrimPrefix(n, "#")
	}
	if verb == "find" && len(names) > 0 {
		return m.findChannels(names)
	}
	if m.activeChan == nil || !hasArgs || len(names) != 1 || (verb != "add" && verb != "rm") {
		return m, m.setError(errors.New(chtagUsage))
	}
	if verb == "add" {
		known := m.searchTagByName(names[0], m.activeChan.ID)
		return m, AddChannelTagCmd(m.reqCtx(), m.client, m.activeChan.ID, names[0], known)
	}
	return m.askRemoveChannelTag(names[0])
}

// askRemoveChannelTag asks before it takes a tag off the open channel.
func (m Model) askRemoveChannelTag(name string) (tea.Model, tea.Cmd) {
	assert.That(name != "", "askRemoveChannelTag: empty name")
	assert.That(m.activeChan != nil, "askRemoveChannelTag: no open channel")
	for _, t := range m.channelTags {
		if strings.EqualFold(t.Name, name) {
			q := fmt.Sprintf("remove #%s from this channel? Press y to confirm, any other key to cancel", t.Name)
			subject := subjectRemoveChannelTag + m.activeChan.ID + "/" + t.ID
			return m, m.confirm.Ask(subject, q, key.NewBinding(key.WithKeys("y")))
		}
	}
	return m, m.setError(fmt.Errorf("#%s is not on this channel", name))
}

// removeConfirmedChannelTag takes the tag off the channel, after a yes.
func (m *Model) removeConfirmedChannelTag(ref string) (tea.Cmd, bool) {
	assert.That(ref != "", "removeConfirmedChannelTag: empty reference")
	assert.That(m.client != nil && m.scope != nil, "removeConfirmedChannelTag: no client or request scope")
	channelID, tagID, ok := strings.Cut(ref, "/")
	if !ok || channelID == "" || tagID == "" {
		return nil, false
	}
	return RemoveChannelTagCmd(m.reqCtx(), m.client, channelID, tagID), true
}

// findChannels finds the channels that carry each of the tags names.
func (m Model) findChannels(names []string) (tea.Model, tea.Cmd) {
	assert.That(len(names) > 0, "findChannels: no names")
	if len(names) > model.MaxSearchTags {
		return m, m.setError(fmt.Errorf("find takes at most %d tags", model.MaxSearchTags))
	}
	ids := make([]string, 0, len(names))
	for _, n := range names {
		t := m.searchTagByName(n, m.activeChannelID())
		if t == nil {
			return m, m.setError(fmt.Errorf("no tag #%s", n))
		}
		ids = append(ids, t.ID)
	}
	assert.That(len(ids) == len(names), "findChannels: an ID for each name")
	return m, FindChannelsCmd(m.reqCtx(), m.client, ids)
}

// handleChannelTagChanged learns a tag that an add made, and loads the tags
// of the channel again.
func (m Model) handleChannelTagChanged(msg ChannelTagChangedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleChannelTagChanged: not signed in")
	assert.That(msg.ChannelID != "", "handleChannelTagChanged: no channel")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	if msg.Tag != nil {
		m.learnTags([]*model.Tag{msg.Tag})
	}
	if msg.ChannelID != m.activeChannelID() {
		return m, nil
	}
	return m, FetchChannelTags(m.reqCtx(), m.client, msg.ChannelID)
}

// handleChannelsFound lists the found channels in the info box. A digit
// key opens one.
func (m Model) handleChannelsFound(msg ChannelsFoundMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleChannelsFound: not signed in")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	m.foundChannels = nil
	var rows []infobox.Row
	for _, tc := range msg.Results {
		if tc == nil || tc.Channel == nil || len(m.foundChannels) >= 9 {
			continue
		}
		m.foundChannels = append(m.foundChannels, tc.Channel)
		rows = append(rows, infobox.Row{Label: strconv.Itoa(len(m.foundChannels)), Value: tc.Channel.DisplayName + "  " + strings.Join(hashed(tc.Tags), " ")})
	}
	if len(rows) == 0 {
		return m, m.setError(errors.New("no channel of yours carries those tags"))
	}
	m.infoBox.Open(subjectFoundChannels, "Channels with the tags", rows, foundActions(len(rows)))
	assert.That(len(rows) == len(m.foundChannels), "handleChannelsFound: a row for each channel")
	return m, nil
}

// foundActions gives the digit keys 1 to n; only the first has a hint.
func foundActions(n int) []infobox.Action {
	assert.That(n >= 1 && n <= 9, "foundActions: 1 to 9 channels")
	actions := make([]infobox.Action, n)
	for i := range n {
		actions[i] = infobox.Action{Key: strconv.Itoa(i + 1)}
	}
	actions[0].Label = "opens the channel"
	if n > 1 {
		actions[0].Label = fmt.Sprintf("to %d opens a channel", n)
	}
	return actions
}

// handleInfoAction opens the found channel of a digit key.
func (m Model) handleInfoAction(msg infobox.ActionMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Subject != "" && msg.Key != "", "handleInfoAction: empty subject or key")
	assert.That(len(m.foundChannels) <= 9, "handleInfoAction: more found channels than digit keys")
	if msg.Subject != subjectFoundChannels {
		return m, nil
	}
	i, err := strconv.Atoi(msg.Key)
	if err != nil || i < 1 || i > len(m.foundChannels) {
		return m, nil
	}
	m.infoBox.Close()
	return m.handleChannelChosen(palette.ChannelChosenMsg{Channel: m.foundChannels[i-1]})
}

// hashed returns the names of tags, each with its "#".
func hashed(tags []*model.Tag) []string {
	assert.That(len(tags) <= maxChannelTags*2, "hashed: more tags than a channel can carry")
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, "#"+t.Name)
	}
	assert.That(len(out) == len(tags), "hashed: a name for each tag")
	return out
}
