package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// ChitClient defines the API surface the TUI uses.
type ChitClient interface {
	GetMe(ctx context.Context) (*model.User, error)
	GetMyTeams(ctx context.Context) ([]*model.Team, error)
	GetMyChannels(ctx context.Context, teamID string) ([]*model.Channel, error)
	GetChannelPosts(ctx context.Context, channelID string, page, perPage int) (*model.PostList, error)
	CreatePost(ctx context.Context, post *model.Post) (*model.Post, error)
	GetPost(ctx context.Context, postID string) (*model.Post, error)
	PinPost(ctx context.Context, postID string) error
	UnpinPost(ctx context.Context, postID string) error
	GetThread(ctx context.Context, postID string) (*model.PostList, error)
	GetCommands(ctx context.Context) ([]*model.Command, error)
	ViewChannel(ctx context.Context, channelID string) error
	GetUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error)
	SearchPosts(ctx context.Context, channelID, term string, tagIDs []string) (*model.PostList, error)
	GetChannelMembers(ctx context.Context, channelID string) ([]*model.ChannelMember, error)
	CreateDirectChannel(ctx context.Context, userID1, userID2 string) (*model.Channel, error)
	GetMyDirectChannels(ctx context.Context) ([]*model.Channel, error)
	SearchUsers(ctx context.Context, term string, page, perPage int) ([]*model.User, error)
	CreateGroupChannel(ctx context.Context, userIDs []string) (*model.Channel, error)
	CreateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error)
	GetAllTags(ctx context.Context) ([]*model.Tag, error)
	CreateTag(ctx context.Context, name string) (*model.Tag, error)
	GetTagsForPost(ctx context.Context, postID string) ([]*model.Tag, error)
	AddTagToPost(ctx context.Context, postID, tagID string) error
	RemoveTagFromPost(ctx context.Context, postID, tagID string) error
	AddChannelMember(ctx context.Context, channelID, userID string) error
	RemoveChannelMember(ctx context.Context, channelID, userID string) error
}

type httpClient struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a new ChitClient.
func NewClient(baseURL, token string) ChitClient {
	return &httpClient{
		baseURL: baseURL + "/api/v1",
		http: &http.Client{
			Transport: newAuthTransport(nil, func() string { return token }),
		},
	}
}

// NewClientWithHeader creates a ChitClient that uses a custom auth header name.
func NewClientWithHeader(baseURL, token, headerName string) ChitClient {
	return &httpClient{
		baseURL: baseURL + "/api/v1",
		http: &http.Client{
			Transport: newAuthTransportWithHeader(nil, func() string { return token }, headerName),
		},
	}
}

// NewClientWithTokenFn creates a ChitClient that reads the token dynamically via tokenFn.
func NewClientWithTokenFn(baseURL string, tokenFn func() string, headerName string) ChitClient {
	return &httpClient{
		baseURL: baseURL + "/api/v1",
		http: &http.Client{
			Transport: newAuthTransportWithHeader(nil, tokenFn, headerName),
		},
	}
}

func (c *httpClient) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *httpClient) post(ctx context.Context, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out)
}

func (c *httpClient) del(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *httpClient) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp)
	}
	defer func() { _ = resp.Body.Close() }()
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *httpClient) GetMe(ctx context.Context) (*model.User, error) {
	var u model.User
	err := c.get(ctx, "/users/me", &u)
	return &u, err
}

func (c *httpClient) GetMyTeams(ctx context.Context) ([]*model.Team, error) {
	var teams []*model.Team
	err := c.get(ctx, "/users/me/teams", &teams)
	return teams, err
}

func (c *httpClient) GetMyChannels(ctx context.Context, teamID string) ([]*model.Channel, error) {
	var channels []*model.Channel
	err := c.get(ctx, fmt.Sprintf("/users/me/teams/%s/channels", teamID), &channels)
	return channels, err
}

func (c *httpClient) GetChannelPosts(ctx context.Context, channelID string, page, perPage int) (*model.PostList, error) {
	var pl model.PostList
	err := c.get(ctx, fmt.Sprintf("/channels/%s/posts?page=%d&per_page=%d", channelID, page, perPage), &pl)
	return &pl, err
}

func (c *httpClient) CreatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	var created model.Post
	err := c.post(ctx, "/posts", post, &created)
	return &created, err
}

func (c *httpClient) GetPost(ctx context.Context, postID string) (*model.Post, error) {
	var p model.Post
	err := c.get(ctx, fmt.Sprintf("/posts/%s", postID), &p)
	return &p, err
}

func (c *httpClient) PinPost(ctx context.Context, postID string) error {
	return c.post(ctx, fmt.Sprintf("/posts/%s/pin", postID), nil, nil)
}

func (c *httpClient) UnpinPost(ctx context.Context, postID string) error {
	return c.post(ctx, fmt.Sprintf("/posts/%s/unpin", postID), nil, nil)
}

func (c *httpClient) GetThread(ctx context.Context, postID string) (*model.PostList, error) {
	var pl model.PostList
	err := c.get(ctx, fmt.Sprintf("/posts/%s/thread", postID), &pl)
	return &pl, err
}

func (c *httpClient) GetCommands(ctx context.Context) ([]*model.Command, error) {
	var cmds []*model.Command
	err := c.get(ctx, "/commands", &cmds)
	return cmds, err
}

func (c *httpClient) ViewChannel(ctx context.Context, channelID string) error {
	return c.post(ctx, fmt.Sprintf("/channels/%s/members/me/view", channelID), nil, nil)
}

func (c *httpClient) GetUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	var users []*model.User
	err := c.post(ctx, "/users/ids", ids, &users)
	return users, err
}

func (c *httpClient) SearchPosts(ctx context.Context, channelID, term string, tagIDs []string) (*model.PostList, error) {
	var pl model.PostList
	body := map[string]interface{}{"terms": term}
	if len(tagIDs) > 0 {
		body["tag_ids"] = tagIDs
	}
	err := c.post(ctx, fmt.Sprintf("/channels/%s/posts/search", channelID), body, &pl)
	return &pl, err
}

func (c *httpClient) GetChannelMembers(ctx context.Context, channelID string) ([]*model.ChannelMember, error) {
	var members []*model.ChannelMember
	err := c.get(ctx, fmt.Sprintf("/channels/%s/members", channelID), &members)
	return members, err
}

func (c *httpClient) CreateDirectChannel(ctx context.Context, userID1, userID2 string) (*model.Channel, error) {
	var ch model.Channel
	err := c.post(ctx, "/channels/direct", []string{userID1, userID2}, &ch)
	return &ch, err
}

func (c *httpClient) GetMyDirectChannels(ctx context.Context) ([]*model.Channel, error) {
	var channels []*model.Channel
	err := c.get(ctx, "/users/me/channels/direct", &channels)
	return channels, err
}

func (c *httpClient) SearchUsers(ctx context.Context, term string, page, perPage int) ([]*model.User, error) {
	var users []*model.User
	err := c.get(ctx, fmt.Sprintf("/users?term=%s&page=%d&per_page=%d", term, page, perPage), &users)
	return users, err
}

func (c *httpClient) CreateGroupChannel(ctx context.Context, userIDs []string) (*model.Channel, error) {
	var ch model.Channel
	err := c.post(ctx, "/channels/group", userIDs, &ch)
	return &ch, err
}

func (c *httpClient) CreateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
	var ch model.Channel
	err := c.post(ctx, "/channels", channel, &ch)
	return &ch, err
}

func (c *httpClient) GetAllTags(ctx context.Context) ([]*model.Tag, error) {
	var tags []*model.Tag
	err := c.get(ctx, "/tags", &tags)
	return tags, err
}

func (c *httpClient) CreateTag(ctx context.Context, name string) (*model.Tag, error) {
	var tag model.Tag
	err := c.post(ctx, "/tags", map[string]string{"name": name}, &tag)
	return &tag, err
}

func (c *httpClient) GetTagsForPost(ctx context.Context, postID string) ([]*model.Tag, error) {
	var tags []*model.Tag
	err := c.get(ctx, fmt.Sprintf("/posts/%s/tags", postID), &tags)
	return tags, err
}

func (c *httpClient) AddTagToPost(ctx context.Context, postID, tagID string) error {
	return c.post(ctx, fmt.Sprintf("/posts/%s/tags", postID), map[string]string{"tag_id": tagID}, nil)
}

func (c *httpClient) RemoveTagFromPost(ctx context.Context, postID, tagID string) error {
	return c.del(ctx, fmt.Sprintf("/posts/%s/tags/%s", postID, tagID))
}

func (c *httpClient) AddChannelMember(ctx context.Context, channelID, userID string) error {
	return c.post(ctx, fmt.Sprintf("/channels/%s/members", channelID), map[string]string{"user_id": userID}, nil)
}

// RemoveChannelMember removes a user from a channel. Removing yourself is
// leaving; removing anyone else requires system-admin rights server-side.
func (c *httpClient) RemoveChannelMember(ctx context.Context, channelID, userID string) error {
	return c.del(ctx, fmt.Sprintf("/channels/%s/members/%s", channelID, userID))
}
