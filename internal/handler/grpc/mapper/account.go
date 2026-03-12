package mapper

import (
	"google.golang.org/protobuf/types/known/structpb"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// AccountToPbMapper converts internal DTOs to gRPC Protobuf messages.
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
	// goverter:ignore state sizeCache unknownFields
	ToRegisterDeviceResponse(in *dto.RegisterDeviceResponse) *impb.RegisterDeviceResponse
	// goverter:ignore state sizeCache unknownFields
	ToUnregisterDeviceResponse(in *dto.UnregisterDeviceResponse) *impb.UnregisterDeviceResponse
}

// AccountToDtoMapper converts gRPC Protobuf messages to internal DTOs.
// goverter:converter
// goverter:matchIgnoreCase
// goverter:output:file ./generated/account_to_dto.go
// goverter:extend ParsePUSHSubscription
type AccountToDtoMapper interface {
	// ToTokenRequest converts a TokenRequest from the gRPC layer to the internal DTO representation.
	// We ignore GrantType (handled via ParseGrantType) and Headers (handled via gRPC metadata).
	// goverter:ignore GrantType Headers
	ToTokenRequest(in *impb.TokenRequest) *dto.TokenRequest

	// ToRegisterDeviceRequest converts gRPC RegisterDeviceRequest to DTO.
	ToRegisterDeviceRequest(in *impb.RegisterDeviceRequest) *dto.RegisterDeviceRequest

	// ToUnregisterDeviceRequest converts gRPC UnregisterDeviceRequest to DTO.
	ToUnregisterDeviceRequest(in *impb.UnregisterDeviceRequest) *dto.UnregisterDeviceRequest

	// ToPUSHSubscription is handled by the ParsePUSHSubscription extension.
	ToPUSHSubscription(in *impb.PUSHSubscription) *dto.PUSHSubscription
}

// ParseGrantType extracts the specific grant type from the Protobuf TokenRequest.
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

// ParsePUSHSubscription handles the conversion of the PUSHSubscription oneof field.
func ParsePUSHSubscription(in *impb.PUSHSubscription) *dto.PUSHSubscription {
	if in == nil {
		return nil
	}

	res := &dto.PUSHSubscription{
		Parameters: make(map[string]any),
	}

	// Extract provider and token from the proto oneof
	switch t := in.Token.(type) {
	case *impb.PUSHSubscription_Fcm:
		res.Provider = "fcm"
		res.Token = t.Fcm
	case *impb.PUSHSubscription_Apn:
		res.Provider = "apns"
		res.Token = t.Apn
	case *impb.PUSHSubscription_Web:
		res.Provider = "web"
		if t.Web != nil {
			res.Token = t.Web.Endpoint
			// Adding web-specific keys to parameters
			if t.Web.Key != nil {
				res.Parameters["auth"] = t.Web.Key.Auth
				res.Parameters["p256dh"] = t.Web.Key.P256Dh
			}
		}
	}

	// Add secret if present
	if len(in.Secret) > 0 {
		res.Parameters["secret"] = in.Secret
	}

	return res
}

// PbStructToAny converts a Protobuf Struct to a Go map.
func PbStructToAny(in *structpb.Struct) any {
	if in == nil {
		return nil
	}
	return in.AsMap()
}

// AnyToPbStruct converts a Go map to a Protobuf Struct.
func AnyToPbStruct(in map[string]any) (*structpb.Struct, error) {
	if in == nil {
		return nil, nil
	}
	return structpb.NewStruct(in)
}
