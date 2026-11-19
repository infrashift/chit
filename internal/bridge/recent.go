package bridge

import "github.com/infrashift/chit/internal/assert"

// recentSet is a set of strings that remembers at most its capacity, forgetting
// the oldest addition first. It is not safe for concurrent use; the bridge
// guards each one with its mutex.
type recentSet struct {
	limit int
	items map[string]struct{}
	order []string // items, oldest first
}

func newRecentSet(limit int) *recentSet {
	assert.That(limit > 0, "newRecentSet: limit not positive")
	return &recentSet{limit: limit, items: make(map[string]struct{}, limit)}
}

// add inserts s and reports whether it was absent.
func (r *recentSet) add(s string) bool {
	assert.That(r.items != nil && r.limit > 0, "recentSet.add: set not built by newRecentSet")
	if _, ok := r.items[s]; ok {
		return false
	}
	r.items[s] = struct{}{}
	r.order = append(r.order, s)
	if len(r.order) > r.limit {
		delete(r.items, r.order[0])
		r.order = r.order[1:]
	}
	assert.That(len(r.order) <= r.limit, "recentSet.add: order above the limit")
	assert.That(len(r.items) <= len(r.order), "recentSet.add: item without an order entry")
	return true
}

func (r *recentSet) has(s string) bool {
	assert.That(r.items != nil, "recentSet.has: set not built by newRecentSet")
	_, ok := r.items[s]
	return ok
}

// remove deletes s. Its slot in order is reclaimed when it ages out, so an
// item removed and added again may be forgotten early; callers use these sets
// as caches, where that only costs a lookup.
func (r *recentSet) remove(s string) {
	assert.That(r.items != nil, "recentSet.remove: set not built by newRecentSet")
	delete(r.items, s)
}

func (r *recentSet) len() int { return len(r.items) }
