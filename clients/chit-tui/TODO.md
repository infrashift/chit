# Phase 1 TODO

## Sub-task 0: Project Bootstrap
- [x] go.mod
- [x] Makefile (all, build, test, cover, lint, clean)
- [x] .golangci.yml
- [x] .gitignore
- [x] TODO.md

## Sub-task 1: Theme & Styles
- [x] Theme struct with Tokyo Night defaults
- [x] Lipgloss style constructors
- [x] Tests

## Sub-task 2: Model DTOs
- [x] All DTOs mirroring server JSON shapes
- [x] Tests (JSON round-trips, helpers)

## Sub-task 3: Test Utilities
- [x] Factory functions
- [x] Mock API server builder

## Sub-task 4: Configuration
- [x] Config struct, env var loading, defaults
- [x] Tests

## Sub-task 5: API Client - Errors & Transport
- [x] APIError, parseErrorResponse
- [x] authTransport
- [x] Tests

## Sub-task 6: API Client - Interface & Methods
- [x] ChitClient interface, httpClient struct
- [x] All endpoint methods
- [x] Tests

## Sub-task 7: WebSocket Client
- [x] WSClient interface, wsClient impl
- [x] Exponential backoff reconnect
- [x] Tests

## Sub-task 8: Tea Messages & Commands
- [x] All tea.Msg types
- [x] tea.Cmd wrappers
- [x] Keymap
- [x] Tests

## Sub-task 9: PostBubble Component
- [x] Post rendering with glamour
- [x] Tests

## Sub-task 10: Sidebar Component
- [x] Team/channel navigation
- [x] Unread badges
- [x] Tests

## Sub-task 11: Viewport Component
- [x] Post list viewport
- [x] Tests

## Sub-task 12: Input Component
- [x] Message input with textarea
- [x] Tests

## Sub-task 13: Thread Panel
- [x] Thread side panel
- [x] Tests

## Sub-task 14: Command Palette
- [x] Fuzzy-filter overlay
- [x] Tests

## Sub-task 15: Root Model Integration
- [x] App composition and focus management
- [x] Tests

## Sub-task 16: Entry Point
- [x] cmd/chit-tui/main.go

# Gap Fixes

## Gap 1: WebSocket Reconnection
- [x] readLoop uses Backoff to auto-reconnect on connection drop
- [x] Close() cancels reconnect loop via done channel
- [x] Tests (reconnect after drop, close stops reconnect)

## Gap 2: Unread Counts
- [x] GetChannelMembers API method
- [x] Fetch channel members when channels load
- [x] Compute unread = TotalMsgCount - MsgCount
- [x] Wire sidebar.SetUnread() with computed values
- [x] Tests

## Gap 3: Search UI
- [x] Search overlay component (internal/tui/search/)
- [x] Wire to SearchPosts API
- [x] Keybinding (Ctrl+S)
- [x] Tests

## Gap 4: Theme Loading
- [x] Name/Author fields on Theme struct
- [x] JSON theme file format + loader (ParseJSON, LoadFromFile)
- [x] ~/.config/chit/skins/ directory support (LoadNamed)
- [x] CHIT_THEME env var
- [x] Tests

# E2E Testing Infrastructure

## Configurable Auth Header
- [x] authTransport supports custom header name (headerName field)
- [x] NewClientWithHeader constructor for API client
- [x] NewWSClientWithHeader constructor for WS client
- [x] CreateDirectChannel API method
- [x] Unit tests for custom header transport

## E2E Container Infrastructure
- [x] Containerfiles/Containerfile.e2e (Go build + test runner)
- [x] scripts/e2e-entrypoint.sh (wait for chitd, run tests)
- [x] Makefile targets (e2e-build, e2e-build-server, test-e2e, test-e2e-clean)

## E2E Test Suite (tests/e2e/, build tag: e2e)
- [x] helpers_test.go (shared setup, test client constructors)
- [x] api_test.go (GetMe, GetMyTeams, GetMyChannels, CreateAndGetPost, GetChannelPosts, PinUnpinPost, GetThread, SearchPosts, GetUsersByIDs, ViewChannel, CreateDMAndPost)
- [x] ws_test.go (WSConnectAndReceiveEvent)

# Mention Feature

## Phase 1: Theme & Style Extensions
- [x] MentionBadge, MentionText, MentionSelfBg colors in all 4 themes
- [x] themeFile JSON tags + colorOrDefault fallbacks in toTheme()
- [x] 6 new styles: MentionBadge, MentionText, MentionSelfHighlight, AutocompletePanel, AutocompleteItem, AutocompleteActive
- [x] Tests updated for new colors and styles

## Phase 2: Mention Parsing Utility
- [x] internal/tui/mention/parse.go: ExtractMentionPrefix, FilterEntries, FindMentions
- [x] internal/tui/mention/parse_test.go: full TDD test suite

## Phase 3: Autocomplete Popup Component
- [x] internal/tui/mention/mention.go: Model with Show/Hide/Update/View
- [x] UserSelectedMsg emitted on Enter/Tab
- [x] internal/tui/mention/mention_test.go: full test suite

## Phase 4: Input @ Trigger Detection
- [x] AtTriggerMsg and AtDismissMsg message types
- [x] detectAtTrigger() after textarea update
- [x] ReplaceAtMention() method
- [x] Tests for @ trigger, dismiss, replace

## Phase 5: Post Mention Highlighting
- [x] currentUsername field in post.Model, viewport.Model, thread.Model
- [x] highlightMentions() post-processing in post.View()
- [x] SetCurrentUsername() on viewport and thread
- [x] app.go wires currentUsername on UserLoadedMsg
- [x] Tests for mention highlighting, self-mention, multiple mentions

## Phase 6: Sidebar Mention Badges + WebSocket + Root Wiring
- [x] MentionCounts map + SetMention() in sidebar
- [x] Mention badge rendering [@N] in sidebar View
- [x] channelMembers cache + mention model in app.go
- [x] AtTriggerMsg/AtDismissMsg/UserSelectedMsg handlers in app.go
- [x] Key interception when mention popup visible
- [x] WebSocketEventMentioned handler increments badge
- [x] ChannelViewedMsg clears mention count
- [x] Mention popup overlay in View
- [x] buildMentionEntries() with specials + channel members
- [x] Tests for WS mention, channel viewed, @ trigger/dismiss/select

# Thread Completion

## Reply Count Badges
- [x] replyCount field in post.Model
- [x] [N replies] badge in post.View()
- [x] threadCounts map in viewport with SetThreadCounts()
- [x] threadCounts cache in app.Model, wired to ThreadLoadedMsg and WS events
- [x] Tests

## WebSocketEventThreadUpdated Handler
- [x] Parse thread + post from event data
- [x] Update threadCounts and viewport
- [x] Append reply to open thread panel
- [x] Tests

## Test Coverage
- [x] viewport: SetCurrentUsername, Update (not focused / non-key / default key), SelectedPost (empty), View (focused)
- [x] thread: SetCurrentUsername, SetUsernames
- [x] post: reply count badge (with count, zero count)
- [x] app: WS edge cases (other channel, thread reply, thread_updated, increment thread count, ThreadLoadedMsg caches count)
- [x] Coverage >90% on post, viewport, thread

# Direct Messaging

## Phase 2: TUI — API Client Methods
- [x] GetMyDirectChannels(ctx) in ChitClient interface + httpClient
- [x] SearchUsers(ctx, term, page, perPage) in ChitClient interface + httpClient
- [x] Tests (TestGetMyDirectChannels, TestSearchUsers, TestCreateDirectChannel_Success)

## Phase 3: TUI — Messages & Commands
- [x] DMChannelsLoadedMsg, UserSearchResultsMsg, DMCreatedMsg message types
- [x] FetchDMChannels, SearchUsersCmd, CreateDMChannel command functions
- [x] Tests (3 new command tests + updated mockClient)

## Phase 4: TUI — DM Picker Overlay Component
- [x] internal/tui/dmpicker/dmpicker.go: Model with Open/Close/SetResults/Update/View
- [x] UserPickedMsg emitted on Enter with selected user
- [x] SearchTriggeredMsg emitted on Enter with search text
- [x] Up/Down navigation, Escape closes
- [x] internal/tui/dmpicker/dmpicker_test.go: 12 tests (95.7% coverage)

## Phase 5: TUI — Sidebar DM Section
- [x] DMChannels field, SetDMChannels(), SetDMDisplayName() on sidebar
- [x] Unified cursor navigation across channels + DMs
- [x] "Direct Messages" header rendered when DMs present
- [x] DM display name resolution (dmDisplayNames → DisplayName → Name fallback)
- [x] Badge rendering (unread + mention) on DM channels
- [x] Tests: 7 new sidebar tests (DM section, nav, select, display name, unread, bounds)

## Phase 6: TUI — Root Model Integration
- [x] FocusDMPicker focus area + dmPicker field in Model
- [x] Ctrl+D keybinding opens DM picker overlay
- [x] DMChannelsLoadedMsg handler: stores channels, sets sidebar, resolves names, fetches members
- [x] SearchTriggeredMsg → SearchUsersCmd, UserSearchResultsMsg → dmPicker.SetResults
- [x] UserPickedMsg → CreateDMChannel, DMCreatedMsg → switch to DM channel
- [x] resolveDMDisplayNames() parses userID1__userID2 name format
- [x] Status bar shows resolved username for DM channels
- [x] WebSocket channel_created for type D/G triggers FetchDMChannels
- [x] computeUnread() searches both channels and dmChannels
- [x] DM picker overlay centered in View()
- [x] Tests: 18 new app tests (Ctrl+D, escape, all message handlers, WS, display name, status bar, unread)

## Phase 7: E2E Tests
- [x] TestGetMyDirectChannels — create DM, fetch via GetMyDirectChannels, verify
- [x] TestSearchUsers — search by term, verify results

## Phase 1 (Server): List DM Channels Endpoint
- [x] GetDirectChannelsForUser in ChannelStore interface
- [x] SQL implementation in sqlstore/channel_store.go
- [x] App layer method in app/channel.go
- [x] getMyDirectChannels API handler in api/channel.go
- [x] GET /users/me/channels/direct route in api/api.go
- [x] Mock stubs in all test files (api, app, mcp)
- [x] Tests (TestGetMyDirectChannels, TestGetMyDirectChannels_Empty)

# Skin Command (Runtime Theme Hot-Swap)

## Theme Discovery
- [x] theme.ListAvailable() returns sorted built-in + custom skin names
- [x] Tests (TestListAvailable_IncludesBuiltins, TestListAvailable_IncludesCustomSkins)

## SetStyles Method on All Components
- [x] sidebar, viewport, input, thread, cmdpalette, search, mention, dmpicker
- [x] Cache invalidation in viewport and thread

## Skin Picker Overlay Component
- [x] internal/tui/skinpicker/skinpicker.go: Model with Open/Close/SetSkins/Update/View
- [x] SkinSelectedMsg emitted on Enter
- [x] j/k and up/down navigation, Escape closes
- [x] Tests: 13 tests (95.8% coverage)

## Root Model Integration
- [x] FocusSkinPicker focus area + skinPicker field
- [x] /skin interception in SlashTriggerMsg handler
- [x] SkinSelectedMsg handler: load theme, rebuild styles, propagate to all components
- [x] Key interception, overlay rendering
- [x] Tests: 5 new app tests (slash skin, slash other, skin selected, escape, key interception)

# Channel Creation

## Phase A: API Client
- [x] CreateGroupChannel(ctx, userIDs) → POST /channels/group
- [x] CreateChannel(ctx, channel) → POST /channels
- [x] Tests (TestCreateGroupChannel_Success, TestCreateChannel_Success)

## Phase B: Messages & Commands
- [x] GroupCreatedMsg, ChannelCreatedMsg message types
- [x] CreateGroupChannel, CreateChannel command functions
- [x] Tests (TestCreateGroupChannel_ReturnsGroupCreatedMsg, TestCreateChannel_ReturnsChannelCreatedMsg)
- [x] Updated mockClient with groupChannel, createdChannel fields

## Phase C: DM Picker Multi-Select (Group Channels)
- [x] selected []*model.User field, toggleSelected, isSelected
- [x] Tab toggles user selection (max 7)
- [x] Enter with 2+ selected emits GroupPickedMsg
- [x] Enter with 0-1 selected: existing DM behavior (UserPickedMsg)
- [x] Backspace on empty input removes last selected
- [x] Open() resets selected to nil
- [x] View: selected chips, [x]/[ ] markers, group hint
- [x] Tests: 8 new tests (97.1% coverage)

## Phase D: Channel Creator Component
- [x] internal/tui/chcreator/chcreator.go: overlay form with 4 fields
- [x] Auto-slug: display name → lowercase, spaces→hyphens, strip invalid
- [x] Type toggle: Open/Private via Left/Right arrows
- [x] Validation: display name required, name regex, purpose max 250
- [x] Tab/Shift+Tab field navigation, manual name disables auto-slug
- [x] ChannelSubmittedMsg emitted on valid submit
- [x] Tests: 17 tests (90.7% coverage)

## Phase E: App Integration
- [x] FocusChCreator focus area + chCreator field in Model
- [x] Ctrl+N opens chCreator with activeTeam.ID (no-op without team)
- [x] Key interception when chCreator visible
- [x] GroupPickedMsg → CreateGroupChannel with self ID prepended
- [x] GroupCreatedMsg → set active, prepend to dmChannels, resolve names
- [x] ChannelSubmittedMsg → CreateChannel
- [x] ChannelCreatedMsg → prepend to channels, update sidebar, fetch posts/members
- [x] resolveDMDisplayNames extended for ChannelGroup type
- [x] statusBar extended for group channels (uses sidebar.GetDMDisplayName)
- [x] sidebar.GetDMDisplayName getter added
- [x] WS channel_created extended: O/P types → FetchChannels if team matches
- [x] chCreator added to setFocus, delegateKey, resizeComponents, View overlay
- [x] SetStyles call in SkinSelectedMsg handler
- [x] NewChannel keybinding (ctrl+n) in keymap
- [x] Tests: 13 new app tests

# Theme-Aware Markdown Rendering

## Markdown Style Package
- [x] internal/tui/ui/markdown/markdown.go: StyleConfig(theme) → ansi.StyleConfig
- [x] Color mapping: headings (H1=Username, H2=Warning, H3=Success, H4=Accent, H5=Subtle, H6=Muted)
- [x] Links, code, emphasis, lists, blockquotes, horizontal rules styled from theme
- [x] Chroma syntax highlighting mapped to theme colors
- [x] Tests: 5 tests (100% coverage)

## Styles Integration
- [x] MarkdownStyleConfig field added to Styles struct
- [x] Set via markdown.StyleConfig(t) in New()
- [x] Test assertion for MarkdownStyleConfig.Document.Color

## Viewport & Thread Renderer Fix
- [x] ensureRenderer() uses glamour.WithStyles(m.styles.MarkdownStyleConfig)
- [x] WithPreservedNewLines() and WithEmoji() enabled
- [x] SetStyles() resets renderer (m.renderer = nil) — fixes theme-switch bug
- [x] Both viewport and thread updated identically

## Multiline Chat Input
- [x] Alt+Enter inserts newline in textarea
- [x] Plain Enter still sends message (unchanged behavior)
- [x] Updated placeholder text with Alt+Enter hint
- [x] Tests (Alt+Enter newline, multiline send, plain Enter)

## Post Test Update
- [x] testRenderer() uses markdown.StyleConfig(theme.TokyoNight()) instead of "dark"

# Sidebar Init Bug Fix & Channel Creation Docs

## Bug Fix: Sidebar Focus Initialization
- [x] NewModel() calls sidebar.Focus() after construction so sidebar responds to keys immediately
- [x] TestModel_SidebarResponsiveOnInit — verifies Enter works without Tab cycling

## Documentation: Channel Creation Keybindings
- [x] Ctrl+N and Ctrl+D added to Global Keybindings table
- [x] Channel Creator section added to keybindings reference
- [x] DM Picker section added to keybindings reference

## Documentation: UAT Tutorial Fixes
- [x] Scenario 2 rewritten to account for auto-selection (sidebar starts in channels view)
- [x] Scenario 11: Create a Team Channel (Ctrl+N)
- [x] Scenario 12: Create a Direct Message (Ctrl+D)
- [x] Scenario 13: Create a Group Channel (Ctrl+D multi-select)

# Tag Integration Foundation

## B1: API Client — 5 Tag Methods
- [x] GetAllTags, CreateTag, GetTagsForPost, AddTagToPost, RemoveTagFromPost in ChitClient interface
- [x] `del` helper method on httpClient (DELETE requests)
- [x] All 5 method implementations on httpClient
- [x] 5 tests (TestGetAllTags, TestCreateTag, TestGetTagsForPost, TestAddTagToPost, TestRemoveTagFromPost)

## B2: Tea Messages & Commands
- [x] AllTagsLoadedMsg, TagCreatedMsg, PostTagsLoadedMsg, TagAddedToPostMsg, TagRemovedFromPostMsg
- [x] FetchAllTags, CreateTagCmd, FetchPostTags, AddTagToPostCmd, RemoveTagFromPostCmd, CreateTagAndApplyCmd
- [x] Updated mockClient with allTags, createdTag, postTags fields + 5 interface methods
- [x] 5 tests (FetchAllTags, CreateTagCmd, FetchPostTags, AddTagToPostCmd, RemoveTagFromPostCmd)

## B3: Theme & Styles — Tag Badge
- [x] TagBadge color in Theme struct + themeFile JSON tag
- [x] TagBadge color set in all 4 themes (TokyoNight, Catppuccin, Kanagawa, Nightfox)
- [x] colorOrDefault fallback in toTheme()
- [x] TagBadge lipgloss.Style in Styles struct + initialization in New()

## B4: Post Tag Display
- [x] tags []string field in post.Model
- [x] Updated New() signature with tags parameter
- [x] Tag badge rendering (#tagName) in View() after reply count badges
- [x] All callers updated (viewport, thread, tests)
- [x] 2 new tests (TestPostView_TagBadges, TestPostView_NoTags)

## B5: Tag Picker Overlay Component
- [x] internal/tui/tagpicker/tagpicker.go: Model with Open/Close/Update/View
- [x] Filter tags by prefix in text input
- [x] [x]/[ ] checkboxes for applied/unapplied tags
- [x] Enter toggles tag → TagToggledMsg
- [x] Ctrl+N creates new tag → TagCreateRequestMsg
- [x] Escape closes overlay
- [x] Tests: open/close, view, escape, toggle, cursor nav, ctrl+n create

## B6: Hashtag Input Parsing
- [x] internal/tui/tagpicker/parse.go: ExtractHashtags, StripHashtags
- [x] Regex: (?:^|\s)#([a-zA-Z0-9_-]+)
- [x] Tests: extraction, stripping, edge cases (no tags, inline, multiple, trailing)

## B7: Root Model Integration
- [x] FocusTagPicker focus area constant
- [x] tagPicker, allTags, postTags, pendingPostTags fields in Model
- [x] TagPicker key binding ("t") in keymap
- [x] FetchAllTags in Init()
- [x] Tag picker key interception when visible
- [x] "t" in FocusViewport opens tag picker for selected post
- [x] AllTagsLoadedMsg, PostTagsLoadedMsg handlers
- [x] TagToggledMsg → AddTagToPostCmd/RemoveTagFromPostCmd
- [x] TagCreateRequestMsg → CreateTagAndApplyCmd
- [x] TagCreatedMsg → append to allTags
- [x] TagAddedToPostMsg / TagRemovedFromPostMsg → re-fetch post tags
- [x] Hashtag flow: StripHashtags in input.SendMsg, pending tags applied in PostCreatedMsg
- [x] PostsLoadedMsg dispatches FetchPostTags for each post
- [x] Tag picker overlay in View()
- [x] Wired into setFocus, resizeComponents, delegateKey, SkinSelectedMsg

## B8: Viewport & Thread Tag Display
- [x] postTags map[string][]string in viewport Model
- [x] SetPostTags(postID, tagNames) invalidates cache + re-renders
- [x] postTagNames helper passes tags to post.New()
- [x] Thread passes nil tags to post.New()

## A1-A2: Server Store — Tag-Based Search
- [x] GetPostIDsByTags and FilterPostIDsByTags in TagStore interface
- [x] SQL implementations with AND semantics (HAVING COUNT(DISTINCT) = N)
- [x] Pagination support with LIMIT/OFFSET
- [x] Soft-delete exclusion (delete_at = 0)

## A3: Server App — Extended SearchPosts
- [x] SearchPosts accepts tagIDs []string parameter
- [x] Three branches: tag-only, text-only, hybrid (text + tag filter)
- [x] Hybrid: ZincSearch over-fetch 3x, then FilterPostIDsByTags

## A4: Server API — Extended Search Request Bodies
- [x] TagIDs []string field in searchPostsInTeam and searchPostsInChannel
- [x] Validation: at least one of Terms or TagIDs required
- [x] Backward compatible (omitting tag_ids preserves existing behavior)

## A5: Server Mocks
- [x] All 4 mock TagStores updated with GetPostIDsByTags and FilterPostIDsByTags stubs

## B9: Search Extension (TUI)
- [x] SearchPosts interface signature extended with tagIDs []string parameter
- [x] httpClient.SearchPosts sends "terms" and "tag_ids" in body
- [x] SearchPosts command accepts and passes tagIDs
- [x] search.SubmitMsg handler parses #tag syntax via StripHashtags
- [x] Resolved tag names to IDs from allTags before sending search
- [x] All mocks and e2e tests updated for new signature

# Search Overlay Rendering & Error Feedback Fix

## ANSI-Aware Overlay
- [x] Rewrote placeOverlay() to use charmbracelet/x/ansi (Truncate, TruncateLeft, StringWidth)
- [x] No more rune-by-rune replacement — ANSI escape sequences preserved intact
- [x] Fixes all 7 overlay components (search, cmd palette, mention, DM picker, skin picker, channel creator, tag picker)
- [x] 6 overlay tests: BasicASCII, ANSIAware, ForegroundWiderThanBackground, OverlayAtEdge, BackgroundShorterThanX, BeyondHeight
- [x] placeOverlay at 100% coverage

## Search Error Feedback
- [x] Added `err` field to search.Model
- [x] SetError() method sets error text
- [x] Open() and SetResults() clear error
- [x] View() renders error via styles.ErrorText instead of "Press Enter to search"
- [x] SearchResultsMsg handler in app.go calls m.search.SetError() on error
- [x] 3 new search tests: SetError, ErrorClearedOnResults, ErrorClearedOnOpen
- [x] Search package at 90.8% coverage

## Dependency
- [x] Promoted charmbracelet/x/ansi from indirect to direct via go mod tidy

# Bug Fixes

## Unread Badge Not Appearing on Esc Back to Teams
- [x] Added BackToTeamsMsg in sidebar package
- [x] Sidebar Esc/Backspace handler emits BackToTeamsMsg when navigating from channels to teams
- [x] Root model handles BackToTeamsMsg by clearing activeChan to nil
- [x] WS posted events now correctly increment unread when user is in team list view
- [x] ChannelsLoadedMsg only auto-selects first channel on initial load (m.channels == nil)
- [x] Re-loading channels (team re-selection, WS channel_created) preserves unread badges
- [x] Sidebar tests: BackToTeamsMsg emission on Esc, no-op in team view, TeamSelect updated
- [x] App tests: BackToTeamsMsg clears activeChan, channels reload skips auto-select

## Unread Badge Shows Total Count Instead of Missed Count
- [x] Server: UpdateLastViewedAt now also syncs msg_count to channel's total_msg_count and resets mention_count
- [x] TUI: BackToTeamsMsg calls ViewChannel for the departing channel before clearing activeChan
- [x] TUI: ChannelSelectedMsg calls ViewChannel for the old channel before switching to the new one
- [x] computeUnread (TotalMsgCount - MsgCount) now gives correct results because server keeps MsgCount in sync

# Private Channel Member Selection + Open Channel Broadcast Fix

## Part 1: AddChannelMember API Method
- [x] AddChannelMember(ctx, channelID, userID) in ChitClient interface
- [x] httpClient implementation (POST /channels/{id}/members)
- [x] TestAddChannelMember

## Part 2: Member Picker for Private Channel Creation
- [x] Mode type (ModeDM / ModeMemberPicker) in dmpicker
- [x] OpenForMembers() opens picker in member-picker mode
- [x] MembersPickedMsg emitted on Enter in member-picker mode
- [x] Mode-specific hint text ("add members" vs "create group")
- [x] GetMode() accessor
- [x] 5 new dmpicker tests (mode, MembersPickedMsg, single user, hint text, mode reset)
- [x] ChannelMemberAddedMsg, AllMembersAddedMsg message types
- [x] AddChannelMemberCmd, AddChannelMembersCmd command functions
- [x] 2 new command tests
- [x] pendingPrivateChannel, pendingMembers fields in app.Model
- [x] ChannelSubmittedMsg: Private → open member picker; Open → create immediately
- [x] MembersPickedMsg → store members, create channel
- [x] ChannelCreatedMsg: if pendingMembers set, call AddChannelMembersCmd
- [x] AllMembersAddedMsg → refresh channel members
- [x] ChannelMemberAddedMsg → handle errors
- [x] Escape from dmpicker with pending channel → create without extra members
- [x] Updated mockClient with AddChannelMember method

## Part 2b: WS user_added Handler
- [x] handleWSEvent handles WebSocketEventUserAdded
- [x] When current user is added, triggers FetchChannels to refresh sidebar
- [x] Ignores events for other users
- [x] TestModel_WSEventUserAddedRefetchesChannels
- [x] TestModel_WSEventUserAddedIgnoresOtherUser

## Part 3: Open Channel Broadcast (Server-Side)
- [x] CreateChannel auto-adds all team members for Open channels
- [x] Fetches team members via Store.Team().GetMembers()
- [x] Skips creator (already added)
- [x] Writes Keto relations for each member
- [x] TestCreateChannel_OpenAutoAddsTeamMembers
- [x] TestCreateChannel_PrivateOnlyAddsCreator

# Ory Kratos Authentication

## Phase 1: Token Store + Auth Package
- [x] internal/auth/tokenstore.go: thread-safe TokenStore (Get/Set with RWMutex)
- [x] internal/auth/kratos.go: KratosClient with InitLoginFlow, SubmitLogin, CheckSession
- [x] internal/auth/persist.go: SaveSession, LoadSession, ClearSession (~/.config/chit-tui/session.json, 0600)
- [x] DTOs: LoginFlow, Session, Identity, Traits, KratosError, StoredSession
- [x] Tests: tokenstore (concurrent, initial), kratos (success, wrong pw, expired flow, network, server error), persist (save/load/clear/permissions/corrupt/no-file/overwrite)
- [x] Coverage: 90.4%

## Phase 2: API + WebSocket Client Refactoring
- [x] NewClientWithTokenFn(baseURL, tokenFn, headerName) constructor
- [x] IsUnauthorized(err) bool — uses errors.As to check *APIError 401
- [x] SetToken(token) on WSClient interface + implementation (mutex-protected)
- [x] WS Connect() and reconnect() read token under lock
- [x] Tests: dynamic token changes, IsUnauthorized (401, 403, wrapped, non-API, nil)

## Phase 3: Config Refactoring
- [x] SessionToken optional in Validate() (no longer blocks startup if empty)
- [x] HasToken() bool helper
- [x] KratosBaseURL() string (ServerURL + "/kratos")
- [x] Tests updated: MissingToken no longer error, HasToken, KratosBaseURL

## Phase 4: Login TUI Component
- [x] internal/tui/login/login.go: Elm Architecture (Model/Update/View)
- [x] States: Idle, Loading, Error
- [x] Fields: identifier (textinput), password (textinput, masked)
- [x] Keys: Tab/Shift-Tab/Up/Down cycle fields, Enter submits, loading ignores keys
- [x] View: centered box with styled fields, error display, keybinding hints
- [x] internal/tui/login/commands.go: InitLoginAndSubmit, ValidateSession
- [x] LoginSuccessMsg{Token, ExpiresAt}, LoginErrorMsg{Err}
- [x] Tests: tab cycling, empty validation, error display, loading state, navigation, commands (success/flow-error/login-error/validate)
- [x] Coverage: 100%

## Phase 5: Root Model Integration
- [x] AppState enum: AppStateLogin, AppStateReLogin, AppStateRunning
- [x] NewModel extended with tokenStore, kratosClient params (nil-safe)
- [x] Init(): skips API init when in login state
- [x] Update(): delegates to login model in login/re-login state
- [x] handleLoginSuccess: updates tokenStore, persists session, sets WS token, transitions to running
- [x] handleAuthExpired: transitions to re-login, clears stored session, closes WS
- [x] handleLogout: clears token + session, transitions to login, closes WS
- [x] View(): renders login screen full-screen in login/re-login state
- [x] 401 detection on UserLoaded, TeamsLoaded, ChannelsLoaded, PostsLoaded, PostCreated
- [x] AuthExpiredMsg and LogoutMsg message types
- [x] All existing NewModel calls updated with nil, nil
- [x] All existing tests pass

## Phase 6: Polish
- [x] Startup session validation: stored token validated via CheckSession before TUI starts
- [x] /logout slash command: triggers handleLogout from input
- [x] Security: session file at 0600, password cleared after error, password masked with *
- [x] cmd/chit-tui/main.go: new startup flow (env var > stored session > login screen)

## Documentation Updates
- [x] UAT tutorial: updated auth flow (Kratos login via Oathkeeper, not X-User-Id)
- [x] UAT tutorial: added make uat-seed-kratos step, Oathkeeper proxy endpoints
- [x] UAT tutorial: added Scenarios 19-21 (session persistence, logout, login error handling)
- [x] UAT tutorial: updated two-terminal scenario for session clearing between logins
- [x] Configuration docs: CHIT_SESSION_TOKEN now optional, added session management section
- [x] Quick Start: updated for interactive login flow (email/password)

# Status Bar Fixes & Search "No Results" Feedback

## Fix 1: Auto-dismiss error messages after 10 seconds
- [x] ClearErrMsg message type in messages.go
- [x] errSeq counter + setError() helper on Model (tea.Tick 10s auto-clear)
- [x] ClearErrMsg handler in Update() (clears only if seq matches)
- [x] All 21 `m.err = msg.Err` sites replaced with m.setError(msg.Err)
- [x] Tests: ErrorMsg returns cmd, ClearErrMsg clears error, stale seq ignored

## Fix 2: Username truncation in status bar
- [x] Gap filler uses zero-padding style (StatusBar.Padding(0,0)) to avoid +2 width overflow
- [x] Removed unused repeatStr helper
- [x] Test: full username visible in status bar

## Fix 3: "No matches found" feedback in search
- [x] submitted bool field in search.Model
- [x] Set true on Enter/submit, reset on Open(), reset when input value changes
- [x] View: "No matches found" when submitted + empty results, "Press Enter to search" otherwise
- [x] Tests: NoMatchesFound, PressEnterToSearch, EditAfterNoMatches

# Search "No Matches Found" & Hashtag-only Post 500 Error

## Fix 1: Search ZincSearch empty results fallback (Server)
- [x] `hasQuery && !hasTags`: fall back to SQL when zincSearch returns empty results (len check)
- [x] `hasQuery && hasTags`: fall back to SQL+tag filter when zincSearch returns empty results
- [x] Tests: 5 new search_test.go tests (SQL fallback, text-only, empty query, tag-only, hybrid fallback)

## Fix 2: Hashtag-only post preserves content (TUI)
- [x] When StripHashtags returns empty content but found tags, use original content
- [x] Tests: HashtagOnlyPreservesContent, HashtagWithTextStripsCorrectly

## Fix 3: createPost API error propagation (Server)
- [x] Check if error is already *model.AppError before wrapping as 500
- [x] Preserves validation errors (400) from app layer

# Day Separators in Chat Viewport

## Styles
- [x] DaySeparator lipgloss.Style in Styles struct (Foreground: t.Muted)

## Viewport Helpers & Rendering
- [x] sameDay(a, b time.Time) bool — calendar date comparison
- [x] formatDaySeparator(t, width) — centered ───── Day YYYY-MM-DD ───── label
- [x] updateContent() inserts styled separator between posts on different days

## Tests
- [x] separator_test.go: TestSameDay_True, TestSameDay_False, TestFormatDaySeparator, TestFormatDaySeparator_NarrowWidth
- [x] viewport_test.go: TestViewport_DaySeparator_MultiDay, TestViewport_DaySeparator_SameDay, TestViewport_DaySeparator_SinglePost
- [x] Coverage: 92% overall viewport, 100% on new helpers
