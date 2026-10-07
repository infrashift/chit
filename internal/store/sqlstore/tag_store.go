package sqlstore

import (
	"context"
	"fmt"

	"github.com/infrashift/chit/internal/model"
)

type SqlTagStore struct {
	sqlStore *SqlStore
}

func (s *SqlTagStore) Save(ctx context.Context, tag *model.Tag) (*model.Tag, error) {
	tag.PreSave()
	if err := tag.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO tags (id, name) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name`
	err := s.sqlStore.pool.QueryRow(ctx, query, tag.ID, tag.Name).Scan(&tag.ID, &tag.Name)
	if err != nil {
		return nil, fmt.Errorf("save tag: %w", err)
	}

	return tag, nil
}

func (s *SqlTagStore) GetAll(ctx context.Context) ([]*model.Tag, error) {
	query := `SELECT id, name FROM tags ORDER BY name`

	rows, err := s.sqlStore.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all tags: %w", err)
	}
	defer rows.Close()

	var tags []*model.Tag
	for rows.Next() {
		t := &model.Tag{}
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (s *SqlTagStore) AddTagToPost(ctx context.Context, messageID, tagID string) error {
	query := `INSERT INTO message_tags (message_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.sqlStore.pool.Exec(ctx, query, messageID, tagID)
	if err != nil {
		return fmt.Errorf("add tag to post: %w", err)
	}
	return nil
}

func (s *SqlTagStore) RemoveTagFromPost(ctx context.Context, messageID, tagID string) error {
	query := `DELETE FROM message_tags WHERE message_id = $1 AND tag_id = $2`
	_, err := s.sqlStore.pool.Exec(ctx, query, messageID, tagID)
	if err != nil {
		return fmt.Errorf("remove tag from post: %w", err)
	}
	return nil
}

func (s *SqlTagStore) GetTagsForPost(ctx context.Context, messageID string) ([]*model.Tag, error) {
	query := `SELECT t.id, t.name
		FROM tags t
		INNER JOIN message_tags mt ON t.id = mt.tag_id
		WHERE mt.message_id = $1
		ORDER BY t.name`

	rows, err := s.sqlStore.pool.Query(ctx, query, messageID)
	if err != nil {
		return nil, fmt.Errorf("get tags for post: %w", err)
	}
	defer rows.Close()

	var tags []*model.Tag
	for rows.Next() {
		t := &model.Tag{}
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}

	return tags, rows.Err()
}

func (s *SqlTagStore) FilterPostIDsByTags(ctx context.Context, postIDs, tagIDs []string) ([]string, error) {
	query := `SELECT mt.message_id FROM message_tags mt
		WHERE mt.message_id = ANY($1) AND mt.tag_id = ANY($2)
		GROUP BY mt.message_id
		HAVING COUNT(DISTINCT mt.tag_id) = $3`

	rows, err := s.sqlStore.pool.Query(ctx, query, postIDs, tagIDs, len(tagIDs))
	if err != nil {
		return nil, fmt.Errorf("filter post ids by tags: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan post id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetTagsForPosts returns the tags of many posts in one query, keyed by post
// ID. Posts with no tags are absent from the map.
//
// It exists because the client needs the tags of a whole page of history at
// once; asking per post issued sixty round trips per channel open, enough to
// trip the server's own rate limit.
func (s *SqlTagStore) GetTagsForPosts(ctx context.Context, messageIDs []string) (map[string][]*model.Tag, error) {
	result := make(map[string][]*model.Tag)
	if len(messageIDs) == 0 {
		return result, nil
	}

	query := `SELECT mt.message_id, t.id, t.name
		FROM tags t
		INNER JOIN message_tags mt ON t.id = mt.tag_id
		WHERE mt.message_id = ANY($1)
		ORDER BY mt.message_id, t.name`

	rows, err := s.sqlStore.pool.Query(ctx, query, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("get tags for posts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var postID string
		t := &model.Tag{}
		if err := rows.Scan(&postID, &t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		result[postID] = append(result[postID], t)
	}

	return result, rows.Err()
}
