package auth

#Command: {
	id:          string & =~"^[a-z0-9-]+$"
	slug:        string & =~"^[a-z0-9-]+$"
	description: string
	category:    "chat" | "admin" | "agent"
}

registry: [string]: #Command

registry: {
	help: {
		id:          "help"
		slug:        "help"
		description: "List available commands"
		category:    "chat"
	}
	invite: {
		id:          "invite"
		slug:        "invite"
		description: "Invite a user or agent to the channel"
		category:    "chat"
	}
	kick: {
		id:          "kick"
		slug:        "kick"
		description: "Remove a user from the channel"
		category:    "admin"
	}
	topic: {
		id:          "topic"
		slug:        "topic"
		description: "Set the channel topic/header"
		category:    "chat"
	}
	summarize: {
		id:          "summarize"
		slug:        "summarize"
		description: "Generate a TL;DR of channel history"
		category:    "agent"
	}
}
