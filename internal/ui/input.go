package ui

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// ReadPassword reads a line of input from stdin without echoing it to the terminal.
func ReadPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Fallback for non-interactive shells
		var s string
		_, err := fmt.Scanln(&s)
		return s, err
	}
	bytePassword, err := term.ReadPassword(fd)
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}
