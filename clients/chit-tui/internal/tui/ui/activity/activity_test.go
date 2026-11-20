package activity_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/activity"
)

func TestActivity_SetRefreshAndClear(t *testing.T) {
	m := activity.New(testutil.Styles(), 4, time.Second)
	if m.View(80) != "" {
		t.Error("an empty set drew something")
	}
	m.Set("a", "agent is responding…")
	m.Set("b", "bot is responding…")
	m.Set("a", "agent is still responding…")
	if !slices.Equal(m.Labels(), []string{"agent is still responding…", "bot is responding…"}) {
		t.Errorf("Labels = %v", m.Labels())
	}
	if !m.Clear("a") || m.Clear("a") || m.Len() != 1 {
		t.Error("Clear did not remove a once")
	}
	m.ClearAll()
	if m.Len() != 0 {
		t.Error("ClearAll left items")
	}
}

func TestActivity_CapDropsTheOldest(t *testing.T) {
	m := activity.New(testutil.Styles(), 2, time.Second)
	m.Set("a", "A")
	m.Set("b", "B")
	m.Set("c", "C")
	if !slices.Equal(m.Labels(), []string{"B", "C"}) {
		t.Errorf("Labels = %v, want [B C]", m.Labels())
	}
}

func TestActivity_ExpireOnlyTheLatestTimer(t *testing.T) {
	m := activity.New(testutil.Styles(), 4, time.Millisecond)
	old, ok := m.Set("a", "A")().(activity.ExpiredMsg)
	if !ok {
		t.Fatal("Set sent no ExpiredMsg")
	}
	latest, ok := m.Set("a", "A again")().(activity.ExpiredMsg)
	if !ok {
		t.Fatal("Set sent no ExpiredMsg")
	}
	if m.Expire(old) || m.Len() != 1 {
		t.Error("the timer of a refreshed item removed it")
	}
	if !m.Expire(latest) || m.Len() != 0 {
		t.Error("the latest timer did not remove the item")
	}
	if m.Expire(latest) {
		t.Error("Expire removed an item twice")
	}
}

func TestActivity_ViewCutsToWidth(t *testing.T) {
	m := activity.New(testutil.Styles(), 4, time.Second)
	m.SetStyles(testutil.Styles())
	m.Set("a", "chit-agent is responding…")
	m.Set("b", "bot is responding…")
	full := testutil.StripANSI(m.View(200))
	if full != "chit-agent is responding… · bot is responding…" {
		t.Errorf("View = %q", full)
	}
	cut := testutil.StripANSI(m.View(12))
	if len([]rune(cut)) > 12 || !strings.HasSuffix(cut, "…") {
		t.Errorf("cut View = %q", cut)
	}
	if m.View(0) != "" {
		t.Error("a zero width drew something")
	}
}

func TestActivity_ClearPrefix(t *testing.T) {
	m := activity.New(testutil.Styles(), 4, time.Second)
	m.Set("u1/a", "A")
	m.Set("u2/b", "B")
	m.Set("u1/c", "C")
	if n := m.ClearPrefix("u1/"); n != 2 || !slices.Equal(m.Labels(), []string{"B"}) {
		t.Errorf("ClearPrefix removed %d, left %v", n, m.Labels())
	}
	if n := m.ClearPrefix("zz"); n != 0 {
		t.Errorf("ClearPrefix of no match removed %d", n)
	}
}
