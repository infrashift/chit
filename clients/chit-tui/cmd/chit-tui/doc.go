// Command chit-tui is the terminal client for Chit. It reads the
// configuration and flags, resolves the theme before the program starts
// (asking the terminal for its background would otherwise leak the reply
// into the input), restores a stored session, and runs the Bubble Tea
// program.
package main
