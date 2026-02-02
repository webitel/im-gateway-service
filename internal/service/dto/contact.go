package dto

type Contact struct {
	ID        string
	IssID     string
	AppID     string
	Type      string
	Name      string
	Username  string
	Metadata  map[string]string
	CreatedAt int64
	UpdatedAt int64
	Subject   string
}
type SearchContactRequest struct {
	Page     int
	Size     int
	Q        string
	Sort     string
	Fields   []string
	AppID    []string
	IssID    []string
	Type     []string
	IDs      []string
	Subjects []string
	DomainID int
}

type ContactList struct {
	Page  int
	Size  int
	Next  bool
	Items []*Contact
}
