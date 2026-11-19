package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

type SqlFileStore struct {
	sqlStore *SqlStore
}

// ready reports whether s was built by SqlStore.File.
func (s *SqlFileStore) ready() bool {
	return s.sqlStore != nil && s.sqlStore.pool != nil
}

// fileColumns reads a file under the alias f, with the post it is attached
// to.
const fileColumns = `f.id, f.channel_id, f.uploader_actor_id, COALESCE(f.via_actor_id::text, ''), f.name,
	f.sha256, f.content_type, f.size_bytes, f.width, f.height, f.alt_text, f.s3_key, f.create_at, f.delete_at,
	COALESCE((SELECT pf.post_id::text FROM post_files pf WHERE pf.file_id = f.id), '')`

func scanFile(row pgx.Row) (*model.File, error) {
	assert.That(row != nil, "scanFile: nil row")
	f := &model.File{}
	err := row.Scan(&f.ID, &f.ChannelID, &f.UploaderID, &f.ViaActorID, &f.Name,
		&f.SHA256, &f.ContentType, &f.SizeBytes, &f.Width, &f.Height, &f.AltText, &f.S3Key, &f.CreateAt, &f.DeleteAt,
		&f.PostID)
	return f, err
}

func collectFiles(rows pgx.Rows) ([]*model.File, error) {
	assert.That(rows != nil, "collectFiles: nil rows")
	defer rows.Close()
	var out []*model.File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *SqlFileStore) Save(ctx context.Context, f *model.File) (*model.File, error) {
	assert.That(s.ready(), "SqlFileStore.Save: nil store")
	assert.That(f != nil, "SqlFileStore.Save: nil file")
	assert.That(f.ID != "" && f.S3Key != "", "SqlFileStore.Save: file has no id or object key")
	_, err := s.sqlStore.pool.Exec(ctx,
		`INSERT INTO files (id, channel_id, uploader_actor_id, via_actor_id, name, sha256, content_type,
			size_bytes, width, height, alt_text, s3_key, create_at, delete_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 0)`,
		f.ID, f.ChannelID, f.UploaderID, nullable(f.ViaActorID), f.Name, f.SHA256, f.ContentType,
		f.SizeBytes, f.Width, f.Height, f.AltText, f.S3Key, f.CreateAt)
	if err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}
	return f, nil
}

func (s *SqlFileStore) Get(ctx context.Context, id string) (*model.File, error) {
	assert.That(s.ready(), "SqlFileStore.Get: nil store")
	if !validIDs(id) {
		return nil, model.NewNotFoundError("SqlFileStore.Get", id)
	}
	f, err := scanFile(s.sqlStore.pool.QueryRow(ctx,
		`SELECT `+fileColumns+` FROM files f WHERE f.id = $1 AND f.delete_at = 0`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlFileStore.Get", id)
		}
		return nil, fmt.Errorf("get file: %w", err)
	}
	return f, nil
}

func (s *SqlFileStore) GetByIDs(ctx context.Context, ids []string) ([]*model.File, error) {
	ids = slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return !model.IsValidID(id) })
	if len(ids) == 0 {
		return nil, nil
	}
	assert.That(s.ready(), "SqlFileStore.GetByIDs: nil store")
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT `+fileColumns+` FROM files f WHERE f.id = ANY($1::uuid[]) AND f.delete_at = 0`, ids)
	if err != nil {
		return nil, fmt.Errorf("get files by ids: %w", err)
	}
	files, err := collectFiles(rows)
	// One row for each distinct id at most.
	assert.That(len(files) <= len(ids), "SqlFileStore.GetByIDs: more files than ids")
	return files, err
}

// GetBySHA256 returns the oldest live file with these bytes in the channel.
func (s *SqlFileStore) GetBySHA256(ctx context.Context, channelID, sha256 string) (*model.File, error) {
	assert.That(s.ready(), "SqlFileStore.GetBySHA256: nil store")
	assert.That(sha256 != "", "SqlFileStore.GetBySHA256: empty digest")
	if !validIDs(channelID) {
		return nil, model.NewNotFoundError("SqlFileStore.GetBySHA256", sha256)
	}
	f, err := scanFile(s.sqlStore.pool.QueryRow(ctx,
		`SELECT `+fileColumns+` FROM files f
		 WHERE f.channel_id = $1 AND f.sha256 = $2 AND f.delete_at = 0
		 ORDER BY f.create_at, f.id LIMIT 1`, channelID, sha256))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlFileStore.GetBySHA256", sha256)
		}
		return nil, fmt.Errorf("get file by sha256: %w", err)
	}
	return f, nil
}

func (s *SqlFileStore) UsageBytes(ctx context.Context, uploaderID string) (int64, error) {
	assert.That(s.ready(), "SqlFileStore.UsageBytes: nil store")
	if !validIDs(uploaderID) {
		return 0, nil
	}
	var n int64
	err := s.sqlStore.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(size_bytes), 0) FROM files WHERE uploader_actor_id = $1 AND delete_at = 0`,
		uploaderID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("file usage: %w", err)
	}
	return n, nil
}

// Search matches the name or the alt text, never anything derived from the
// image. Every filter is in the query, before LIMIT.
func (s *SqlFileStore) Search(ctx context.Context, q *model.FileSearch) ([]*model.File, error) {
	assert.That(s.ready(), "SqlFileStore.Search: nil store")
	assert.That(q != nil, "SqlFileStore.Search: nil query")
	assert.That(q.Page >= 0 && q.PerPage > 0, "SqlFileStore.Search: pagination not clamped")
	if len(q.ChannelIDs) == 0 || !validIDs(q.ReaderID) {
		return nil, nil
	}
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT `+fileColumns+` FROM files f
		 WHERE f.delete_at = 0
		   AND f.channel_id = ANY($1::uuid[])
		   AND (f.name ILIKE $2 ESCAPE '\' OR f.alt_text ILIKE $2 ESCAPE '\')
		   AND (f.uploader_actor_id = $3 OR EXISTS (SELECT 1 FROM post_files pf WHERE pf.file_id = f.id))
		 ORDER BY f.create_at DESC, f.id DESC
		 LIMIT $4 OFFSET $5`,
		q.ChannelIDs, likePattern(q.Terms), q.ReaderID, q.PerPage, q.Page*q.PerPage)
	if err != nil {
		return nil, fmt.Errorf("search files: %w", err)
	}
	return collectFiles(rows)
}

func (s *SqlFileStore) ExpireUnattached(ctx context.Context, createdBefore, deleteAt int64, limit int) (int, error) {
	assert.That(s.ready(), "SqlFileStore.ExpireUnattached: nil store")
	assert.That(limit > 0, "SqlFileStore.ExpireUnattached: limit must be positive")
	tag, err := s.sqlStore.pool.Exec(ctx,
		`UPDATE files SET delete_at = $2
		 WHERE id IN (
			SELECT f.id FROM files f
			WHERE f.delete_at = 0 AND f.create_at < $1
			  AND NOT EXISTS (SELECT 1 FROM post_files pf WHERE pf.file_id = f.id)
			ORDER BY f.create_at
			LIMIT $3)`,
		createdBefore, deleteAt, limit)
	if err != nil {
		return 0, fmt.Errorf("expire unattached files: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (s *SqlFileStore) Deleted(ctx context.Context, deletedBefore int64, limit int) ([]*model.File, error) {
	assert.That(s.ready(), "SqlFileStore.Deleted: nil store")
	assert.That(limit > 0, "SqlFileStore.Deleted: limit must be positive")
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT `+fileColumns+` FROM files f
		 WHERE f.delete_at > 0 AND f.delete_at < $1
		 ORDER BY f.delete_at
		 LIMIT $2`, deletedBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("deleted files: %w", err)
	}
	return collectFiles(rows)
}

func (s *SqlFileStore) ObjectInUse(ctx context.Context, s3Key string) (bool, error) {
	assert.That(s.ready(), "SqlFileStore.ObjectInUse: nil store")
	var inUse bool
	err := s.sqlStore.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM files WHERE s3_key = $1 AND delete_at = 0)`, s3Key).Scan(&inUse)
	if err != nil {
		return false, fmt.Errorf("object in use: %w", err)
	}
	return inUse, nil
}

// Purge removes a soft-deleted file. A live one is left alone.
func (s *SqlFileStore) Purge(ctx context.Context, id string) error {
	assert.That(s.ready(), "SqlFileStore.Purge: nil store")
	if _, err := s.sqlStore.pool.Exec(ctx, `DELETE FROM files WHERE id = $1 AND delete_at > 0`, id); err != nil {
		return fmt.Errorf("purge file: %w", err)
	}
	return nil
}
