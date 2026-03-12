package mapper

import (
	"google.golang.org/protobuf/types/known/structpb"

	authv1 "github.com/webitel/im-gateway-service/gen/go/auth/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// InMapper handles conversion from Protobuf responses to DTOs.
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
	ToAuthContact(in *authv1.Contact) (*dto.AuthContact, error)
	// goverter:useZeroValueOnPointerInconsistency
	ToAccessToken(in *authv1.AccessToken) (*dto.AccessToken, error)
}

// goverter:converter
// goverter:matchIgnoreCase
// goverter:output:file ./generated/out.go
// goverter:extend ToPbPUSHSubscription
type OutMapper interface {
	// ToTokenRequest converts model to protobuf request.
	// goverter:ignore GrantType state sizeCache unknownFields
	ToTokenRequest(in *dto.TokenRequest) *authv1.TokenRequest

	// ToRegisterDeviceRequest converts DTO to protobuf request.
	// goverter:map Push Push
	// goverter:ignore state sizeCache unknownFields
	ToRegisterDeviceRequest(in *dto.RegisterDeviceRequest) *authv1.RegisterDeviceRequest

	// ToUnregisterDeviceRequest converts DTO to protobuf request.
	// goverter:map Push Push
	// goverter:ignore state sizeCache unknownFields
	ToUnregisterDeviceRequest(in *dto.UnregisterDeviceRequest) *authv1.UnregisterDeviceRequest
}

// SetTokenGrant handles the complex mapping of oneof GrantType in TokenRequest.
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
	}
	return nil
}

// ToPbPUSHSubscription converts the DTO PUSHSubscription back to the oneof Protobuf format.
func ToPbPUSHSubscription(in *dto.PUSHSubscription) *authv1.PUSHSubscription {
	if in == nil {
		return nil
	}

	res := &authv1.PUSHSubscription{}

	// Map secret back to bytes if present in parameters
	if val, ok := in.Parameters["secret"]; ok {
		if secret, ok := val.([]byte); ok {
			res.Secret = secret
		}
	}

	// Restore the specific token type based on the Provider string
	switch in.Provider {
	case "fcm":
		res.Token = &authv1.PUSHSubscription_Fcm{Fcm: in.Token}
	case "apns":
		res.Token = &authv1.PUSHSubscription_Apn{Apn: in.Token}
	case "web":
		webSub := &authv1.WebPushSubscription{
			Endpoint: in.Token,
		}
		// If we stored keys in parameters, map them back
		if auth, ok := in.Parameters["auth"].([]byte); ok {
			if webSub.Key == nil {
				webSub.Key = &authv1.WebPushSubscription_Key{}
			}
			webSub.Key.Auth = auth
		}
		if p256dh, ok := in.Parameters["p256dh"].([]byte); ok {
			if webSub.Key == nil {
				webSub.Key = &authv1.WebPushSubscription_Key{}
			}
			webSub.Key.P256Dh = p256dh
		}
		res.Token = &authv1.PUSHSubscription_Web{Web: webSub}
	}

	return res
}

// ParsePbStructToMap converts a Protobuf Struct to a Go map.
func ParsePbStructToMap(in *structpb.Struct) map[string]any {
	if in == nil {
		return nil
	}
	return in.AsMap()
}
