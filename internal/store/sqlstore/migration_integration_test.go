//go:build integration

package sqlstore

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/infrashift/chit/internal/model"
)

// migrationDB creates an empty database next to CHIT_DATABASE_URL's, applies
// the up migrations before version `before`, and returns a pool on it. The
// database is dropped afterwards.
func migrationDB(t *testing.T, before int) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := os.Getenv("CHIT_DATABASE_URL")
	if base == "" {
		t.Fatal("CHIT_DATABASE_URL is not set")
	}
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	name := fmt.Sprintf("chit_migtest_%d", os.Getpid())
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name)
	if _, createErr := admin.Exec(ctx, "CREATE DATABASE "+name); createErr != nil {
		t.Fatal(createErr)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	for v := 1; v < before; v++ {
		applyMigration(t, pool, v, "up")
	}
	return pool
}

func applyMigration(t *testing.T, pool *pgxpool.Pool, version int, dir string) {
	t.Helper()
	files, err := filepath.Glob(fmt.Sprintf("../../../migrations/%06d_*.%s.sql", version, dir))
	if err != nil || len(files) != 1 {
		t.Fatalf("migration %d %s: %v %v", version, dir, files, err)
	}
	sql, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
		t.Fatalf("apply %s: %v", files[0], err)
	}
}

// Migration 000008 turns each old global tag into a channel tag in every
// channel where it is applied, owned by the system actor, and drops tags
// applied nowhere.
func TestMigration000008_ScopedTags(t *testing.T) {
	ctx := context.Background()
	db := migrationDB(t, 8)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	user, team := model.NewID(), model.NewID()
	ch1, ch2 := "00000000-0000-7000-8000-0000000000c1", "00000000-0000-7000-8000-0000000000c2"
	p1, p2, p3 := model.NewID(), model.NewID(), model.NewID()
	bug, unused, solo := "00000000-0000-7000-8000-0000000000a1", model.NewID(), model.NewID()
	exec(`INSERT INTO users (id, kratos_id, username, display_name, email, create_at, update_at)
	      VALUES ($1, $2, 'u', 'U', 'u@x', 1, 1)`, user, model.NewID())
	exec(`INSERT INTO teams (id, name, display_name, creator_id, create_at, update_at) VALUES ($1, 't', 'T', $2, 1, 1)`, team, user)
	for i, ch := range []string{ch1, ch2} {
		exec(`INSERT INTO channels (id, team_id, creator_id, name, display_name, type, create_at, update_at)
		      VALUES ($1, $2, $3, $4, 'C', 'O', 1, 1)`, ch, team, user, fmt.Sprintf("c%d", i))
	}
	exec(`INSERT INTO posts (id, channel_id, user_id, content, create_at, update_at) VALUES
	      ($1, $4, $6, 'a', 1, 1), ($2, $4, $6, 'b', 1, 1), ($3, $5, $6, 'c', 1, 1)`, p1, p2, p3, ch1, ch2, user)
	twin := "00000000-0000-7000-8000-0000000000a2"
	exec(`INSERT INTO tags (id, name) VALUES ($1, 'Bug'), ($2, 'unused'), ($3, 'Straße'), ($4, 'BUG')`, bug, unused, solo, twin)
	// bug: on two posts in ch1 and one in ch2. solo: on one post in ch2.
	// BUG, the same name in another case, on p1 and p2 in ch1: it merges
	// into ch1's Bug (the lower id), and p1 keeps one application.
	exec(`INSERT INTO message_tags (message_id, tag_id) VALUES ($1, $4), ($2, $4), ($3, $4), ($3, $5), ($1, $6), ($2, $6)`, p1, p2, p3, bug, solo, twin)

	applyMigration(t, db, 8, "up")

	type row struct{ id, name, key, owner, scope, channel string }
	rows, err := db.Query(ctx, `SELECT id, name, name_key, owner_id, scope, COALESCE(channel_id::text, '') FROM tags ORDER BY name, channel_id`)
	if err != nil {
		t.Fatal(err)
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.name, &r.key, &r.owner, &r.scope, &r.channel); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	rows.Close()
	if len(got) != 3 {
		t.Fatalf("tags after the migration: %+v; want Bug in two channels and Straße in one, unused dropped", got)
	}
	for _, r := range got {
		if r.owner != model.SystemActorID || r.scope != model.TagScopeChannel {
			t.Errorf("tag %+v: want a channel tag of the system actor", r)
		}
		if r.id == unused || r.id == "00000000-0000-7000-8000-0000000000a2" {
			t.Errorf("tag %s survived: unused tags are dropped and BUG merges into Bug", r.id)
		}
	}
	if got[0].name != "Bug" || got[0].channel != ch1 || got[0].id != bug || got[0].key != "bug" {
		t.Errorf("first channel's Bug = %+v; want the old id in %s", got[0], ch1)
	}
	if got[1].name != "Bug" || got[1].channel != ch2 || got[1].id == bug {
		t.Errorf("second channel's Bug = %+v; want a new id in %s", got[1], ch2)
	}
	if got[2].key != "straße" {
		t.Errorf("Straße key = %q; the migration uses lower(), not folding", got[2].key)
	}

	// Each application points at its own channel's copy.
	tagOf := func(post string) []string {
		var ids []string
		r, err := db.Query(ctx, `SELECT tag_id::text FROM message_tags WHERE message_id = $1 ORDER BY tag_id`, post)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		for r.Next() {
			var id string
			_ = r.Scan(&id)
			ids = append(ids, id)
		}
		return ids
	}
	if ids := tagOf(p1); len(ids) != 1 || ids[0] != bug {
		t.Errorf("p1 tags %v, want [%s]", ids, bug)
	}
	if ids := tagOf(p3); len(ids) != 2 || !slices.Contains(ids, got[1].id) || !slices.Contains(ids, solo) {
		t.Errorf("p3 tags %v, want ch2's Bug and Straße", ids)
	}
	var system string
	if err := db.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, model.SystemActorID).Scan(&system); err != nil || system != "chit-system" {
		t.Errorf("system actor = %q, %v", system, err)
	}

	// The new rules hold: a channel name is unique by key.
	if _, err := db.Exec(ctx, `INSERT INTO tags (id, name, name_key, owner_id, scope, channel_id) VALUES ($1, 'BUG', 'bug', $2, 'channel', $3)`,
		model.NewID(), user, ch1); err == nil || !strings.Contains(err.Error(), "idx_tags_channel_name") {
		t.Errorf("a duplicate channel tag key was accepted: %v", err)
	}

	// Down merges the copies back into one global tag.
	applyMigration(t, db, 8, "down")
	var n int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM tags WHERE name = 'Bug'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("Bug tags after down: %d, %v; want 1", n, err)
	}
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM message_tags`).Scan(&n); err != nil || n != 4 {
		t.Errorf("applications after down: %d, %v; want all 4", n, err)
	}
}

// tagKeyCases are names on which a Go key and a SQL key could part: letters
// with special case rules, compatibility forms that NFKC rewrites, and the
// reserved prefix in disguise.
var tagKeyCases = []string{
	"Straße", "STRASSE", // ß: lowercasing keeps it, folding would make ss
	"Ｆｕｌｌｗｉｄｔｈ", "ﬁle", "Ⅻ", "K", // full width, ligature, Roman numeral, Kelvin sign
	"İstanbul", "ISTANBUL", "ıi", // Turkish dotted capital I and dotless i
	"ΣΊΣΥΦΟΣ", "σίσυφος", "ς", // Greek capital sigma, and the final sigma
	"é", "É", "É", // combining acute accent and the precomposed form
	"Ǆ", "ǅ", // a digraph with a titlecase form
	"PROFILE/Coder", "ＰＲＯＦＩＬＥ/x", "Profile/x", // the reserved prefix
}

// TestTagNameKey_AgreesWithPostgres holds the Go key (model.TagNameKey) and
// the SQL key of migration 000008 to one rule: NFKC, then simple
// lowercasing. The SQL side is the migration itself, run on these names, so
// a change to either side fails here.
func TestTagNameKey_AgreesWithPostgres(t *testing.T) {
	ctx := context.Background()
	db := migrationDB(t, 8)
	user, team := model.NewID(), model.NewID()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO users (id, kratos_id, username, display_name, email, create_at, update_at) VALUES ($1, $2, 'u', 'U', 'u@x', 1, 1)`, user, model.NewID())
	exec(`INSERT INTO teams (id, name, display_name, creator_id, create_at, update_at) VALUES ($1, 't', 'T', $2, 1, 1)`, team, user)
	// Each name in a channel of its own: names that share a key in one
	// channel merge, which TestMigration000008_ScopedTags covers.
	for i, name := range tagKeyCases {
		channel, post, id := model.NewID(), model.NewID(), model.NewID()
		exec(`INSERT INTO channels (id, team_id, creator_id, name, display_name, type, create_at, update_at) VALUES ($1, $2, $3, $4, 'C', 'O', 1, 1)`,
			channel, team, user, fmt.Sprintf("c%d", i))
		exec(`INSERT INTO posts (id, channel_id, user_id, content, create_at, update_at) VALUES ($1, $2, $3, 'p', 1, 1)`, post, channel, user)
		exec(`INSERT INTO tags (id, name) VALUES ($1, $2)`, id, name)
		exec(`INSERT INTO message_tags (message_id, tag_id) VALUES ($1, $2)`, post, id)
	}

	applyMigration(t, db, 8, "up")

	for _, name := range tagKeyCases {
		var sqlKey string
		if err := db.QueryRow(ctx, `SELECT name_key FROM tags WHERE name = $1`, name).Scan(&sqlKey); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		if goKey := model.TagNameKey(name); goKey != sqlKey {
			t.Errorf("%q: Go key %q, SQL key %q", name, goKey, sqlKey)
		}
		if strings.HasPrefix(sqlKey, model.ReservedTagPrefix) != model.IsReservedTagName(name) {
			t.Errorf("%q: the SQL key and IsReservedTagName disagree about profile/", name)
		}
	}
	for _, name := range []string{"PROFILE/Coder", "ＰＲＯＦＩＬＥ/x", "Profile/x"} {
		if !model.IsReservedTagName(name) {
			t.Errorf("%q is not refused as profile/", name)
		}
	}
	if model.TagNameKey("Straße") == model.TagNameKey("Strasse") {
		t.Error("ß and ss share a key")
	}
}
