package model

import "strings"

type AuthContact struct {
	DC        int64
	ContactID string
	Sub       string
	Iss       string
	Name      string
}

func (c *AuthContact) GetContactIDPtr() *string {
	if c == nil {
		return nil
	}

	contactID := strings.Clone(c.ContactID)

	return &contactID
}
