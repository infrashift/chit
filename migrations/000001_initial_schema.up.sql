-- Initial schema for Project Chit

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users: local profile cache linked to Ory Kratos identity
CREATE TABLE users (
    id          UUID PRIMARY KEY,
    kratos_id   UUID UNIQUE NOT NULL,
    username    VARCHAR(64) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    email       VARCHAR(128) UNIQUE NOT NULL,
    roles       VARCHAR(256) DEFAULT 'system_user',
    create_at   BIGINT NOT NULL,
    update_at   BIGINT NOT NULL,
    delete_at   BIGINT DEFAULT 0
);

CREATE INDEX idx_users_kratos_id ON users (kratos_id);
CREATE INDEX idx_users_username ON users (username);
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_delete_at ON users (delete_at);

-- Teams: top-level organizational unit
CREATE TABLE teams (
    id           UUID PRIMARY KEY,
    name         VARCHAR(64) UNIQUE NOT NULL,
    display_name VARCHAR(64) NOT NULL,
    description  VARCHAR(255) DEFAULT '',
    type         VARCHAR(1) DEFAULT 'O',
    creator_id   UUID REFERENCES users(id),
    create_at    BIGINT NOT NULL,
    update_at    BIGINT NOT NULL,
    delete_at    BIGINT DEFAULT 0
);

CREATE INDEX idx_teams_name ON teams (name);
CREATE INDEX idx_teams_delete_at ON teams (delete_at);

-- Team members
CREATE TABLE team_members (
    team_id   UUID REFERENCES teams(id) ON DELETE CASCADE,
    user_id   UUID REFERENCES users(id) ON DELETE CASCADE,
    roles     VARCHAR(256) DEFAULT 'team_user',
    create_at BIGINT NOT NULL,
    delete_at BIGINT DEFAULT 0,
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX idx_team_members_user_id ON team_members (user_id);

-- Channels: belong to a team (NULL for DMs/GMs)
CREATE TABLE channels (
    id              UUID PRIMARY KEY,
    team_id         UUID REFERENCES teams(id),
    creator_id      UUID REFERENCES users(id),
    name            VARCHAR(64) NOT NULL,
    display_name    VARCHAR(64) NOT NULL,
    header          VARCHAR(1024) DEFAULT '',
    purpose         VARCHAR(250) DEFAULT '',
    type            VARCHAR(1) NOT NULL,
    total_msg_count BIGINT DEFAULT 0,
    last_post_at    BIGINT DEFAULT 0,
    create_at       BIGINT NOT NULL,
    update_at       BIGINT NOT NULL,
    delete_at       BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX idx_channels_team_name_unique
    ON channels (team_id, name) WHERE delete_at = 0;
CREATE INDEX idx_channels_team_id ON channels (team_id);
CREATE INDEX idx_channels_delete_at ON channels (delete_at);

-- Channel members
CREATE TABLE channel_members (
    channel_id    UUID REFERENCES channels(id) ON DELETE CASCADE,
    user_id       UUID REFERENCES users(id) ON DELETE CASCADE,
    roles         VARCHAR(256) DEFAULT 'channel_user',
    last_viewed_at BIGINT DEFAULT 0,
    msg_count     BIGINT DEFAULT 0,
    mention_count BIGINT DEFAULT 0,
    notify_props  JSONB DEFAULT '{}',
    create_at     BIGINT NOT NULL,
    PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX idx_channel_members_user_id ON channel_members (user_id);

-- Posts (messages)
CREATE TABLE posts (
    id                UUID PRIMARY KEY,
    channel_id        UUID REFERENCES channels(id) NOT NULL,
    user_id           UUID REFERENCES users(id) NOT NULL,
    root_id           UUID REFERENCES posts(id),
    content           TEXT NOT NULL DEFAULT '',
    content_encrypted BYTEA,
    type              VARCHAR(26) DEFAULT '',
    props             JSONB DEFAULT '{}',
    hashtags          VARCHAR(1000) DEFAULT '',
    is_pinned         BOOLEAN DEFAULT FALSE,
    edit_at           BIGINT DEFAULT 0,
    create_at         BIGINT NOT NULL,
    update_at         BIGINT NOT NULL,
    delete_at         BIGINT DEFAULT 0
);

CREATE INDEX idx_posts_channel_create ON posts (channel_id, create_at);
CREATE INDEX idx_posts_root_id ON posts (root_id) WHERE root_id IS NOT NULL;
CREATE INDEX idx_posts_user_id ON posts (user_id);
CREATE INDEX idx_posts_delete_at ON posts (delete_at);

-- Threads: denormalized aggregate for root posts with replies
CREATE TABLE threads (
    post_id       UUID PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
    channel_id    UUID REFERENCES channels(id) NOT NULL,
    reply_count   INT DEFAULT 0,
    last_reply_at BIGINT DEFAULT 0,
    participants  JSONB DEFAULT '[]'
);

CREATE INDEX idx_threads_channel_id ON threads (channel_id);

-- Thread memberships: per-user thread follow/read state
CREATE TABLE thread_memberships (
    post_id              UUID REFERENCES posts(id) ON DELETE CASCADE,
    user_id              UUID REFERENCES users(id) ON DELETE CASCADE,
    following            BOOLEAN DEFAULT TRUE,
    last_viewed_at       BIGINT DEFAULT 0,
    unread_mention_count INT DEFAULT 0,
    PRIMARY KEY (post_id, user_id)
);

CREATE INDEX idx_thread_memberships_user_id ON thread_memberships (user_id);

-- Tags
CREATE TABLE tags (
    id   UUID PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

-- Message-tag junction
CREATE TABLE message_tags (
    message_id UUID REFERENCES posts(id) ON DELETE CASCADE,
    tag_id     UUID REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (message_id, tag_id)
);

CREATE INDEX idx_message_tags_tag_id ON message_tags (tag_id);
