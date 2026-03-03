package model

// Permission constants for RBAC and Keto integration.
const (
	PermissionCreateTeam        = "create_team"
	PermissionManageTeam        = "manage_team"
	PermissionCreateChannel     = "create_channel"
	PermissionManageChannel     = "manage_channel"
	PermissionCreatePost        = "create_post"
	PermissionEditPost          = "edit_post"
	PermissionDeletePost        = "delete_post"
	PermissionManageChannelTags = "manage_channel_tags"
	PermissionViewChannel       = "view_channel"

	// Keto namespace and relations
	KetoNamespaceChannel = "chit/channel"
	KetoRelationMember   = "member"
	KetoRelationWriter   = "writer"
	KetoRelationReader   = "reader"
)
