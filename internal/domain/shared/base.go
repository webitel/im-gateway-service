// Package model defines the core domain entities and business logic rules
// for the IM Thread service. This package is the heart of the application
// and must remain independent of any external frameworks or transport layers.
package shared

import (
	"time"

	"github.com/google/uuid"
)

type PeerType int16

//go:generate stringer -type=PeerType
const (
	PeerContact PeerType = iota + 1
	PeerGroup
	PeerChannel
	PeerThread
)

type Peer struct {
	ID     string
	Issuer string
	Type   PeerType
}

type BaseModel struct {
	ID        uuid.UUID `json:"id"`
	DomainID  int       `json:"domain_id"`
	CreatedBy int       `json:"created_by"`
	UpdatedBy int       `json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Entity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Value  string `json:"value"`
}
