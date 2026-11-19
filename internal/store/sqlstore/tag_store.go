package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

type SqlTagStore struct {
	sqlStore *SqlStore
}

const tagColumns = `t.id, t.name, t.scope, t.owner_id, COALESCE(t.channel_id::text, ''), t.name_key, t.create_at, t.update_at`

func scanTag(row pgx.Row) (*model.Tag, error) {
	assert.That(row != nil, "scanTag: nil row")
	t := &model.Tag{}
	if err := row.Scan(&t.ID, &t.Name, &t.Scope, &t.OwnerID, &t.ChannelID, &t.NameKey, &t.CreateAt, &t.UpdateAt); err != nil {
		return nil, err
	}
	return t, nil
}

func collectTags(rows pgx.Rows) ([]*model.Tag, error) {
	assert.That(rows != nil, "collectTags: nil rows")
	defer rows.Close()
	var tags []*model.Tag
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Save inserts a new Tag. A Tag with the same key for the same owner
// (private) or channel (channel) is a conflict.
func (s *SqlTagStore) Save(ctx context.Context, tag *model.Tag) (*model.Tag, error) {
	assert.That(tag != nil, "SqlTagStore.Save: nil tag")
	assert.That(s.sqlStore != nil, "SqlTagStore.Save: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.Save: nil pool")
	tag.PreSave()
	if err := tag.IsValid(); err != nil {
		return nil, err
	}
	assert.That(tag.ID != "" && tag.UpdateAt > 0, "SqlTagStore.Save: PreSave left no id or time")
	_, err := s.sqlStore.pool.Exec(ctx,
		`INSERT INTO tags (id, name, name_key, owner_id, scope, channel_id, create_at, update_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tag.ID, tag.Name, tag.NameKey, tag.OwnerID, tag.Scope, nullable(tag.ChannelID), tag.CreateAt, tag.UpdateAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.NewConflictError("SqlTagStore.Save", "a tag with this name already exists")
		}
		return nil, fmt.Errorf("save tag: %w", err)
	}
	return tag, nil
}

func (s *SqlTagStore) Get(ctx context.Context, id string) (*model.Tag, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.Get: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.Get: nil pool")
	if !validIDs(id) {
		return nil, model.NewNotFoundError("SqlTagStore.Get", id)
	}
	t, err := scanTag(s.sqlStore.pool.QueryRow(ctx, `SELECT `+tagColumns+` FROM tags t WHERE t.id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlTagStore.Get", id)
		}
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return t, nil
}

func (s *SqlTagStore) GetByIDs(ctx context.Context, ids []string) ([]*model.Tag, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.GetByIDs: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.GetByIDs: nil pool")
	if len(ids) == 0 || !validIDs(ids...) {
		return nil, nil
	}
	rows, err := s.sqlStore.pool.Query(ctx, `SELECT `+tagColumns+` FROM tags t WHERE t.id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("get tags by ids: %w", err)
	}
	return collectTags(rows)
}

// GetByKey finds the Tag with this key in a scope: by owner for private
// Tags, by channel for channel Tags.
func (s *SqlTagStore) GetByKey(ctx context.Context, scope, ownerID, channelID, nameKey string) (*model.Tag, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.GetByKey: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.GetByKey: nil pool")
	assert.That(scope == model.TagScopePrivate || scope == model.TagScopeChannel, "SqlTagStore.GetByKey: unknown scope")
	assert.That(scope != model.TagScopeChannel || channelID != "", "SqlTagStore.GetByKey: channel scope without a channel")
	var row pgx.Row
	if scope == model.TagScopeChannel {
		row = s.sqlStore.pool.QueryRow(ctx,
			`SELECT `+tagColumns+` FROM tags t WHERE t.scope = 'channel' AND t.channel_id = $1 AND t.name_key = $2`, channelID, nameKey)
	} else {
		row = s.sqlStore.pool.QueryRow(ctx,
			`SELECT `+tagColumns+` FROM tags t WHERE t.scope = 'private' AND t.owner_id = $1 AND t.name_key = $2`, ownerID, nameKey)
	}
	t, err := scanTag(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlTagStore.GetByKey", nameKey)
		}
		return nil, fmt.Errorf("get tag by key: %w", err)
	}
	return t, nil
}

// Update renames a Tag.
func (s *SqlTagStore) Update(ctx context.Context, tag *model.Tag) (*model.Tag, error) {
	assert.That(tag != nil, "SqlTagStore.Update: nil tag")
	assert.That(s.sqlStore != nil, "SqlTagStore.Update: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.Update: nil pool")
	tag.Name = model.NormalizeTagName(tag.Name)
	tag.NameKey = model.TagNameKey(tag.Name)
	tag.UpdateAt = model.GetMillis()
	if err := tag.IsValid(); err != nil {
		return nil, err
	}
	ct, err := s.sqlStore.pool.Exec(ctx,
		`UPDATE tags SET name = $1, name_key = $2, update_at = $3 WHERE id = $4`,
		tag.Name, tag.NameKey, tag.UpdateAt, tag.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.NewConflictError("SqlTagStore.Update", "a tag with this name already exists")
		}
		return nil, fmt.Errorf("update tag: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return nil, model.NewNotFoundError("SqlTagStore.Update", tag.ID)
	}
	return tag, nil
}

// Delete removes a Tag and, by cascade, every application of it.
func (s *SqlTagStore) Delete(ctx context.Context, id string) error {
	assert.That(s.sqlStore != nil, "SqlTagStore.Delete: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.Delete: nil pool")
	if _, err := s.sqlStore.pool.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}

// DeleteByOwner removes every Tag ownerID owns, with all their applications,
// and forgets ownerID as the applier of other owners' Tags.
func (s *SqlTagStore) DeleteByOwner(ctx context.Context, ownerID string) error {
	assert.That(s.sqlStore != nil, "SqlTagStore.DeleteByOwner: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.DeleteByOwner: nil pool")
	tx, err := s.sqlStore.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete tags by owner: %w", err)
	}
	defer rollback(ctx, tx)
	for _, q := range []string{
		`DELETE FROM tags WHERE owner_id = $1`,
		`UPDATE message_tags SET applied_by = NULL WHERE applied_by = $1`,
		`UPDATE channel_tags SET applied_by = NULL WHERE applied_by = $1`,
	} {
		if _, err := tx.Exec(ctx, q, ownerID); err != nil {
			return fmt.Errorf("delete tags by owner: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (s *SqlTagStore) CountByOwner(ctx context.Context, ownerID string) (int, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.CountByOwner: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.CountByOwner: nil pool")
	var n int
	if err := s.sqlStore.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tags WHERE owner_id = $1`, ownerID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count tags: %w", err)
	}
	return n, nil
}

// GetVisible lists the Tags userID can see: their own private Tags, and the
// channel Tags of channelIDs (the channels userID belongs to).
func (s *SqlTagStore) GetVisible(ctx context.Context, userID string, channelIDs []string) ([]*model.Tag, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.GetVisible: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.GetVisible: nil pool")
	if channelIDs == nil {
		channelIDs = []string{}
	}
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT `+tagColumns+` FROM tags t
		 WHERE (t.scope = 'private' AND t.owner_id = $1)
		    OR (t.scope = 'channel' AND t.channel_id = ANY($2))
		 ORDER BY t.name_key, t.id`, userID, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("get visible tags: %w", err)
	}
	return collectTags(rows)
}

func (s *SqlTagStore) AddTagToPost(ctx context.Context, messageID, tagID, appliedBy string) error {
	assert.That(s.sqlStore != nil, "SqlTagStore.AddTagToPost: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.AddTagToPost: nil pool")
	_, err := s.sqlStore.pool.Exec(ctx,
		`INSERT INTO message_tags (message_id, tag_id, applied_by, create_at) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		messageID, tagID, nullable(appliedBy), model.GetMillis())
	if err != nil {
		return fmt.Errorf("add tag to post: %w", err)
	}
	return nil
}

func (s *SqlTagStore) RemoveTagFromPost(ctx context.Context, messageID, tagID string) error {
	assert.That(s.sqlStore != nil, "SqlTagStore.RemoveTagFromPost: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.RemoveTagFromPost: nil pool")
	if _, err := s.sqlStore.pool.Exec(ctx, `DELETE FROM message_tags WHERE message_id = $1 AND tag_id = $2`, messageID, tagID); err != nil {
		return fmt.Errorf("remove tag from post: %w", err)
	}
	return nil
}

// GetTagsForPost returns every Tag on a post, of every owner and scope. The
// caller filters them for the reader.
func (s *SqlTagStore) GetTagsForPost(ctx context.Context, messageID string) ([]*model.Tag, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.GetTagsForPost: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.GetTagsForPost: nil pool")
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT `+tagColumns+` FROM tags t JOIN message_tags mt ON t.id = mt.tag_id
		 WHERE mt.message_id = $1 ORDER BY t.name_key, t.id`, messageID)
	if err != nil {
		return nil, fmt.Errorf("get tags for post: %w", err)
	}
	return collectTags(rows)
}

// GetTagsForPosts returns every Tag on many posts in one query, keyed by post
// ID. Posts with no tags are absent from the map. Like GetTagsForPost, it
// does not filter for a reader.
//
// It exists because the client needs the tags of a whole page of history at
// once; asking per post issued sixty round trips per channel open, enough to
// trip the server's own rate limit.
func (s *SqlTagStore) GetTagsForPosts(ctx context.Context, messageIDs []string) (map[string][]*model.Tag, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.GetTagsForPosts: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.GetTagsForPosts: nil pool")
	result := make(map[string][]*model.Tag)
	if len(messageIDs) == 0 {
		return result, nil
	}
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT mt.message_id, `+tagColumns+` FROM tags t JOIN message_tags mt ON t.id = mt.tag_id
		 WHERE mt.message_id = ANY($1) ORDER BY mt.message_id, t.name_key, t.id`, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("get tags for posts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var postID string
		t := &model.Tag{}
		if err := rows.Scan(&postID, &t.ID, &t.Name, &t.Scope, &t.OwnerID, &t.ChannelID, &t.NameKey, &t.CreateAt, &t.UpdateAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		result[postID] = append(result[postID], t)
	}
	return result, rows.Err()
}

func (s *SqlTagStore) AddTagToChannel(ctx context.Context, channelID, tagID, appliedBy string) error {
	assert.That(s.sqlStore != nil, "SqlTagStore.AddTagToChannel: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.AddTagToChannel: nil pool")
	_, err := s.sqlStore.pool.Exec(ctx,
		`INSERT INTO channel_tags (channel_id, tag_id, applied_by, create_at) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		channelID, tagID, nullable(appliedBy), model.GetMillis())
	if err != nil {
		return fmt.Errorf("add tag to channel: %w", err)
	}
	return nil
}

func (s *SqlTagStore) RemoveTagFromChannel(ctx context.Context, channelID, tagID string) error {
	assert.That(s.sqlStore != nil, "SqlTagStore.RemoveTagFromChannel: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.RemoveTagFromChannel: nil pool")
	if _, err := s.sqlStore.pool.Exec(ctx, `DELETE FROM channel_tags WHERE channel_id = $1 AND tag_id = $2`, channelID, tagID); err != nil {
		return fmt.Errorf("remove tag from channel: %w", err)
	}
	return nil
}

// GetTagsForChannel returns every Tag on a channel itself (not on its
// posts), of every owner and scope. The caller filters them for the reader.
func (s *SqlTagStore) GetTagsForChannel(ctx context.Context, channelID string) ([]*model.Tag, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.GetTagsForChannel: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.GetTagsForChannel: nil pool")
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT `+tagColumns+` FROM tags t JOIN channel_tags ct ON t.id = ct.tag_id
		 WHERE ct.channel_id = $1 ORDER BY t.name_key, t.id`, channelID)
	if err != nil {
		return nil, fmt.Errorf("get tags for channel: %w", err)
	}
	return collectTags(rows)
}

func (s *SqlTagStore) FilterPostIDsByTags(ctx context.Context, postIDs, tagIDs []string) ([]string, error) {
	assert.That(len(postIDs) > 0, "SqlTagStore.FilterPostIDsByTags: no posts")
	assert.That(len(tagIDs) > 0, "SqlTagStore.FilterPostIDsByTags: no tags")
	query := `SELECT mt.message_id FROM message_tags mt
		WHERE mt.message_id = ANY($1) AND mt.tag_id = ANY($2)
		GROUP BY mt.message_id
		HAVING COUNT(DISTINCT mt.tag_id) = $3`
	return s.ids(ctx, "filter post ids by tags", query, postIDs, tagIDs, len(tagIDs))
}

// FilterChannelIDsByTags keeps the channels that have every Tag in tagIDs,
// each on the channel itself or on one of its live posts.
func (s *SqlTagStore) FilterChannelIDsByTags(ctx context.Context, channelIDs, tagIDs []string) ([]string, error) {
	assert.That(len(channelIDs) > 0, "SqlTagStore.FilterChannelIDsByTags: no channels")
	assert.That(len(tagIDs) > 0, "SqlTagStore.FilterChannelIDsByTags: no tags")
	query := `SELECT channel_id FROM (
			SELECT ct.channel_id, ct.tag_id FROM channel_tags ct
			WHERE ct.channel_id = ANY($1) AND ct.tag_id = ANY($2)
			UNION
			SELECT p.channel_id, mt.tag_id FROM message_tags mt JOIN posts p ON p.id = mt.message_id
			WHERE p.channel_id = ANY($1) AND mt.tag_id = ANY($2) AND p.delete_at = 0
		) hits
		GROUP BY channel_id
		HAVING COUNT(DISTINCT tag_id) = $3`
	return s.ids(ctx, "filter channel ids by tags", query, channelIDs, tagIDs, len(tagIDs))
}

func (s *SqlTagStore) ids(ctx context.Context, what, query string, args ...any) ([]string, error) {
	assert.That(s.sqlStore != nil, "SqlTagStore.ids: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlTagStore.ids: nil pool")
	assert.That(what != "", "SqlTagStore.ids: empty description")
	assert.That(query != "", "SqlTagStore.ids: empty query")
	rows, err := s.sqlStore.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
