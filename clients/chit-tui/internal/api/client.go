package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// ChitClient defines the API surface the TUI uses.
type ChitClient interface {
	GetMe(ctx context.Context) (*model.User, error)
	UpdateMe(ctx context.Context, patch *model.User) (*model.User, error)
	GetMyTeams(ctx context.Context) ([]*model.Team, error)
	GetMyChannels(ctx context.Context, teamID string) ([]*model.Channel, error)
	GetChannelPosts(ctx context.Context, channelID string, page, perPage int) (*model.PostList, error)
	CreatePost(ctx context.Context, post *model.Post) (*model.Post, error)
	UpdatePost(ctx context.Context, postID, content string) (*model.Post, error)
	DeletePost(ctx context.Context, postID string) error
	GetPost(ctx context.Context, postID string) (*model.Post, error)
	PinPost(ctx context.Context, postID string) error
	UnpinPost(ctx context.Context, postID string) error
	GetThread(ctx context.Context, postID string) (*model.PostList, error)
	// GetMyThreads lists the threads the caller follows in a team.
	GetMyThreads(ctx context.Context, teamID string, page, perPage int) (*model.UserThreadList, error)
	// GetMyDirectThreads lists the threads the caller follows in direct and
	// group channels, which belong to no team.
	GetMyDirectThreads(ctx context.Context, page, perPage int) (*model.UserThreadList, error)
	MarkThreadRead(ctx context.Context, threadID string) error
	SetThreadFollowing(ctx context.Context, threadID string, following bool) error
	GetCommands(ctx context.Context) ([]*model.Command, error)
	ViewChannel(ctx context.Context, channelID string) error
	GetUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error)
	SearchPosts(ctx context.Context, channelID, term string, tagIDs []string) (*model.PostList, error)
	// SearchPostsEverywhere searches every channel the user belongs to.
	SearchPostsEverywhere(ctx context.Context, term string, tagIDs []string) (*model.PostList, error)
	GetChannelMembers(ctx context.Context, channelID string) ([]*model.ChannelMember, error)
	// GetMyChannelMembers returns the caller's own member rows for a team's
	// channels, which carry their unread and mention counts.
	GetMyChannelMembers(ctx context.Context, teamID string) ([]*model.ChannelMember, error)
	CreateDirectChannel(ctx context.Context, userID1, userID2 string) (*model.Channel, error)
	GetMyDirectChannels(ctx context.Context) ([]*model.Channel, error)
	SearchUsers(ctx context.Context, term string, page, perPage int) ([]*model.User, error)
	CreateGroupChannel(ctx context.Context, userIDs []string) (*model.Channel, error)
	CreateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error)
	GetAllTags(ctx context.Context) ([]*model.Tag, error)
	// CreateTag creates a tag: scope "channel" in channelID (shared with its
	// members), or "private" (seen by the caller only, channelID empty).
	CreateTag(ctx context.Context, name, scope, channelID string) (*model.Tag, error)
	GetTagsForPost(ctx context.Context, postID string) ([]*model.Tag, error)
	// GetTagsForPosts fetches tags for many posts in one request, so opening
	// a channel does not cost one round trip per message.
	GetTagsForPosts(ctx context.Context, postIDs []string) (map[string][]*model.Tag, error)
	AddTagToPost(ctx context.Context, postID, tagID string) error
	RemoveTagFromPost(ctx context.Context, postID, tagID string) error
	AddChannelMember(ctx context.Context, channelID, userID string) error
	RemoveChannelMember(ctx context.Context, channelID, userID string) error
}

type httpClient struct {
	baseURL string
	http    *http.Client
}

// apiPrefix is the path every request URL starts with.
const apiPrefix = "/api/v1"

// DefaultTimeout bounds every request. Without one, a server that accepted a
// request and never answered left its command waiting forever.
const DefaultTimeout = 30 * time.Second

// NewClient creates a new ChitClient.
func NewClient(baseURL, token string) ChitClient {
	assert.That(baseURL != "", "NewClient: empty base URL")
	return newHTTPClient(baseURL, newAuthTransport(nil, staticToken(token)))
}

// NewClientWithHeader creates a ChitClient that uses a custom auth header name.
func NewClientWithHeader(baseURL, token, headerName string) ChitClient {
	assert.That(baseURL != "", "NewClientWithHeader: empty base URL")
	assert.That(headerName != "", "NewClientWithHeader: empty header name")
	return newHTTPClient(baseURL, newAuthTransportWithHeader(nil, staticToken(token), headerName))
}

// NewClientWithTokenSource creates a ChitClient that reads the token from
// tokens on every request, so a token set after sign-in takes effect at once.
func NewClientWithTokenSource(baseURL string, tokens TokenSource, headerName string) ChitClient {
	assert.That(baseURL != "", "NewClientWithTokenSource: empty base URL")
	assert.That(tokens != nil, "NewClientWithTokenSource: nil token source")
	assert.That(headerName != "", "NewClientWithTokenSource: empty header name")
	return newHTTPClient(baseURL, newAuthTransportWithHeader(nil, tokens, headerName))
}

// newHTTPClient builds the client every constructor returns: requests go to
// baseURL's API root through transport, and none waits longer than
// DefaultTimeout.
func newHTTPClient(baseURL string, transport http.RoundTripper) *httpClient {
	assert.That(transport != nil, "newHTTPClient: nil transport")
	c := &httpClient{
		baseURL: baseURL + apiPrefix,
		http: &http.Client{
			Timeout:   DefaultTimeout,
			Transport: transport,
		},
	}
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "newHTTPClient: base URL must end at the API root")
	assert.That(c.http.Timeout > 0, "newHTTPClient: requests must time out")
	return c
}

func (c *httpClient) get(ctx context.Context, path string, out any) error {
	assert.That(ctx != nil, "get: nil context")
	assert.That(strings.HasPrefix(path, "/"), "get: path must start with /")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "get: base URL must end at the API root")
	assert.That(out != nil, "get: nil destination")
	assert.That(reflect.ValueOf(out).Kind() == reflect.Pointer, "get: destination must be a pointer")
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

// put issues a PUT with a JSON body. Editing is the only thing that needs it,
// so it mirrors post rather than generalising further.
func (c *httpClient) put(ctx context.Context, path string, body, out any) error {
	assert.That(ctx != nil, "put: nil context")
	assert.That(strings.HasPrefix(path, "/"), "put: path must start with /")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "put: base URL must end at the API root")
	assert.That(out == nil || reflect.ValueOf(out).Kind() == reflect.Pointer, "put: destination must be a pointer")
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *httpClient) post(ctx context.Context, path string, body, out any) error {
	assert.That(ctx != nil, "post: nil context")
	assert.That(strings.HasPrefix(path, "/"), "post: path must start with /")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "post: base URL must end at the API root")
	assert.That(out == nil || reflect.ValueOf(out).Kind() == reflect.Pointer, "post: destination must be a pointer")
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
	assert.That(ctx != nil, "del: nil context")
	assert.That(strings.HasPrefix(path, "/"), "del: path must start with /")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "del: base URL must end at the API root")
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *httpClient) do(req *http.Request, out any) error {
	assert.That(req != nil, "do: nil request")
	assert.That(c.http != nil && c.http.Timeout > 0, "do: every request must time out")
	assert.That(out == nil || reflect.ValueOf(out).Kind() == reflect.Pointer, "do: destination must be a pointer")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "do: base URL must end at the API root")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	assert.That(resp != nil && resp.Body != nil, "do: no response without an error")
	defer func() {
		// A body left partly unread closes the connection rather than
		// returning it for reuse.
		if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)); err != nil {
			errlog.Debug(err, "drain api response body")
		}
		errlog.Close(resp.Body, "api response body")
	}()
	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *httpClient) GetMe(ctx context.Context) (*model.User, error) {
	assert.That(ctx != nil, "GetMe: nil context")
	assert.That(c.http != nil, "GetMe: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetMe: base URL must end at the API root")
	var u model.User
	err := c.get(ctx, "/users/me", &u)
	return &u, err
}

// UpdateMe applies a partial update to the signed-in user.
//
// The body carries only the fields being changed. Marshaling the whole user
// would send username:"" on a display-name change, and the schema's username
// pattern does not match the empty string — the request is rejected outright,
// not quietly ignored.
func (c *httpClient) UpdateMe(ctx context.Context, patch *model.User) (*model.User, error) {
	assert.That(ctx != nil, "UpdateMe: nil context")
	assert.That(c.http != nil, "UpdateMe: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "UpdateMe: base URL must end at the API root")
	assert.That(patch != nil, "UpdateMe: nil patch")
	assert.That(patch.DisplayName != "" || patch.Username != "", "UpdateMe: patch changes nothing")
	body := map[string]string{}
	if patch.DisplayName != "" {
		body["display_name"] = patch.DisplayName
	}
	if patch.Username != "" {
		body["username"] = patch.Username
	}
	assert.That(len(body) >= 1 && len(body) <= 2, "UpdateMe: body must carry one or two changed fields")

	var u model.User
	err := c.put(ctx, "/users/me", body, &u)
	return &u, err
}

func (c *httpClient) GetMyThreads(ctx context.Context, teamID string, page, perPage int) (*model.UserThreadList, error) {
	assert.That(ctx != nil, "GetMyThreads: nil context")
	assert.That(c.http != nil, "GetMyThreads: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetMyThreads: base URL must end at the API root")
	assert.That(teamID != "", "GetMyThreads: empty team ID")
	assert.That(page >= 0 && perPage > 0, "GetMyThreads: page must be >= 0 and perPage > 0")
	var list model.UserThreadList
	err := c.get(ctx, fmt.Sprintf("/users/me/teams/%s/threads?page=%d&per_page=%d",
		teamID, page, perPage), &list)
	return &list, err
}

func (c *httpClient) GetMyDirectThreads(ctx context.Context, page, perPage int) (*model.UserThreadList, error) {
	assert.That(ctx != nil, "GetMyDirectThreads: nil context")
	assert.That(c.http != nil, "GetMyDirectThreads: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetMyDirectThreads: base URL must end at the API root")
	assert.That(page >= 0 && perPage > 0, "GetMyDirectThreads: page must be >= 0 and perPage > 0")
	var list model.UserThreadList
	err := c.get(ctx, fmt.Sprintf("/users/me/threads/direct?page=%d&per_page=%d", page, perPage), &list)
	return &list, err
}

// MarkThreadRead and SetThreadFollowing use the routes without a team: the
// team-scoped ones ignore their team ID, and a thread in a DM has none.
func (c *httpClient) MarkThreadRead(ctx context.Context, threadID string) error {
	assert.That(ctx != nil, "MarkThreadRead: nil context")
	assert.That(c.http != nil, "MarkThreadRead: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "MarkThreadRead: base URL must end at the API root")
	return c.put(ctx, fmt.Sprintf("/users/me/threads/%s/read", threadID), nil, nil)
}

func (c *httpClient) SetThreadFollowing(ctx context.Context, threadID string, following bool) error {
	assert.That(ctx != nil, "SetThreadFollowing: nil context")
	assert.That(c.http != nil, "SetThreadFollowing: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "SetThreadFollowing: base URL must end at the API root")
	return c.put(ctx, fmt.Sprintf("/users/me/threads/%s/following", threadID),
		map[string]bool{"following": following}, nil)
}

func (c *httpClient) GetMyTeams(ctx context.Context) ([]*model.Team, error) {
	assert.That(ctx != nil, "GetMyTeams: nil context")
	assert.That(c.http != nil, "GetMyTeams: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetMyTeams: base URL must end at the API root")
	var teams []*model.Team
	err := c.get(ctx, "/users/me/teams", &teams)
	return teams, err
}

func (c *httpClient) GetMyChannels(ctx context.Context, teamID string) ([]*model.Channel, error) {
	assert.That(ctx != nil, "GetMyChannels: nil context")
	assert.That(c.http != nil, "GetMyChannels: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetMyChannels: base URL must end at the API root")
	var channels []*model.Channel
	err := c.get(ctx, fmt.Sprintf("/users/me/teams/%s/channels", teamID), &channels)
	return channels, err
}

func (c *httpClient) GetChannelPosts(ctx context.Context, channelID string, page, perPage int) (*model.PostList, error) {
	assert.That(ctx != nil, "GetChannelPosts: nil context")
	assert.That(c.http != nil, "GetChannelPosts: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetChannelPosts: base URL must end at the API root")
	assert.That(page >= 0 && perPage > 0, "GetChannelPosts: page must be >= 0 and perPage > 0")
	var pl model.PostList
	err := c.get(ctx, fmt.Sprintf("/channels/%s/posts?page=%d&per_page=%d", channelID, page, perPage), &pl)
	return &pl, err
}

func (c *httpClient) CreatePost(ctx context.Context, post *model.Post) (*model.Post, error) {
	assert.That(ctx != nil, "CreatePost: nil context")
	assert.That(c.http != nil, "CreatePost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "CreatePost: base URL must end at the API root")
	assert.That(post != nil, "CreatePost: nil post")
	var created model.Post
	err := c.post(ctx, "/posts", post, &created)
	return &created, err
}

func (c *httpClient) GetPost(ctx context.Context, postID string) (*model.Post, error) {
	assert.That(ctx != nil, "GetPost: nil context")
	assert.That(c.http != nil, "GetPost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetPost: base URL must end at the API root")
	var p model.Post
	err := c.get(ctx, fmt.Sprintf("/posts/%s", postID), &p)
	return &p, err
}

func (c *httpClient) PinPost(ctx context.Context, postID string) error {
	assert.That(ctx != nil, "PinPost: nil context")
	assert.That(c.http != nil, "PinPost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "PinPost: base URL must end at the API root")
	return c.post(ctx, fmt.Sprintf("/posts/%s/pin", postID), nil, nil)
}

func (c *httpClient) UnpinPost(ctx context.Context, postID string) error {
	assert.That(ctx != nil, "UnpinPost: nil context")
	assert.That(c.http != nil, "UnpinPost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "UnpinPost: base URL must end at the API root")
	return c.post(ctx, fmt.Sprintf("/posts/%s/unpin", postID), nil, nil)
}

func (c *httpClient) GetThread(ctx context.Context, postID string) (*model.PostList, error) {
	assert.That(ctx != nil, "GetThread: nil context")
	assert.That(c.http != nil, "GetThread: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetThread: base URL must end at the API root")
	var pl model.PostList
	err := c.get(ctx, fmt.Sprintf("/posts/%s/thread", postID), &pl)
	return &pl, err
}

func (c *httpClient) GetCommands(ctx context.Context) ([]*model.Command, error) {
	assert.That(ctx != nil, "GetCommands: nil context")
	assert.That(c.http != nil, "GetCommands: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetCommands: base URL must end at the API root")
	var cmds []*model.Command
	err := c.get(ctx, "/commands", &cmds)
	return cmds, err
}

func (c *httpClient) ViewChannel(ctx context.Context, channelID string) error {
	assert.That(ctx != nil, "ViewChannel: nil context")
	assert.That(c.http != nil, "ViewChannel: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "ViewChannel: base URL must end at the API root")
	return c.post(ctx, fmt.Sprintf("/channels/%s/members/me/view", channelID), nil, nil)
}

func (c *httpClient) GetUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	assert.That(ctx != nil, "GetUsersByIDs: nil context")
	assert.That(c.http != nil, "GetUsersByIDs: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetUsersByIDs: base URL must end at the API root")
	assert.That(len(ids) > 0, "GetUsersByIDs: no IDs to look up")
	var users []*model.User
	err := c.post(ctx, "/users/ids", ids, &users)
	return users, err
}

func (c *httpClient) SearchPosts(ctx context.Context, channelID, term string, tagIDs []string) (*model.PostList, error) {
	assert.That(ctx != nil, "SearchPosts: nil context")
	assert.That(c.http != nil, "SearchPosts: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "SearchPosts: base URL must end at the API root")
	var pl model.PostList
	body := map[string]interface{}{"terms": term}
	if len(tagIDs) > 0 {
		body["tag_ids"] = tagIDs
	}
	err := c.post(ctx, fmt.Sprintf("/channels/%s/posts/search", channelID), body, &pl)
	return &pl, err
}

func (c *httpClient) SearchPostsEverywhere(ctx context.Context, term string, tagIDs []string) (*model.PostList, error) {
	assert.That(ctx != nil, "SearchPostsEverywhere: nil context")
	assert.That(c.http != nil, "SearchPostsEverywhere: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "SearchPostsEverywhere: base URL must end at the API root")
	var pl model.PostList
	// Like SearchPosts, tag_ids is sent only when there are some; nil
	// marshals as null.
	body := map[string]any{"terms": term}
	if len(tagIDs) > 0 {
		body["tag_ids"] = tagIDs
	}
	err := c.post(ctx, "/posts/search", body, &pl)
	return &pl, err
}

func (c *httpClient) GetChannelMembers(ctx context.Context, channelID string) ([]*model.ChannelMember, error) {
	assert.That(ctx != nil, "GetChannelMembers: nil context")
	assert.That(c.http != nil, "GetChannelMembers: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetChannelMembers: base URL must end at the API root")
	var members []*model.ChannelMember
	err := c.get(ctx, fmt.Sprintf("/channels/%s/members", channelID), &members)
	return members, err
}

func (c *httpClient) GetMyChannelMembers(ctx context.Context, teamID string) ([]*model.ChannelMember, error) {
	assert.That(ctx != nil, "GetMyChannelMembers: nil context")
	assert.That(c.http != nil, "GetMyChannelMembers: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetMyChannelMembers: base URL must end at the API root")
	var members []*model.ChannelMember
	err := c.get(ctx, fmt.Sprintf("/users/me/teams/%s/channels/members", teamID), &members)
	return members, err
}

func (c *httpClient) CreateDirectChannel(ctx context.Context, userID1, userID2 string) (*model.Channel, error) {
	assert.That(ctx != nil, "CreateDirectChannel: nil context")
	assert.That(c.http != nil, "CreateDirectChannel: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "CreateDirectChannel: base URL must end at the API root")
	var ch model.Channel
	err := c.post(ctx, "/channels/direct", []string{userID1, userID2}, &ch)
	return &ch, err
}

func (c *httpClient) GetMyDirectChannels(ctx context.Context) ([]*model.Channel, error) {
	assert.That(ctx != nil, "GetMyDirectChannels: nil context")
	assert.That(c.http != nil, "GetMyDirectChannels: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetMyDirectChannels: base URL must end at the API root")
	var channels []*model.Channel
	err := c.get(ctx, "/users/me/channels/direct", &channels)
	return channels, err
}

func (c *httpClient) SearchUsers(ctx context.Context, term string, page, perPage int) ([]*model.User, error) {
	assert.That(ctx != nil, "SearchUsers: nil context")
	assert.That(c.http != nil, "SearchUsers: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "SearchUsers: base URL must end at the API root")
	assert.That(page >= 0 && perPage > 0, "SearchUsers: page must be >= 0 and perPage > 0")
	var users []*model.User
	// The term is user input and goes into a query string, so it has to be
	// escaped: a space or an ampersand would otherwise split the parameter
	// and silently search for something else.
	q := url.Values{}
	q.Set("term", term)
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))

	err := c.get(ctx, "/users?"+q.Encode(), &users)
	return users, err
}

func (c *httpClient) CreateGroupChannel(ctx context.Context, userIDs []string) (*model.Channel, error) {
	assert.That(ctx != nil, "CreateGroupChannel: nil context")
	assert.That(c.http != nil, "CreateGroupChannel: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "CreateGroupChannel: base URL must end at the API root")
	assert.That(len(userIDs) >= 2, "CreateGroupChannel: a group needs at least two members")
	var ch model.Channel
	err := c.post(ctx, "/channels/group", userIDs, &ch)
	return &ch, err
}

func (c *httpClient) CreateChannel(ctx context.Context, channel *model.Channel) (*model.Channel, error) {
	assert.That(ctx != nil, "CreateChannel: nil context")
	assert.That(c.http != nil, "CreateChannel: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "CreateChannel: base URL must end at the API root")
	assert.That(channel != nil, "CreateChannel: nil channel")
	var ch model.Channel
	err := c.post(ctx, "/channels", channel, &ch)
	return &ch, err
}

// UpdatePost edits a post's text. The server rejects anyone but the author or
// a system admin.
func (c *httpClient) UpdatePost(ctx context.Context, postID, content string) (*model.Post, error) {
	assert.That(ctx != nil, "UpdatePost: nil context")
	assert.That(c.http != nil, "UpdatePost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "UpdatePost: base URL must end at the API root")
	var updated model.Post
	err := c.put(ctx, fmt.Sprintf("/posts/%s", postID),
		map[string]string{"content": content}, &updated)
	return &updated, err
}

// DeletePost soft-deletes a post.
func (c *httpClient) DeletePost(ctx context.Context, postID string) error {
	assert.That(ctx != nil, "DeletePost: nil context")
	assert.That(c.http != nil, "DeletePost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "DeletePost: base URL must end at the API root")
	return c.del(ctx, fmt.Sprintf("/posts/%s", postID))
}

func (c *httpClient) GetAllTags(ctx context.Context) ([]*model.Tag, error) {
	assert.That(ctx != nil, "GetAllTags: nil context")
	assert.That(c.http != nil, "GetAllTags: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetAllTags: base URL must end at the API root")
	var tags []*model.Tag
	err := c.get(ctx, "/tags", &tags)
	return tags, err
}

func (c *httpClient) CreateTag(ctx context.Context, name, scope, channelID string) (*model.Tag, error) {
	assert.That(ctx != nil, "CreateTag: nil context")
	assert.That(c.http != nil, "CreateTag: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "CreateTag: base URL must end at the API root")
	assert.That(scope == model.TagScopeChannel || scope == model.TagScopePrivate, "CreateTag: unknown scope")
	assert.That((scope == model.TagScopePrivate) == (channelID == ""), "CreateTag: a channel tag needs a channel and a private tag has none")
	var tag model.Tag
	body := map[string]string{"name": name, "scope": scope}
	if channelID != "" {
		body["channel_id"] = channelID
	}
	err := c.post(ctx, "/tags", body, &tag)
	return &tag, err
}

func (c *httpClient) GetTagsForPost(ctx context.Context, postID string) ([]*model.Tag, error) {
	assert.That(ctx != nil, "GetTagsForPost: nil context")
	assert.That(c.http != nil, "GetTagsForPost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetTagsForPost: base URL must end at the API root")
	var tags []*model.Tag
	err := c.get(ctx, fmt.Sprintf("/posts/%s/tags", postID), &tags)
	return tags, err
}

func (c *httpClient) GetTagsForPosts(ctx context.Context, postIDs []string) (map[string][]*model.Tag, error) {
	assert.That(ctx != nil, "GetTagsForPosts: nil context")
	assert.That(c.http != nil, "GetTagsForPosts: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "GetTagsForPosts: base URL must end at the API root")
	tags := make(map[string][]*model.Tag)
	if len(postIDs) == 0 {
		return tags, nil
	}
	err := c.post(ctx, "/tags/posts", map[string][]string{"post_ids": postIDs}, &tags)
	return tags, err
}

func (c *httpClient) AddTagToPost(ctx context.Context, postID, tagID string) error {
	assert.That(ctx != nil, "AddTagToPost: nil context")
	assert.That(c.http != nil, "AddTagToPost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "AddTagToPost: base URL must end at the API root")
	return c.post(ctx, fmt.Sprintf("/posts/%s/tags", postID), map[string]string{"tag_id": tagID}, nil)
}

func (c *httpClient) RemoveTagFromPost(ctx context.Context, postID, tagID string) error {
	assert.That(ctx != nil, "RemoveTagFromPost: nil context")
	assert.That(c.http != nil, "RemoveTagFromPost: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "RemoveTagFromPost: base URL must end at the API root")
	return c.del(ctx, fmt.Sprintf("/posts/%s/tags/%s", postID, tagID))
}

func (c *httpClient) AddChannelMember(ctx context.Context, channelID, userID string) error {
	assert.That(ctx != nil, "AddChannelMember: nil context")
	assert.That(c.http != nil, "AddChannelMember: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "AddChannelMember: base URL must end at the API root")
	return c.post(ctx, fmt.Sprintf("/channels/%s/members", channelID), map[string]string{"user_id": userID}, nil)
}

// RemoveChannelMember removes a user from a channel. Removing yourself is
// leaving; removing anyone else requires system-admin rights server-side.
func (c *httpClient) RemoveChannelMember(ctx context.Context, channelID, userID string) error {
	assert.That(ctx != nil, "RemoveChannelMember: nil context")
	assert.That(c.http != nil, "RemoveChannelMember: client has no http.Client")
	assert.That(strings.HasSuffix(c.baseURL, apiPrefix), "RemoveChannelMember: base URL must end at the API root")
	return c.del(ctx, fmt.Sprintf("/channels/%s/members/%s", channelID, userID))
}
