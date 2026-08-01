// Schema for ~/.config/chit/themes/<name>.toml.
//
// Unlike config.toml, a theme file must be complete: every palette key is
// required, so a theme can never render half-styled because of a typo. The
// three derived keys are optional — when absent they are computed from the
// palette.

// A color is either a hex literal or one of the terminal's own palette
// entries. A named color follows whatever the user's terminal theme sets it
// to, which is how a theme can blend into an existing colour scheme.
#ThemeNamedColor: "black" | "red" | "green" | "yellow" | "blue" | "magenta" |
	"cyan" | "gray" | "grey" | "darkgray" | "dark_gray" | "darkgrey" |
	"dark_grey" | "lightred" | "light_red" | "lightgreen" | "light_green" |
	"lightyellow" | "light_yellow" | "lightblue" | "light_blue" |
	"lightmagenta" | "light_magenta" | "lightcyan" | "light_cyan" | "white"

#ThemeColor: =~"^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$" | #ThemeNamedColor

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
