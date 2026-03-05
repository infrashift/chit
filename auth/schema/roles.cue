package auth

#Role: {
	name:             string & =~"^[a-z0-9-]+$"
	allowed_commands: [...string]
}
