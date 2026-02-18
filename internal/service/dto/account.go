package dto

import "google.golang.org/grpc/metadata"

type GrantTyper interface {
	isTokenRequest_GrantType()
}

type TokenRequest struct {
	State        string
	Scope        []string
	ClientID     string
	ClientSecret string
	GrantType    GrantTyper
	Headers metadata.MD
}

type IdentityGrant struct {
	Iss                 string
	Sub                 string
	Name                string
	GivenName           string
	MiddleName          string
	FamilyName          string
	Birthdate           string
	Zoneinfo            string
	Profile             string
	Picture             string
	Gender              string
	Locale              string
	Email               string
	EmailVerified       bool
	PhoneNumber         string
	PhoneNumberVerified bool
	Metadata            map[string]any
}

func (i *IdentityGrant) isTokenRequest_GrantType() {
}

type Code struct {
	Code string
}

func (i *Code) isTokenRequest_GrantType() {
}

type RefreshToken struct {
	RefreshToken string
}

func (i *RefreshToken) isTokenRequest_GrantType() {
}

type Authorization struct {
	Dc      int64
	Id      string
	Date    int64
	Name    string
	AppId   string
	Device  *Device
	Contact *AuthContact
	Token   *AccessToken
	Current bool
}

type Device struct {
	Id  string
	Ip  string
	App *UserAgent
}

type UserAgent struct {
	Name      string
	Version   string
	Os        string
	OsVersion string
	Device    string
	Mobile    bool
	Tablet    bool
	Desktop   bool
	Bot       bool
	String_   string
}

type AuthContact struct {
	Dc                  int64
	Id                  string
	Iss                 string
	Sub                 string
	App                 string
	Type                string
	Name                string
	GivenName           string
	MiddleName          string
	FamilyName          string
	Username            string
	Birthdate           string
	Zoneinfo            string
	Profile             string
	Picture             string
	Gender              string
	Locale              string
	Email               string
	EmailVerified       bool
	PhoneNumber         string
	PhoneNumberVerified bool
	Metadata            map[string]any
	CreatedAt           int64
	UpdatedAt           int64
	DeletedAt           int64
}

type AccessToken struct {
	TokenType    string
	AccessToken  string
	RefreshToken string
	ExpiresIn    int32
	Scope        []string
	State        string
}
