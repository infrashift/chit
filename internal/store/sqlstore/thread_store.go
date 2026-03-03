package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/model"
)

type SqlThreadStore struct {
	sqlStore *SqlStore
}

func (s *SqlThreadStore) SaveOrUpdate(thread *model.Thread) error {
	if err := thread.IsValid(); err != nil {
		return err
	}

	query := `INSERT INTO threads (post_id, channel_id, reply_count, last_reply_at, participants)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (post_id) DO UPDATE SET
			reply_count = EXCLUDED.reply_count,
			last_reply_at = EXCLUDED.last_reply_at,
			participants = EXCLUDED.participants`

	_, err := s.sqlStore.pool.Exec(context.Background(), query,
		thread.PostID, thread.ChannelID, thread.ReplyCount, thread.LastReplyAt, thread.Participants,
	)
	if err != nil {
		return fmt.Errorf("save or update thread: %w", err)
	}

	return nil
}

func (s *SqlThreadStore) Get(postID string) (*model.Thread, error) {
	query := `SELECT post_id, channel_id, reply_count, last_reply_at, participants
		FROM threads WHERE post_id = $1`

	t := &model.Thread{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, postID).Scan(
		&t.PostID, &t.ChannelID, &t.ReplyCount, &t.LastReplyAt, &t.Participants,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlThreadStore.Get", postID)
		}
		return nil, fmt.Errorf("get thread: %w", err)
	}

	return t, nil
}

func (s *SqlThreadStore) IncrementReplyCount(postID string, timestamp int64, userID string) error {
	// Atomically increment reply_count, update last_reply_at, and add participant if not present.
	query := `UPDATE threads SET
		reply_count = reply_count + 1,
		last_reply_at = GREATEST(last_reply_at, $1),
		participants = CASE
			WHEN NOT participants ? $2 THEN participants || to_jsonb($2::text)
			ELSE participants
		END
		WHERE post_id = $3`

	_, err := s.sqlStore.pool.Exec(context.Background(), query, timestamp, userID, postID)
	if err != nil {
		return fmt.Errorf("increment reply count: %w", err)
	}

	return nil
}

func (s *SqlThreadStore) IncrementMentionCount(postID, userID string) error {
	query := `UPDATE thread_memberships SET unread_mention_count = unread_mention_count + 1 WHERE post_id = $1 AND user_id = $2`
	_, err := s.sqlStore.pool.Exec(context.Background(), query, postID, userID)
	if err != nil {
		return fmt.Errorf("increment thread mention count: %w", err)
	}
	return nil
}

func (s *SqlThreadStore) SaveMembership(membership *model.ThreadMembership) error {
	if err := membership.IsValid(); err != nil {
		return err
	}

	query := `INSERT INTO thread_memberships (post_id, user_id, following, last_viewed_at, unread_mention_count)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (post_id, user_id) DO UPDATE SET
			following = EXCLUDED.following,
			last_viewed_at = EXCLUDED.last_viewed_at`

	_, err := s.sqlStore.pool.Exec(context.Background(), query,
		membership.PostID, membership.UserID, membership.Following,
		membership.LastViewedAt, membership.UnreadMentionCount,
	)
	if err != nil {
		return fmt.Errorf("save thread membership: %w", err)
	}

	return nil
}

func (s *SqlThreadStore) GetMembership(postID, userID string) (*model.ThreadMembership, error) {
	query := `SELECT post_id, user_id, following, last_viewed_at, unread_mention_count
		FROM thread_memberships WHERE post_id = $1 AND user_id = $2`

	m := &model.ThreadMembership{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, postID, userID).Scan(
		&m.PostID, &m.UserID, &m.Following, &m.LastViewedAt, &m.UnreadMentionCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlThreadStore.GetMembership", postID+"/"+userID)
		}
		return nil, fmt.Errorf("get thread membership: %w", err)
	}

	return m, nil
}

func (s *SqlThreadStore) UpdateMembership(membership *model.ThreadMembership) error {
	query := `UPDATE thread_memberships SET following = $1, last_viewed_at = $2, unread_mention_count = $3
		WHERE post_id = $4 AND user_id = $5`

	tag, err := s.sqlStore.pool.Exec(context.Background(), query,
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

func (s *SqlThreadStore) GetThreadsForUser(userID, teamID string, page, perPage int) (*model.UserThreadList, error) {
	query := `SELECT t.post_id, t.channel_id, t.reply_count, t.last_reply_at, t.participants
		FROM threads t
		INNER JOIN thread_memberships tm ON t.post_id = tm.post_id
		INNER JOIN channels c ON t.channel_id = c.id
		WHERE tm.user_id = $1 AND tm.following = TRUE AND c.team_id = $2
		ORDER BY t.last_reply_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, userID, teamID, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("get threads for user: %w", err)
	}
	defer rows.Close()

	var threads []*model.ThreadResponse
	for rows.Next() {
		t := &model.Thread{}
		if err := rows.Scan(&t.PostID, &t.ChannelID, &t.ReplyCount, &t.LastReplyAt, &t.Participants); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		threads = append(threads, &model.ThreadResponse{Thread: t})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	countQuery := `SELECT COUNT(*)
		FROM threads t
		INNER JOIN thread_memberships tm ON t.post_id = tm.post_id
		INNER JOIN channels c ON t.channel_id = c.id
		WHERE tm.user_id = $1 AND tm.following = TRUE AND c.team_id = $2`

	var total int64
	if err := s.sqlStore.pool.QueryRow(context.Background(), countQuery, userID, teamID).Scan(&total); err != nil {
		return nil, fmt.Errorf("count threads for user: %w", err)
	}

	return &model.UserThreadList{Threads: threads, Total: total}, nil
}

func (s *SqlThreadStore) MarkAsRead(postID, userID string, timestamp int64) error {
	query := `UPDATE thread_memberships SET last_viewed_at = $1, unread_mention_count = 0
		WHERE post_id = $2 AND user_id = $3`

	tag, err := s.sqlStore.pool.Exec(context.Background(), query, timestamp, postID, userID)
	if err != nil {
		return fmt.Errorf("mark thread as read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlThreadStore.MarkAsRead", postID+"/"+userID)
	}

	return nil
}
