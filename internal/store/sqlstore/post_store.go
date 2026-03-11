package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/model"
)

type SqlPostStore struct {
	sqlStore *SqlStore
}

func (s *SqlPostStore) Save(post *model.Post) (*model.Post, error) {
	post.PreSave()
	if err := post.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO posts (id, channel_id, user_id, root_id, content, type, props, hashtags, is_pinned, edit_at, create_at, update_at, delete_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	var rootID any
	if post.RootID != "" {
		rootID = post.RootID
	}

	_, err := s.sqlStore.pool.Exec(context.Background(), query,
		post.ID, post.ChannelID, post.UserID, rootID, post.Content,
		post.Type, post.Props, post.Hashtags, post.IsPinned, post.EditAt,
		post.CreateAt, post.UpdateAt, post.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	return post, nil
}

func (s *SqlPostStore) Get(id string) (*model.Post, error) {
	query := `SELECT id, channel_id, user_id, COALESCE(root_id::text, ''), content, type, props, hashtags, is_pinned, edit_at, create_at, update_at, delete_at
		FROM posts WHERE id = $1 AND delete_at = 0`

	p := &model.Post{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, id).Scan(
		&p.ID, &p.ChannelID, &p.UserID, &p.RootID, &p.Content,
		&p.Type, &p.Props, &p.Hashtags, &p.IsPinned, &p.EditAt,
		&p.CreateAt, &p.UpdateAt, &p.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlPostStore.Get", id)
		}
		return nil, fmt.Errorf("get post: %w", err)
	}

	return p, nil
}

func (s *SqlPostStore) Update(post *model.Post) (*model.Post, error) {
	post.PreUpdate()
	post.EditAt = post.UpdateAt

	query := `UPDATE posts SET content = $1, props = $2, hashtags = $3, edit_at = $4, update_at = $5
		WHERE id = $6 AND delete_at = 0`

	tag, err := s.sqlStore.pool.Exec(context.Background(), query,
		post.Content, post.Props, post.Hashtags, post.EditAt, post.UpdateAt, post.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, model.NewNotFoundError("SqlPostStore.Update", post.ID)
	}

	return post, nil
}

func (s *SqlPostStore) Delete(id string, deleteAt int64) error {
	query := `UPDATE posts SET delete_at = $1, update_at = $1 WHERE id = $2 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(context.Background(), query, deleteAt, id)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlPostStore.Delete", id)
	}
	return nil
}

func (s *SqlPostStore) GetPostsForChannel(channelID string, opts model.GetPostsOptions) (*model.PostList, error) {
	perPage := opts.PerPage
	if perPage == 0 {
		perPage = 60
	}

	query := `SELECT id, channel_id, user_id, COALESCE(root_id::text, ''), content, type, props, hashtags, is_pinned, edit_at, create_at, update_at, delete_at
		FROM posts
		WHERE channel_id = $1 AND delete_at = 0
		ORDER BY create_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, channelID, perPage, opts.Page*perPage)
	if err != nil {
		return nil, fmt.Errorf("get posts for channel: %w", err)
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}

	return &model.PostList{Order: posts}, nil
}

func (s *SqlPostStore) GetPostsForThread(rootID string) (*model.PostList, error) {
	query := `SELECT id, channel_id, user_id, COALESCE(root_id::text, ''), content, type, props, hashtags, is_pinned, edit_at, create_at, update_at, delete_at
		FROM posts
		WHERE (id = $1 OR root_id = $1) AND delete_at = 0
		ORDER BY create_at ASC`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, rootID)
	if err != nil {
		return nil, fmt.Errorf("get posts for thread: %w", err)
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}

	return &model.PostList{Order: posts}, nil
}

func (s *SqlPostStore) GetPinnedPosts(channelID string) (*model.PostList, error) {
	query := `SELECT id, channel_id, user_id, COALESCE(root_id::text, ''), content, type, props, hashtags, is_pinned, edit_at, create_at, update_at, delete_at
		FROM posts
		WHERE channel_id = $1 AND is_pinned = TRUE AND delete_at = 0
		ORDER BY create_at DESC`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, channelID)
	if err != nil {
		return nil, fmt.Errorf("get pinned posts: %w", err)
	}
	defer rows.Close()

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}

	return &model.PostList{Order: posts}, nil
}

func (s *SqlPostStore) SetPinned(id string, pinned bool) error {
	query := `UPDATE posts SET is_pinned = $1, update_at = $2 WHERE id = $3 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(context.Background(), query, pinned, model.GetMillis(), id)
	if err != nil {
		return fmt.Errorf("set pinned: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlPostStore.SetPinned", id)
	}
	return nil
}

func (s *SqlPostStore) SearchByContent(channelID, query string, page, perPage int) ([]*model.Post, error) {
	if channelID != "" {
		q := `SELECT id, channel_id, user_id, COALESCE(root_id::text, ''), content, type, props, hashtags, is_pinned, edit_at, create_at, update_at, delete_at
			FROM posts
			WHERE content ILIKE '%' || $1 || '%' AND channel_id = $2 AND delete_at = 0
			ORDER BY create_at DESC
			LIMIT $3 OFFSET $4`
		rows, err := s.sqlStore.pool.Query(context.Background(), q, query, channelID, perPage, page*perPage)
		if err != nil {
			return nil, fmt.Errorf("search posts by content: %w", err)
		}
		defer rows.Close()
		return scanPosts(rows)
	}

	q := `SELECT id, channel_id, user_id, COALESCE(root_id::text, ''), content, type, props, hashtags, is_pinned, edit_at, create_at, update_at, delete_at
		FROM posts
		WHERE content ILIKE '%' || $1 || '%' AND delete_at = 0
		ORDER BY create_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := s.sqlStore.pool.Query(context.Background(), q, query, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("search posts by content: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

func scanPosts(rows pgx.Rows) ([]*model.Post, error) {
	var posts []*model.Post
	for rows.Next() {
		p := &model.Post{}
		if err := rows.Scan(
			&p.ID, &p.ChannelID, &p.UserID, &p.RootID, &p.Content,
			&p.Type, &p.Props, &p.Hashtags, &p.IsPinned, &p.EditAt,
			&p.CreateAt, &p.UpdateAt, &p.DeleteAt,
		); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
