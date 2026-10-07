package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/model"
)

type SqlChannelStore struct {
	sqlStore *SqlStore
}

func (s *SqlChannelStore) Save(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
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

	_, err := s.sqlStore.pool.Exec(ctx, query,
		channel.ID, teamID, channel.CreatorID, channel.Name, channel.DisplayName,
		channel.Header, channel.Purpose, channel.Type, channel.TotalMsgCount,
		channel.LastPostAt, channel.CreateAt, channel.UpdateAt, channel.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save channel: %w", err)
	}

	return channel, nil
}

func (s *SqlChannelStore) Get(ctx context.Context, id string) (*model.Channel, error) {
	if !validIDs(id) {
		return nil, model.NewNotFoundError("SqlChannelStore.Get", id)
	}
	query := `SELECT ` + channelColumns + `
		FROM channels WHERE id = $1 AND delete_at = 0`

	ch, err := scanChannel(s.sqlStore.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlChannelStore.Get", id)
		}
		return nil, fmt.Errorf("get channel: %w", err)
	}

	return ch, nil
}

func (s *SqlChannelStore) GetByName(ctx context.Context, teamID, name string) (*model.Channel, error) {
	query := `SELECT ` + channelColumns + `
		FROM channels WHERE team_id = $1 AND name = $2 AND delete_at = 0`

	ch, err := scanChannel(s.sqlStore.pool.QueryRow(ctx, query, teamID, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlChannelStore.GetByName", name)
		}
		return nil, fmt.Errorf("get channel by name: %w", err)
	}

	return ch, nil
}

func (s *SqlChannelStore) Update(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
	channel.PreUpdate()

	query := `UPDATE channels SET name = $1, display_name = $2, header = $3, purpose = $4, update_at = $5
		WHERE id = $6 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(ctx, query,
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

func (s *SqlChannelStore) Delete(ctx context.Context, id string, deleteAt int64) error {
	query := `UPDATE channels SET delete_at = $1, update_at = $1 WHERE id = $2 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(ctx, query, deleteAt, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlChannelStore.Delete", id)
	}
	return nil
}

// AddTeamMembers adds every member of teamID to channelID in one statement,
// returning the user IDs it added (existing members are skipped).
func (s *SqlChannelStore) AddTeamMembers(ctx context.Context, channelID, teamID string) ([]string, error) {
	rows, err := s.sqlStore.pool.Query(ctx,
		`INSERT INTO channel_members (channel_id, user_id, create_at)
		SELECT $1, tm.user_id, $3 FROM team_members tm
		WHERE tm.team_id = $2 AND tm.delete_at = 0
		ON CONFLICT (channel_id, user_id) DO NOTHING
		RETURNING user_id::text`,
		channelID, teamID, model.GetMillis())
	if err != nil {
		return nil, fmt.Errorf("add team members to channel: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// DeleteForTeam soft-deletes every channel on a team, returning their IDs.
func (s *SqlChannelStore) DeleteForTeam(ctx context.Context, teamID string, deleteAt int64) ([]string, error) {
	rows, err := s.sqlStore.pool.Query(ctx,
		`UPDATE channels SET delete_at = $1, update_at = $1 WHERE team_id = $2 AND delete_at = 0 RETURNING id`,
		deleteAt, teamID)
	if err != nil {
		return nil, fmt.Errorf("delete team channels: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// RemoveMemberFromTeam removes userID from every channel on teamID, returning
// the channels they were removed from.
func (s *SqlChannelStore) RemoveMemberFromTeam(ctx context.Context, teamID, userID string) ([]string, error) {
	rows, err := s.sqlStore.pool.Query(ctx,
		`DELETE FROM channel_members cm USING channels c
		WHERE c.id = cm.channel_id AND c.team_id = $1 AND cm.user_id = $2
		RETURNING cm.channel_id`,
		teamID, userID)
	if err != nil {
		return nil, fmt.Errorf("remove team channel memberships: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// GetChannelsForTeam lists a team's open channels: the ones any team member
// may browse and join. Private channels are listed only to their members, by
// GetChannelsForUser.
func (s *SqlChannelStore) GetChannelsForTeam(ctx context.Context, teamID string, page, perPage int) ([]*model.Channel, error) {
	query := `SELECT ` + channelColumns + `
		FROM channels WHERE team_id = $1 AND type = 'O' AND delete_at = 0 ORDER BY display_name LIMIT $2 OFFSET $3`

	rows, err := s.sqlStore.pool.Query(ctx, query, teamID, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("get channels for team: %w", err)
	}
	defer rows.Close()

	return scanChannels(rows)
}

func (s *SqlChannelStore) GetChannelsForUser(ctx context.Context, userID, teamID string) ([]*model.Channel, error) {
	query := `SELECT ` + channelColumnsAs("c") + `
		FROM channels c
		INNER JOIN channel_members cm ON c.id = cm.channel_id
		WHERE cm.user_id = $1 AND c.team_id = $2 AND c.delete_at = 0
		ORDER BY c.display_name`

	rows, err := s.sqlStore.pool.Query(ctx, query, userID, teamID)
	if err != nil {
		return nil, fmt.Errorf("get channels for user: %w", err)
	}
	defer rows.Close()

	return scanChannels(rows)
}

func (s *SqlChannelStore) SaveMember(ctx context.Context, member *model.ChannelMember) (*model.ChannelMember, error) {
	member.PreSave()
	if err := member.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO channel_members (channel_id, user_id, roles, last_viewed_at, msg_count, mention_count, notify_props, create_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (channel_id, user_id) DO NOTHING`
	_, err := s.sqlStore.pool.Exec(ctx, query,
		member.ChannelID, member.UserID, member.Roles, member.LastViewedAt,
		member.MsgCount, member.MentionCount, member.NotifyProps, member.CreateAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save channel member: %w", err)
	}

	return member, nil
}

func (s *SqlChannelStore) RemoveMember(ctx context.Context, channelID, userID string) error {
	query := `DELETE FROM channel_members WHERE channel_id = $1 AND user_id = $2`
	_, err := s.sqlStore.pool.Exec(ctx, query, channelID, userID)
	if err != nil {
		return fmt.Errorf("remove channel member: %w", err)
	}
	return nil
}

func (s *SqlChannelStore) GetMembers(ctx context.Context, channelID string, page, perPage int) ([]*model.ChannelMember, error) {
	query := `SELECT channel_id, user_id, roles, last_viewed_at, msg_count, mention_count, notify_props, create_at
		FROM channel_members WHERE channel_id = $1
		ORDER BY create_at LIMIT $2 OFFSET $3`

	rows, err := s.sqlStore.pool.Query(ctx, query, channelID, perPage, page*perPage)
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

func (s *SqlChannelStore) GetChannelIDsForUser(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT cm.channel_id FROM channel_members cm
		INNER JOIN channels c ON c.id = cm.channel_id AND c.delete_at = 0
		WHERE cm.user_id = $1`

	rows, err := s.sqlStore.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get channel ids for user: %w", err)
	}
	defer rows.Close()

	var channelIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan channel id: %w", err)
		}
		channelIDs = append(channelIDs, id)
	}
	return channelIDs, rows.Err()
}

// GetMember returns userID's membership of channelID. A deleted channel has
// no members: this is the access check, and a member of a deleted channel
// could otherwise keep posting to it and reading its history.
func (s *SqlChannelStore) GetMember(ctx context.Context, channelID, userID string) (*model.ChannelMember, error) {
	if !validIDs(channelID, userID) {
		return nil, model.NewNotFoundError("SqlChannelStore.GetMember", channelID+"/"+userID)
	}
	query := `SELECT cm.channel_id, cm.user_id, cm.roles, cm.last_viewed_at, cm.msg_count, cm.mention_count, cm.notify_props, cm.create_at
		FROM channel_members cm
		INNER JOIN channels c ON c.id = cm.channel_id AND c.delete_at = 0
		WHERE cm.channel_id = $1 AND cm.user_id = $2`

	m := &model.ChannelMember{}
	err := s.sqlStore.pool.QueryRow(ctx, query, channelID, userID).Scan(
		&m.ChannelID, &m.UserID, &m.Roles, &m.LastViewedAt, &m.MsgCount, &m.MentionCount, &m.NotifyProps, &m.CreateAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlChannelStore.GetMember", channelID+"/"+userID)
		}
		return nil, fmt.Errorf("get channel member: %w", err)
	}

	return m, nil
}

func (s *SqlChannelStore) GetMembersForUser(ctx context.Context, userID, teamID string) ([]*model.ChannelMember, error) {
	query := `SELECT cm.channel_id, cm.user_id, cm.roles, cm.last_viewed_at, cm.msg_count, cm.mention_count, cm.notify_props, cm.create_at
		FROM channel_members cm
		INNER JOIN channels c ON c.id = cm.channel_id
		WHERE cm.user_id = $1 AND c.team_id = $2 AND c.delete_at = 0`

	rows, err := s.sqlStore.pool.Query(ctx, query, userID, teamID)
	if err != nil {
		return nil, fmt.Errorf("get members for user: %w", err)
	}
	defer rows.Close()

	members := []*model.ChannelMember{}
	for rows.Next() {
		m := &model.ChannelMember{}
		if err := rows.Scan(&m.ChannelID, &m.UserID, &m.Roles, &m.LastViewedAt, &m.MsgCount, &m.MentionCount, &m.NotifyProps, &m.CreateAt); err != nil {
			return nil, fmt.Errorf("scan channel member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *SqlChannelStore) UpdateLastViewedAt(ctx context.Context, channelID, userID string, lastViewedAt int64) error {
	query := `UPDATE channel_members
		SET last_viewed_at = $1,
		    msg_count = (SELECT total_msg_count FROM channels WHERE id = $2),
		    mention_count = 0
		WHERE channel_id = $2 AND user_id = $3`
	_, err := s.sqlStore.pool.Exec(ctx, query, lastViewedAt, channelID, userID)
	if err != nil {
		return fmt.Errorf("update last viewed at: %w", err)
	}
	return nil
}

func (s *SqlChannelStore) GetDirectChannelByName(ctx context.Context, name string) (*model.Channel, error) {
	query := `SELECT ` + channelColumns + `
		FROM channels WHERE team_id IS NULL AND type IN ('D', 'G') AND name = $1 AND delete_at = 0`

	ch, err := scanChannel(s.sqlStore.pool.QueryRow(ctx, query, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.NewNotFoundError("SqlChannelStore.GetDirectChannelByName", name)
		}
		return nil, fmt.Errorf("get direct channel by name: %w", err)
	}

	return ch, nil
}

func (s *SqlChannelStore) SaveDirectChannel(ctx context.Context, channel *model.Channel, userIDs []string) (*model.Channel, error) {
	tx, err := s.sqlStore.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

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
		if isUniqueViolation(err) {
			// Another request created the same direct channel first.
			return nil, model.NewConflictError("SqlChannelStore.SaveDirectChannel", "direct channel already exists")
		}
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

func (s *SqlChannelStore) GetDirectChannelsForUser(ctx context.Context, userID string) ([]*model.Channel, error) {
	query := `SELECT ` + channelColumnsAs("c") + `
		FROM channels c
		INNER JOIN channel_members cm ON c.id = cm.channel_id
		WHERE cm.user_id = $1 AND c.team_id IS NULL
		  AND c.type IN ('D','G') AND c.delete_at = 0
		ORDER BY c.last_post_at DESC`

	rows, err := s.sqlStore.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get direct channels for user: %w", err)
	}
	defer rows.Close()

	return scanChannels(rows)
}

func (s *SqlChannelStore) IncrementMsgCount(ctx context.Context, channelID string, timestamp int64) error {
	query := `UPDATE channels SET total_msg_count = total_msg_count + 1, last_post_at = GREATEST(last_post_at, $1) WHERE id = $2`
	_, err := s.sqlStore.pool.Exec(ctx, query, timestamp, channelID)
	if err != nil {
		return fmt.Errorf("increment msg count: %w", err)
	}
	return nil
}

func (s *SqlChannelStore) IncrementMentionCounts(ctx context.Context, channelID string, userIDs []string) error {
	query := `UPDATE channel_members SET mention_count = mention_count + 1
		WHERE channel_id = $1 AND user_id = ANY($2::uuid[])`
	if _, err := s.sqlStore.pool.Exec(ctx, query, channelID, userIDs); err != nil {
		return fmt.Errorf("increment mention counts: %w", err)
	}
	return nil
}

func (s *SqlChannelStore) GetMemberIDs(ctx context.Context, channelID string) ([]string, error) {
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT user_id::text FROM channel_members WHERE channel_id = $1`, channelID)
	if err != nil {
		return nil, fmt.Errorf("get member ids: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *SqlChannelStore) GetMemberIDsByUsernames(ctx context.Context, channelID string, usernames []string) ([]string, error) {
	rows, err := s.sqlStore.pool.Query(ctx,
		`SELECT u.id::text FROM users u
		INNER JOIN channel_members cm ON cm.user_id = u.id AND cm.channel_id = $1
		WHERE u.username = ANY($2) AND u.delete_at = 0`,
		channelID, usernames)
	if err != nil {
		return nil, fmt.Errorf("get member ids by usernames: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// channelColumns is the one SELECT list for channels, and scanChannel its one
// reader. team_id is NULL for direct and group channels and comes back as ”.
const channelColumns = `id, COALESCE(team_id::text, ''), creator_id, name, display_name, header, purpose,
	type, total_msg_count, last_post_at, create_at, update_at, delete_at`

// channelColumnsAs is channelColumns qualified by alias, for joins.
func channelColumnsAs(alias string) string {
	return alias + `.id, COALESCE(` + alias + `.team_id::text, ''), ` + alias + `.creator_id, ` + alias + `.name, ` +
		alias + `.display_name, ` + alias + `.header, ` + alias + `.purpose, ` + alias + `.type, ` +
		alias + `.total_msg_count, ` + alias + `.last_post_at, ` + alias + `.create_at, ` + alias + `.update_at, ` +
		alias + `.delete_at`
}

func scanChannel(row pgx.Row) (*model.Channel, error) {
	ch := &model.Channel{}
	err := row.Scan(
		&ch.ID, &ch.TeamID, &ch.CreatorID, &ch.Name, &ch.DisplayName,
		&ch.Header, &ch.Purpose, &ch.Type, &ch.TotalMsgCount,
		&ch.LastPostAt, &ch.CreateAt, &ch.UpdateAt, &ch.DeleteAt,
	)
	return ch, err
}

func scanChannels(rows pgx.Rows) ([]*model.Channel, error) {
	var channels []*model.Channel
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}
