package api

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPISpec_NonEmpty(t *testing.T) {
	if len(OpenAPISpec) == 0 {
		t.Fatal("OpenAPISpec is empty")
	}
}

func TestOpenAPISpec_ValidOpenAPI(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(OpenAPISpec)
	if err != nil {
		t.Fatalf("failed to load OpenAPI spec: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("OpenAPI spec validation failed: %v", err)
	}
}

func TestOpenAPISpec_AllEndpoints(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(OpenAPISpec)
	if err != nil {
		t.Fatalf("failed to load OpenAPI spec: %v", err)
	}

	// Collect all operationIds from the spec.
	foundOps := make(map[string]bool)
	for _, pathItem := range doc.Paths.Map() {
		for _, op := range []*openapi3.Operation{
			pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Patch,
			pathItem.Delete, pathItem.Head, pathItem.Options,
		} {
			if op != nil && op.OperationID != "" {
				foundOps[op.OperationID] = true
			}
		}
	}

	expectedOps := []string{
		"system_ping",
		"system_client_config",
		"create_user",
		"get_user",
		"get_user_by_username",
		"get_me",
		"update_me",
		"search_users",
		"get_users_by_ids",
		"create_team",
		"get_team",
		"update_team",
		"delete_team",
		"get_all_teams",
		"get_my_teams",
		"add_team_member",
		"remove_team_member",
		"get_team_members",
		"create_channel",
		"get_channel",
		"update_channel",
		"delete_channel",
		"get_channels_for_team",
		"get_my_channels",
		"add_channel_member",
		"remove_channel_member",
		"get_channel_members",
		"view_channel",
		"create_direct_channel",
		"create_group_channel",
		"create_post",
		"get_post",
		"update_post",
		"delete_post",
		"get_channel_posts",
		"pin_post",
		"unpin_post",
		"get_pinned_posts",
		"get_thread",
		"get_my_threads",
		"mark_thread_as_read",
		"update_thread_following",
		"search_posts_in_channel",
		"search_posts_in_team",
		"create_tag",
		"get_all_tags",
		"add_tag_to_post",
		"remove_tag_from_post",
		"get_tags_for_post",
		"handle_web_socket",
	}

	for _, opID := range expectedOps {
		if !foundOps[opID] {
			t.Errorf("missing operationId: %s", opID)
		}
	}
}

func TestOpenAPISpec_AllSchemas(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(OpenAPISpec)
	if err != nil {
		t.Fatalf("failed to load OpenAPI spec: %v", err)
	}

	expectedSchemas := []string{
		"User",
		"Team",
		"Channel",
		"Post",
		"PostList",
		"Thread",
		"ThreadResponse",
		"UserThreadList",
		"Tag",
		"AppError",
		"TeamMember",
		"ChannelMember",
		"ThreadMembership",
		"MessageTag",
		"WebSocketEvent",
		"WebSocketBroadcast",
	}

	schemas := doc.Components.Schemas
	for _, name := range expectedSchemas {
		if schemas[name] == nil {
			t.Errorf("missing schema: %s", name)
		}
	}
}
