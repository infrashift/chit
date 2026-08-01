// Schema for ~/.config/chit/config.toml.
//
// Every key is optional: an absent key falls back to its built-in default, and
// an invalid one is dropped with a warning rather than aborting startup. Theme
// names are only typed as string here — whether a name resolves is decided
// later, since it may name a user's own theme file on disk.

#Config: {
	// Connection.
	server_url?:  string
	ws_scheme?:   "ws" | "wss"
	auth_header?: string
	session_file?: string

	// Appearance. theme wins outright; theme_dark and theme_light are picked
	// between by appearance when no explicit theme is set.
	theme?:       string
	theme_dark?:  string
	theme_light?: string
	appearance?:  "dark" | "light" | "system"
}
