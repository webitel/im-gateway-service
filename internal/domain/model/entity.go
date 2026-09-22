package model

// EntityType is the kind of formatting span over a message's plain-text body.
// Values correspond to markdown formatting: bold, italic, strikethrough, inline code,
// code blocks (pre), and links.
type EntityType string

const (
	EntityTypeBold          EntityType = "BOLD"
	EntityTypeItalic        EntityType = "ITALIC"
	EntityTypeStrikethrough EntityType = "STRIKETHROUGH"
	EntityTypeCode          EntityType = "CODE"
	EntityTypePre           EntityType = "PRE"
	EntityTypeLink          EntityType = "LINK"
)

// Entity is a single formatting span over a message's plain-text body.
// Offset and Length are UTF-8 BYTE offsets/lengths into the plain-text string,
// not rune/codepoint offsets. This allows correct handling of multi-byte UTF-8
// characters (e.g. Cyrillic, emoji).
// Value is populated only for EntityTypeLink (holds the validated URL) and is nil
// for all other entity types.
type Entity struct {
	Type   EntityType
	Offset int32
	Length int32
	Value  *string
}
