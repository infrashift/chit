// Schema for ~/.config/chit/themes/<name>.toml.
//
// Unlike config.toml, a theme file must be complete: every palette key is
// required, so a theme can never render half-styled because of a typo. The
// three derived keys are optional — when absent they are computed from the
// palette.

#ThemeColor: =~"^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$"

#Theme: {
	name!:   string
	author?: string

	background!:     #ThemeColor
	foreground!:     #ThemeColor
	subtle!:         #ThemeColor
	accent!:         #ThemeColor
	error!:          #ThemeColor
	success!:        #ThemeColor
	warning!:        #ThemeColor
	border!:         #ThemeColor
	active_border!:  #ThemeColor
	highlight!:      #ThemeColor
	muted!:          #ThemeColor
	username!:       #ThemeColor
	timestamp!:      #ThemeColor
	unread_badge!:   #ThemeColor
	pin_badge!:      #ThemeColor
	channel_active!: #ThemeColor
	mention_badge!:  #ThemeColor
	mention_text!:   #ThemeColor
	mention_self_bg!: #ThemeColor
	tag_badge!:      #ThemeColor

	// Derived when omitted.
	selection?:           #ThemeColor
	search_match?:        #ThemeColor
	search_match_active?: #ThemeColor
}
