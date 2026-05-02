// Package tui implements a terminal user interface for GophKeeper.
package tui

import "github.com/user/gophkeeper/internal/client/model"

type (
	// LoginData holds plaintext login credentials.
	LoginData = model.LoginData
	// TextData holds plaintext text content.
	TextData = model.TextData
	// BinaryData holds a filename and its binary content.
	BinaryData = model.BinaryData
	// CardData holds plaintext payment card information.
	CardData = model.CardData
)
