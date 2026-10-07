package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/model"
)

type SqlPostStore struct {
	sqlStore *SqlStore
}

func (s *SqlPostStore) Save(ctx context.Context, post *model.Post) (*model.Post, error) {
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

	_, err := s.sqlStore.pool.Exec(ctx, query,
		post.ID, post.ChannelID, post.UserID, rootID, post.Content,
		post.Type, post.Props, post.Hashtags, post.IsPinned, post.EditAt,
		post.CreateAt, post.UpdateAt, post.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save post: %w", err)
	}

	return post, nil
}

// postColumns is the one SELECT list for posts, and scanPost its one reader.
// root_id is nullable and comes back as ”. Prefix it with postsAlias when the
// query joins posts under an alias.
const postColumns = `id, channel_id, user_id, COALESCE(root_id::text, ''), content, type, props,
	hashtags, is_pinned, edit_at, create_at, update_at, delete_at`

// postColumnsAs is postColumns with every column qualified by alias.
func postColumnsAs(alias string) string {
	return alias + `.id, ` + alias + `.channel_id, ` + alias + `.user_id, COALESCE(` + alias + `.root_id::text, ''), ` +
		alias + `.content, ` + alias + `.type, ` + alias + `.props, ` + alias + `.hashtags, ` + alias + `.is_pinned, ` +
		alias + `.edit_at, ` + alias + `.create_at, ` + alias + `.update_at, ` + alias + `.delete_at`
}

func scanPost(row pgx.Row) (*model.Post, error) {
	p := &model.Post{}
	err := row.Scan(
		&p.ID, &p.ChannelID, &p.UserID, &p.RootID, &p.Content,
		&p.Type, &p.Props, &p.Hashtags, &p.IsPinned, &p.EditAt,
		&p.CreateAt, &p.UpdateAt, &p.DeleteAt,
	)
	return p, err
}

func (s *SqlPostStore) Get(ctx context.Context, id string) (*model.Post, error) {
	// A non-UUID matches nothing; sent as-is it fails the uuid cast as a 500.
	if !model.IsValidID(id) {
		return nil, model.NewNotFoundError("SqlPostStore.Get", id)
	}
	p, err := scanPost(s.sqlStore.pool.QueryRow(ctx, `SELECT `+postColumns+` FROM posts WHERE id = $1 AND delete_at = 0`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlPostStore.Get", id)
		}
		return nil, fmt.Errorf("get post: %w", err)
	}

	return p, nil
}

// GetByIDs returns the live posts among ids, in no particular order. Missing
// and deleted posts are skipped.
func (s *SqlPostStore) GetByIDs(ctx context.Context, ids []string) ([]*model.Post, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT `+postColumns+` FROM posts WHERE id = ANY($1::uuid[]) AND delete_at = 0`, ids)
	if err != nil {
		return nil, fmt.Errorf("get posts by ids: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (s *SqlPostStore) Update(ctx context.Context, post *model.Post) (*model.Post, error) {
	post.PreUpdate()
	post.EditAt = post.UpdateAt

	query := `UPDATE posts SET content = $1, props = $2, hashtags = $3, edit_at = $4, update_at = $5
		WHERE id = $6 AND delete_at = 0`

	tag, err := s.sqlStore.pool.Exec(ctx, query,
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

func (s *SqlPostStore) Delete(ctx context.Context, id string, deleteAt int64) error {
	query := `UPDATE posts SET delete_at = $1, update_at = $1 WHERE id = $2 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(ctx, query, deleteAt, id)
	if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlPostStore.Delete", id)
	}
	return nil
}

func (s *SqlPostStore) GetPostsForChannel(ctx context.Context, channelID string, opts model.GetPostsOptions) (*model.PostList, error) {
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 60
	}
	page := opts.Page
	if page < 0 {
		page = 0
	}

	// Pollers (Since > 0) get chronological order; history readers get
	// newest-first.
	order := "DESC"
	if opts.Since > 0 {
		order = "ASC"
	}

	query := `SELECT ` + postColumns + `
		FROM posts
		WHERE channel_id = $1 AND delete_at = 0 AND ($2::bigint = 0 OR create_at > $2)
		ORDER BY create_at ` + order + `
		LIMIT $3 OFFSET $4`

	rows, err := s.sqlStore.pool.Query(ctx, query, channelID, opts.Since, perPage, page*perPage)
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

func (s *SqlPostStore) GetPostsForThread(ctx context.Context, rootID string) (*model.PostList, error) {
	query := `SELECT ` + postColumns + `
		FROM posts
		WHERE (id = $1 OR root_id = $1) AND delete_at = 0
		ORDER BY create_at ASC`

	rows, err := s.sqlStore.pool.Query(ctx, query, rootID)
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

func (s *SqlPostStore) GetPinnedPosts(ctx context.Context, channelID string) (*model.PostList, error) {
	query := `SELECT ` + postColumns + `
		FROM posts
		WHERE channel_id = $1 AND is_pinned = TRUE AND delete_at = 0
		ORDER BY create_at DESC`

	rows, err := s.sqlStore.pool.Query(ctx, query, channelID)
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

func (s *SqlPostStore) SetPinned(ctx context.Context, id string, pinned bool) error {
	query := `UPDATE posts SET is_pinned = $1, update_at = $2 WHERE id = $3 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(ctx, query, pinned, model.GetMillis(), id)
	if err != nil {
		return fmt.Errorf("set pinned: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlPostStore.SetPinned", id)
	}
	return nil
}

// GetPostsSince returns posts (including soft-deleted ones, so callers can
// remove them from derived indexes) after the cursor in (update_at, id) order,
// updated no later than until.
func (s *SqlPostStore) GetPostsSince(ctx context.Context, after model.PostCursor, until int64, limit int) ([]*model.Post, error) {
	if limit <= 0 {
		limit = 100
	}
	afterID := after.ID
	if afterID == "" {
		afterID = "00000000-0000-0000-0000-000000000000"
	}

	query := `SELECT ` + postColumns + `
		FROM posts
		WHERE (update_at, id) > ($1, $2::uuid) AND update_at <= $3
		ORDER BY update_at ASC, id ASC
		LIMIT $4`

	rows, err := s.sqlStore.pool.Query(ctx, query, after.UpdateAt, afterID, until, limit)
	if err != nil {
		return nil, fmt.Errorf("get posts since: %w", err)
	}
	defer rows.Close()

	return scanPosts(rows)
}

// Search returns one page of live posts matching q, newest first. Every
// filter, the channel scope included, is applied in the query before LIMIT,
// so a page is never short because hits outside the scope were dropped after
// the fact. An empty ChannelIDs matches nothing.
func (s *SqlPostStore) Search(ctx context.Context, q *model.PostSearch) ([]*model.Post, error) {
	if len(q.ChannelIDs) == 0 {
		return nil, nil
	}
	pattern := ""
	if q.Terms != "" {
		pattern = likePattern(q.Terms)
	}
	query := `SELECT ` + postColumns + `
		FROM posts
		WHERE delete_at = 0
			AND channel_id = ANY($1::uuid[])
			AND ($2 = '' OR content ILIKE $2 ESCAPE '\')
			AND ($3 = '' OR user_id = NULLIF($3, '')::uuid)
			AND (cardinality($4::uuid[]) = 0 OR id IN (
				SELECT message_id FROM message_tags
				WHERE tag_id = ANY($4::uuid[])
				GROUP BY message_id
				HAVING COUNT(DISTINCT tag_id) = cardinality($4::uuid[])))
		ORDER BY create_at DESC, id DESC
		LIMIT $5 OFFSET $6`

	tagIDs := q.TagIDs
	if tagIDs == nil {
		tagIDs = []string{}
	}
	rows, err := s.sqlStore.pool.Query(ctx, query,
		q.ChannelIDs, pattern, q.AuthorID, tagIDs, q.PerPage, q.Page*q.PerPage)
	if err != nil {
		return nil, fmt.Errorf("search posts: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

func scanPosts(rows pgx.Rows) ([]*model.Post, error) {
	var posts []*model.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
