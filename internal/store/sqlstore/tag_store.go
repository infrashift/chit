package sqlstore

import (
	"context"
	"fmt"

	"github.com/infrashift/chit/internal/model"
)

type SqlTagStore struct {
	sqlStore *SqlStore
}

func (s *SqlTagStore) Save(tag *model.Tag) (*model.Tag, error) {
	tag.PreSave()
	if err := tag.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO tags (id, name) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name`
	err := s.sqlStore.pool.QueryRow(context.Background(), query, tag.ID, tag.Name).Scan(&tag.ID, &tag.Name)
	if err != nil {
		return nil, fmt.Errorf("save tag: %w", err)
	}

	return tag, nil
}

func (s *SqlTagStore) GetAll() ([]*model.Tag, error) {
	query := `SELECT id, name FROM tags ORDER BY name`

	rows, err := s.sqlStore.pool.Query(context.Background(), query)
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

func (s *SqlTagStore) AddTagToPost(messageID, tagID string) error {
	query := `INSERT INTO message_tags (message_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.sqlStore.pool.Exec(context.Background(), query, messageID, tagID)
	if err != nil {
		return fmt.Errorf("add tag to post: %w", err)
	}
	return nil
}

func (s *SqlTagStore) RemoveTagFromPost(messageID, tagID string) error {
	query := `DELETE FROM message_tags WHERE message_id = $1 AND tag_id = $2`
	_, err := s.sqlStore.pool.Exec(context.Background(), query, messageID, tagID)
	if err != nil {
		return fmt.Errorf("remove tag from post: %w", err)
	}
	return nil
}

func (s *SqlTagStore) GetTagsForPost(messageID string) ([]*model.Tag, error) {
	query := `SELECT t.id, t.name
		FROM tags t
		INNER JOIN message_tags mt ON t.id = mt.tag_id
		WHERE mt.message_id = $1
		ORDER BY t.name`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, messageID)
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
