package dto

type Bot struct {
	ID       string
	DomainID int64
	Username string
	Name     string
	SchemaID string
	Metadata map[string]string
}

type CreateBotRequest struct {
	Username string
	Name     string
	SchemaID string
	Metadata map[string]string
}

type UpdateBotRequest struct {
	ID       string
	Username string
	Name     string
	SchemaID string
	Metadata map[string]string
	Fields   []string
}

type DeleteBotRequest struct {
	ID string
}
