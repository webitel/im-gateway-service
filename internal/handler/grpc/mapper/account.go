package mapper

import (
	"google.golang.org/protobuf/types/known/structpb"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// goverter:converter
// goverter:matchIgnoreCase
// goverter:output:file ./generated/account_to_pb.go
// goverter:extend AnyToPbStruct
type AccountToPbMapper interface {
	// goverter:ignore state sizeCache unknownFields
	ToUserAgent(in *dto.UserAgent) *impb.UserAgent
	// goverter:ignore state sizeCache unknownFields Push
	ToDevice(in *dto.Device) *impb.Device
	// goverter:ignore state sizeCache unknownFields
	ToAuthContact(in *dto.AuthContact) (*impb.AuthContact, error)
	// goverter:ignore state sizeCache unknownFields
	ToAccessToken(in *dto.AccessToken) (*impb.AccessToken, error)
	// goverter:ignore state sizeCache unknownFields
	ToAuthorization(in *dto.Authorization) (*impb.Authorization, error)
}

// goverter:converter
// goverter:matchIgnoreCase
// goverter:output:file ./generated/account_to_dto.go
type AccountToDtoMapper interface {
	// ToTokenRequest converts a TokenRequest from the gRPC layer to the internal DTO representation.
	// GrantType is not converted directly, define your method to convert it or use mapper.ToTokenRequest.
	// goverter:ignore GrantType
	ToTokenRequest(in *impb.TokenRequest) *dto.TokenRequest
}

func ParseGrantType(in *impb.TokenRequest) dto.GrantTyper {
	if in == nil {
		return nil
	}
	if i := in.GetIdentity(); i != nil {
		return &dto.IdentityGrant{
			Iss:                 i.GetIss(),
			Sub:                 i.GetSub(),
			Name:                i.GetName(),
			GivenName:           i.GetGivenName(),
			MiddleName:          i.GetMiddleName(),
			FamilyName:          i.GetFamilyName(),
			Birthdate:           i.GetBirthdate(),
			Zoneinfo:            i.GetZoneinfo(),
			Profile:             i.GetProfile(),
			Picture:             i.GetPicture(),
			Gender:              i.GetGender(),
			Locale:              i.GetLocale(),
			Email:               i.GetEmail(),
			EmailVerified:       i.GetEmailVerified(),
			PhoneNumber:         i.GetPhoneNumber(),
			PhoneNumberVerified: i.GetPhoneNumberVerified(),
		}
	}

	if i := in.GetCode(); i != "" {
		return &dto.Code{
			Code: i,
		}
	}

	if i := in.GetRefreshToken(); i != "" {
		return &dto.RefreshToken{
			RefreshToken: i,
		}
	}

	return nil
}

func PbStructToAny(in *structpb.Struct) any {
	if in == nil {
		return nil
	}
	return in.AsMap()
}

func AnyToPbStruct(in map[string]any) (*structpb.Struct, error) {
	if in == nil {
		return nil, nil
	}
	return structpb.NewStruct(in)
}
