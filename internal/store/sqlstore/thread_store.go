package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

type SqlThreadStore struct {
	sqlStore *SqlStore
}

func (s *SqlThreadStore) SaveOrUpdate(ctx context.Context, thread *model.Thread) error {
	assert.That(thread != nil, "SqlThreadStore.SaveOrUpdate: nil thread")
	assert.That(s.sqlStore != nil, "SqlThreadStore.SaveOrUpdate: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.SaveOrUpdate: nil pool")
	if err := thread.IsValid(); err != nil {
		return err
	}

	// DO NOTHING: this only ensures the thread row exists. Reply counters,
	// last_reply_at, and participants are advanced atomically by
	// IncrementReplyCount; overwriting them here would reset existing threads.
	query := `INSERT INTO threads (post_id, channel_id, reply_count, last_reply_at, participants)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (post_id) DO NOTHING`

	_, err := s.sqlStore.pool.Exec(ctx, query,
		thread.PostID, thread.ChannelID, thread.ReplyCount, thread.LastReplyAt, thread.Participants,
	)
	if err != nil {
		return fmt.Errorf("save or update thread: %w", err)
	}

	return nil
}

func (s *SqlThreadStore) Get(ctx context.Context, postID string) (*model.Thread, error) {
	assert.That(s.sqlStore != nil, "SqlThreadStore.Get: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.Get: nil pool")
	query := `SELECT post_id, channel_id, reply_count, last_reply_at, participants
		FROM threads WHERE post_id = $1`

	t := &model.Thread{}
	err := s.sqlStore.pool.QueryRow(ctx, query, postID).Scan(
		&t.PostID, &t.ChannelID, &t.ReplyCount, &t.LastReplyAt, &t.Participants,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlThreadStore.Get", postID)
		}
		return nil, fmt.Errorf("get thread: %w", err)
	}

	return t, nil
}

func (s *SqlThreadStore) IncrementReplyCount(ctx context.Context, postID string, timestamp int64, userID string) error {
	assert.That(s.sqlStore != nil, "SqlThreadStore.IncrementReplyCount: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.IncrementReplyCount: nil pool")
	assert.That(timestamp > 0, "SqlThreadStore.IncrementReplyCount: timestamp is not a time")
	// Atomically increment reply_count, update last_reply_at, and add participant if not present.
	query := `UPDATE threads SET
		reply_count = reply_count + 1,
		last_reply_at = GREATEST(last_reply_at, $1),
		participants = CASE
			WHEN NOT participants ? $2 THEN participants || to_jsonb($2::text)
			ELSE participants
		END
		WHERE post_id = $3`

	_, err := s.sqlStore.pool.Exec(ctx, query, timestamp, userID, postID)
	if err != nil {
		return fmt.Errorf("increment reply count: %w", err)
	}

	return nil
}

func (s *SqlThreadStore) DecrementReplyCount(ctx context.Context, postID string) error {
	assert.That(s.sqlStore != nil, "SqlThreadStore.DecrementReplyCount: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.DecrementReplyCount: nil pool")
	assert.That(postID != "", "SqlThreadStore.DecrementReplyCount: empty root id")
	query := `UPDATE threads SET
		reply_count = GREATEST(reply_count - 1, 0),
		last_reply_at = COALESCE(
			(SELECT MAX(create_at) FROM posts WHERE root_id = $1 AND delete_at = 0), 0)
		WHERE post_id = $1`
	if _, err := s.sqlStore.pool.Exec(ctx, query, postID); err != nil {
		return fmt.Errorf("decrement reply count: %w", err)
	}
	return nil
}

func (s *SqlThreadStore) IncrementMentionCounts(ctx context.Context, postID string, userIDs []string) error {
	assert.That(s.sqlStore != nil, "SqlThreadStore.IncrementMentionCounts: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.IncrementMentionCounts: nil pool")
	assert.That(postID != "", "SqlThreadStore.IncrementMentionCounts: empty root id")
	assert.That(len(userIDs) > 0, "SqlThreadStore.IncrementMentionCounts: no users")
	query := `UPDATE thread_memberships SET unread_mention_count = unread_mention_count + 1
		WHERE post_id = $1 AND user_id = ANY($2::uuid[]) AND following = TRUE`
	if _, err := s.sqlStore.pool.Exec(ctx, query, postID, userIDs); err != nil {
		return fmt.Errorf("increment thread mention counts: %w", err)
	}
	return nil
}

func (s *SqlThreadStore) SaveMembership(ctx context.Context, membership *model.ThreadMembership) error {
	assert.That(membership != nil, "SqlThreadStore.SaveMembership: nil membership")
	assert.That(s.sqlStore != nil, "SqlThreadStore.SaveMembership: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.SaveMembership: nil pool")
	if err := membership.IsValid(); err != nil {
		return err
	}

	query := `INSERT INTO thread_memberships (post_id, user_id, following, last_viewed_at, unread_mention_count)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (post_id, user_id) DO UPDATE SET
			following = EXCLUDED.following,
			last_viewed_at = GREATEST(thread_memberships.last_viewed_at, EXCLUDED.last_viewed_at)`

	_, err := s.sqlStore.pool.Exec(ctx, query,
		membership.PostID, membership.UserID, membership.Following,
		membership.LastViewedAt, membership.UnreadMentionCount,
	)
	if err != nil {
		return fmt.Errorf("save thread membership: %w", err)
	}

	return nil
}

func (s *SqlThreadStore) GetMembership(ctx context.Context, postID, userID string) (*model.ThreadMembership, error) {
	assert.That(s.sqlStore != nil, "SqlThreadStore.GetMembership: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.GetMembership: nil pool")
	query := `SELECT post_id, user_id, following, last_viewed_at, unread_mention_count
		FROM thread_memberships WHERE post_id = $1 AND user_id = $2`

	m := &model.ThreadMembership{}
	err := s.sqlStore.pool.QueryRow(ctx, query, postID, userID).Scan(
		&m.PostID, &m.UserID, &m.Following, &m.LastViewedAt, &m.UnreadMentionCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlThreadStore.GetMembership", postID+"/"+userID)
		}
		return nil, fmt.Errorf("get thread membership: %w", err)
	}

	return m, nil
}

func (s *SqlThreadStore) UpdateMembership(ctx context.Context, membership *model.ThreadMembership) error {
	assert.That(membership != nil, "SqlThreadStore.UpdateMembership: nil membership")
	assert.That(s.sqlStore != nil, "SqlThreadStore.UpdateMembership: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.UpdateMembership: nil pool")
	query := `UPDATE thread_memberships SET following = $1, last_viewed_at = $2, unread_mention_count = $3
		WHERE post_id = $4 AND user_id = $5`

	tag, err := s.sqlStore.pool.Exec(ctx, query,
		membership.Following, membership.LastViewedAt, membership.UnreadMentionCount,
		membership.PostID, membership.UserID,
	)
	if err != nil {
		return fmt.Errorf("update thread membership: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlThreadStore.UpdateMembership", membership.PostID+"/"+membership.UserID)
	}

	return nil
}

// GetThreadsForUser lists the threads userID follows in teamID's channels.
func (s *SqlThreadStore) GetThreadsForUser(ctx context.Context, userID, teamID string, page, perPage int) (*model.UserThreadList, error) {
	assert.That(s.sqlStore != nil, "SqlThreadStore.GetThreadsForUser: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.GetThreadsForUser: nil pool")
	assert.That(page >= 0, "SqlThreadStore.GetThreadsForUser: negative page")
	assert.That(perPage >= 1, "SqlThreadStore.GetThreadsForUser: per page below 1")
	if !validIDs(userID, teamID) {
		return &model.UserThreadList{Threads: []*model.ThreadResponse{}}, nil
	}
	return s.listThreads(ctx, userID, "c.team_id = $2", []any{teamID}, page, perPage)
}

// GetDirectThreadsForUser lists the threads userID follows in direct and
// group channels, which belong to no team and so never appear in a team's
// list.
func (s *SqlThreadStore) GetDirectThreadsForUser(ctx context.Context, userID string, page, perPage int) (*model.UserThreadList, error) {
	assert.That(s.sqlStore != nil, "SqlThreadStore.GetDirectThreadsForUser: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.GetDirectThreadsForUser: nil pool")
	assert.That(page >= 0, "SqlThreadStore.GetDirectThreadsForUser: negative page")
	assert.That(perPage >= 1, "SqlThreadStore.GetDirectThreadsForUser: per page below 1")
	if !validIDs(userID) {
		return &model.UserThreadList{Threads: []*model.ThreadResponse{}}, nil
	}
	return s.listThreads(ctx, userID, "c.team_id IS NULL", nil, page, perPage)
}

// listThreads lists the threads userID follows in the channels scope selects.
// scope is a SQL predicate on channels c whose parameters, scopeArgs, are
// numbered from $2.
//
// Only threads the user can still read: a member of the channel, with the
// root and channel not deleted. Following is not access, and a user removed
// from a channel kept seeing its thread roots here.
//
// The root post and the caller's read state come back with the list. The
// endpoint is documented to report unread status, and a client that had to
// fetch each root separately would issue one request per thread.
func (s *SqlThreadStore) listThreads(ctx context.Context, userID, scope string, scopeArgs []any, page, perPage int) (*model.UserThreadList, error) {
	assert.That(scope != "", "SqlThreadStore.listThreads: empty scope")
	assert.That(perPage >= 1, "SqlThreadStore.listThreads: per page below 1")
	from := `FROM threads t
		INNER JOIN thread_memberships tm ON t.post_id = tm.post_id
		INNER JOIN channels c ON t.channel_id = c.id
		INNER JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = tm.user_id
		INNER JOIN posts p ON p.id = t.post_id
		WHERE tm.user_id = $1 AND tm.following = TRUE AND ` + scope + `
			AND c.delete_at = 0 AND p.delete_at = 0`
	args := append([]any{userID}, scopeArgs...)
	n := len(args)

	query := `SELECT t.post_id, t.channel_id, t.reply_count, t.last_reply_at, t.participants,
			tm.last_viewed_at, tm.unread_mention_count,
			` + postColumnsAs("p") + `
		` + from + `
		ORDER BY t.last_reply_at DESC
		LIMIT $` + strconv.Itoa(n+1) + ` OFFSET $` + strconv.Itoa(n+2)

	rows, err := s.sqlStore.pool.Query(ctx, query, append(args, perPage, page*perPage)...)
	if err != nil {
		return nil, fmt.Errorf("get threads for user: %w", err)
	}
	defer rows.Close()

	threads := []*model.ThreadResponse{}
	for rows.Next() {
		t := &model.Thread{}
		root := &model.Post{}
		tr := &model.ThreadResponse{Thread: t}
		if err := rows.Scan(
			&t.PostID, &t.ChannelID, &t.ReplyCount, &t.LastReplyAt, &t.Participants,
			&tr.LastViewedAt, &tr.UnreadMentions,
			&root.ID, &root.ChannelID, &root.UserID, &root.RootID, &root.Content,
			&root.Type, &root.Props, &root.Hashtags, &root.IsPinned, &root.EditAt,
			&root.CreateAt, &root.UpdateAt, &root.DeleteAt,
		); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		tr.Posts = []*model.Post{root}
		threads = append(threads, tr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var total int64
	if err := s.sqlStore.pool.QueryRow(ctx, `SELECT COUNT(*) `+from, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count threads for user: %w", err)
	}

	return &model.UserThreadList{Threads: threads, Total: total}, nil
}

func (s *SqlThreadStore) MarkAsRead(ctx context.Context, postID, userID string, timestamp int64) error {
	assert.That(s.sqlStore != nil, "SqlThreadStore.MarkAsRead: nil store")
	assert.That(s.sqlStore.pool != nil, "SqlThreadStore.MarkAsRead: nil pool")
	assert.That(timestamp > 0, "SqlThreadStore.MarkAsRead: timestamp is not a time")
	query := `UPDATE thread_memberships SET last_viewed_at = $1, unread_mention_count = 0
		WHERE post_id = $2 AND user_id = $3`

	tag, err := s.sqlStore.pool.Exec(ctx, query, timestamp, postID, userID)
	if err != nil {
		return fmt.Errorf("mark thread as read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlThreadStore.MarkAsRead", postID+"/"+userID)
	}

	return nil
}
