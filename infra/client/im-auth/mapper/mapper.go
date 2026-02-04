package mapper

import (
	"google.golang.org/protobuf/types/known/structpb"

	authv1 "github.com/webitel/im-gateway-service/gen/go/auth/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// goverter:converter
// goverter:matchIgnoreCase
// goverter:output:file ./generated/in.go
// goverter:extend ParsePbStructToMap
type InMapper interface {
	// goverter:useZeroValueOnPointerInconsistency
	ToAuthorization(*authv1.Authorization) (*dto.Authorization, error)
	// goverter:useZeroValueOnPointerInconsistency
	ToUserAgent(in *authv1.UserAgent) *dto.UserAgent
	// goverter:useZeroValueOnPointerInconsistency
	ToDevice(in *authv1.Device) *dto.Device
	// goverter:useZeroValueOnPointerInconsistency
	ToAuthContact(in *authv1.AuthContact) (*dto.AuthContact, error)
	// goverter:useZeroValueOnPointerInconsistency
	ToAccessToken(in *authv1.AccessToken) (*dto.AccessToken, error)
}

// goverter:converter
// goverter:matchIgnoreCase
// goverter:output:file ./generated/out.go
type OutMapper interface {
	// ToTokenRequest converts a TokenRequest from the gRPC layer to the internal DTO representation.
	// GrantType is not converted directly, define your method to convert it or use mapper.ToTokenRequest.
	// goverter:ignore GrantType state sizeCache unknownFields
	ToTokenRequest(in *dto.TokenRequest) *authv1.TokenRequest
}

func SetTokenGrant(in *authv1.TokenRequest, typer dto.GrantTyper) error {
	if in == nil {
		return nil
	}

	switch grant := typer.(type) {
	case *dto.IdentityGrant:
		meta, err := structpb.NewStruct(grant.Metadata)
		if err != nil {
			return err
		}
		in.GrantType = &authv1.TokenRequest_Identity{
			Identity: &authv1.Identity{
				Iss:                 grant.Iss,
				Sub:                 grant.Sub,
				Name:                grant.Name,
				GivenName:           grant.GivenName,
				MiddleName:          grant.MiddleName,
				FamilyName:          grant.FamilyName,
				Birthdate:           grant.Birthdate,
				Zoneinfo:            grant.Zoneinfo,
				Profile:             grant.Profile,
				Picture:             grant.Picture,
				Gender:              grant.Gender,
				Locale:              grant.Locale,
				Email:               grant.Email,
				EmailVerified:       grant.EmailVerified,
				PhoneNumber:         grant.PhoneNumber,
				PhoneNumberVerified: grant.PhoneNumberVerified,
				Metadata:            meta,
			},
		}
	case *dto.Code:
		in.GrantType = &authv1.TokenRequest_Code{
			Code: grant.Code,
		}
	case *dto.RefreshToken:
		in.GrantType = &authv1.TokenRequest_RefreshToken{
			RefreshToken: grant.RefreshToken,
		}
	default:
		return nil
	}
	return nil
}

func ParsePbStructToMap(in *structpb.Struct) map[string]any {
	if in == nil {
		return nil
	}
	return in.AsMap()
}
