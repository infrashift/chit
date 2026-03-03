package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/model"
)

type SqlChannelStore struct {
	sqlStore *SqlStore
}

func (s *SqlChannelStore) Save(channel *model.Channel) (*model.Channel, error) {
	channel.PreSave()
	if err := channel.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO channels (id, team_id, creator_id, name, display_name, header, purpose, type, total_msg_count, last_post_at, create_at, update_at, delete_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	var teamID any
	if channel.TeamID != "" {
		teamID = channel.TeamID
	}

	_, err := s.sqlStore.pool.Exec(context.Background(), query,
		channel.ID, teamID, channel.CreatorID, channel.Name, channel.DisplayName,
		channel.Header, channel.Purpose, channel.Type, channel.TotalMsgCount,
		channel.LastPostAt, channel.CreateAt, channel.UpdateAt, channel.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save channel: %w", err)
	}

	return channel, nil
}

func (s *SqlChannelStore) Get(id string) (*model.Channel, error) {
	query := `SELECT id, COALESCE(team_id::text, ''), creator_id, name, display_name, header, purpose, type, total_msg_count, last_post_at, create_at, update_at, delete_at
		FROM channels WHERE id = $1 AND delete_at = 0`

	ch := &model.Channel{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, id).Scan(
		&ch.ID, &ch.TeamID, &ch.CreatorID, &ch.Name, &ch.DisplayName,
		&ch.Header, &ch.Purpose, &ch.Type, &ch.TotalMsgCount,
		&ch.LastPostAt, &ch.CreateAt, &ch.UpdateAt, &ch.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlChannelStore.Get", id)
		}
		return nil, fmt.Errorf("get channel: %w", err)
	}

	return ch, nil
}

func (s *SqlChannelStore) GetByName(teamID, name string) (*model.Channel, error) {
	query := `SELECT id, COALESCE(team_id::text, ''), creator_id, name, display_name, header, purpose, type, total_msg_count, last_post_at, create_at, update_at, delete_at
		FROM channels WHERE team_id = $1 AND name = $2 AND delete_at = 0`

	ch := &model.Channel{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, teamID, name).Scan(
		&ch.ID, &ch.TeamID, &ch.CreatorID, &ch.Name, &ch.DisplayName,
		&ch.Header, &ch.Purpose, &ch.Type, &ch.TotalMsgCount,
		&ch.LastPostAt, &ch.CreateAt, &ch.UpdateAt, &ch.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlChannelStore.GetByName", name)
		}
		return nil, fmt.Errorf("get channel by name: %w", err)
	}

	return ch, nil
}

func (s *SqlChannelStore) Update(channel *model.Channel) (*model.Channel, error) {
	channel.PreUpdate()

	query := `UPDATE channels SET name = $1, display_name = $2, header = $3, purpose = $4, update_at = $5
		WHERE id = $6 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(context.Background(), query,
		channel.Name, channel.DisplayName, channel.Header, channel.Purpose, channel.UpdateAt, channel.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, model.NewNotFoundError("SqlChannelStore.Update", channel.ID)
	}

	return channel, nil
}

func (s *SqlChannelStore) Delete(id string, deleteAt int64) error {
	query := `UPDATE channels SET delete_at = $1, update_at = $1 WHERE id = $2 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(context.Background(), query, deleteAt, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlChannelStore.Delete", id)
	}
	return nil
}

func (s *SqlChannelStore) GetChannelsForTeam(teamID string, page, perPage int) ([]*model.Channel, error) {
	query := `SELECT id, COALESCE(team_id::text, ''), creator_id, name, display_name, header, purpose, type, total_msg_count, last_post_at, create_at, update_at, delete_at
		FROM channels WHERE team_id = $1 AND delete_at = 0 ORDER BY display_name LIMIT $2 OFFSET $3`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, teamID, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("get channels for team: %w", err)
	}
	defer rows.Close()

	return scanChannels(rows)
}

func (s *SqlChannelStore) GetChannelsForUser(userID, teamID string) ([]*model.Channel, error) {
	query := `SELECT c.id, COALESCE(c.team_id::text, ''), c.creator_id, c.name, c.display_name, c.header, c.purpose, c.type, c.total_msg_count, c.last_post_at, c.create_at, c.update_at, c.delete_at
		FROM channels c
		INNER JOIN channel_members cm ON c.id = cm.channel_id
		WHERE cm.user_id = $1 AND c.team_id = $2 AND c.delete_at = 0
		ORDER BY c.display_name`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, userID, teamID)
	if err != nil {
		return nil, fmt.Errorf("get channels for user: %w", err)
	}
	defer rows.Close()

	return scanChannels(rows)
}

func (s *SqlChannelStore) SaveMember(member *model.ChannelMember) (*model.ChannelMember, error) {
	member.PreSave()
	if err := member.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO channel_members (channel_id, user_id, roles, last_viewed_at, msg_count, mention_count, notify_props, create_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (channel_id, user_id) DO NOTHING`
	_, err := s.sqlStore.pool.Exec(context.Background(), query,
		member.ChannelID, member.UserID, member.Roles, member.LastViewedAt,
		member.MsgCount, member.MentionCount, member.NotifyProps, member.CreateAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save channel member: %w", err)
	}

	return member, nil
}

func (s *SqlChannelStore) RemoveMember(channelID, userID string) error {
	query := `DELETE FROM channel_members WHERE channel_id = $1 AND user_id = $2`
	_, err := s.sqlStore.pool.Exec(context.Background(), query, channelID, userID)
	if err != nil {
		return fmt.Errorf("remove channel member: %w", err)
	}
	return nil
}

func (s *SqlChannelStore) GetMembers(channelID string, page, perPage int) ([]*model.ChannelMember, error) {
	query := `SELECT channel_id, user_id, roles, last_viewed_at, msg_count, mention_count, notify_props, create_at
		FROM channel_members WHERE channel_id = $1
		ORDER BY create_at LIMIT $2 OFFSET $3`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, channelID, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("get channel members: %w", err)
	}
	defer rows.Close()

	var members []*model.ChannelMember
	for rows.Next() {
		m := &model.ChannelMember{}
		if err := rows.Scan(&m.ChannelID, &m.UserID, &m.Roles, &m.LastViewedAt, &m.MsgCount, &m.MentionCount, &m.NotifyProps, &m.CreateAt); err != nil {
			return nil, fmt.Errorf("scan channel member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *SqlChannelStore) GetMember(channelID, userID string) (*model.ChannelMember, error) {
	query := `SELECT channel_id, user_id, roles, last_viewed_at, msg_count, mention_count, notify_props, create_at
		FROM channel_members WHERE channel_id = $1 AND user_id = $2`

	m := &model.ChannelMember{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, channelID, userID).Scan(
		&m.ChannelID, &m.UserID, &m.Roles, &m.LastViewedAt, &m.MsgCount, &m.MentionCount, &m.NotifyProps, &m.CreateAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlChannelStore.GetMember", channelID+"/"+userID)
		}
		return nil, fmt.Errorf("get channel member: %w", err)
	}

	return m, nil
}

func (s *SqlChannelStore) UpdateLastViewedAt(channelID, userID string, lastViewedAt int64) error {
	query := `UPDATE channel_members SET last_viewed_at = $1 WHERE channel_id = $2 AND user_id = $3`
	_, err := s.sqlStore.pool.Exec(context.Background(), query, lastViewedAt, channelID, userID)
	if err != nil {
		return fmt.Errorf("update last viewed at: %w", err)
	}
	return nil
}

func (s *SqlChannelStore) SaveDirectChannel(channel *model.Channel, userIDs []string) (*model.Channel, error) {
	ctx := context.Background()
	tx, err := s.sqlStore.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	channel.PreSave()
	if err := channel.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO channels (id, team_id, creator_id, name, display_name, header, purpose, type, total_msg_count, last_post_at, create_at, update_at, delete_at)
		VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
	_, err = tx.Exec(ctx, query,
		channel.ID, channel.CreatorID, channel.Name, channel.DisplayName,
		channel.Header, channel.Purpose, channel.Type, channel.TotalMsgCount,
		channel.LastPostAt, channel.CreateAt, channel.UpdateAt, channel.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save direct channel: %w", err)
	}

	for _, userID := range userIDs {
		memberQuery := `INSERT INTO channel_members (channel_id, user_id, roles, create_at) VALUES ($1, $2, 'channel_user', $3)`
		_, err = tx.Exec(ctx, memberQuery, channel.ID, userID, channel.CreateAt)
		if err != nil {
			return nil, fmt.Errorf("save direct channel member: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return channel, nil
}

func (s *SqlChannelStore) IncrementMsgCount(channelID string, timestamp int64) error {
	query := `UPDATE channels SET total_msg_count = total_msg_count + 1, last_post_at = GREATEST(last_post_at, $1) WHERE id = $2`
	_, err := s.sqlStore.pool.Exec(context.Background(), query, timestamp, channelID)
	if err != nil {
		return fmt.Errorf("increment msg count: %w", err)
	}
	return nil
}

func scanChannels(rows pgx.Rows) ([]*model.Channel, error) {
	var channels []*model.Channel
	for rows.Next() {
		ch := &model.Channel{}
		if err := rows.Scan(
			&ch.ID, &ch.TeamID, &ch.CreatorID, &ch.Name, &ch.DisplayName,
			&ch.Header, &ch.Purpose, &ch.Type, &ch.TotalMsgCount,
			&ch.LastPostAt, &ch.CreateAt, &ch.UpdateAt, &ch.DeleteAt,
		); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}
