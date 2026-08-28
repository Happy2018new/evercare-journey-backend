package family

import "github.com/Happy2018new/evercare-journey-backend/service/general"

type FamilyMemberData struct {
	UserIdentity    string `json:"user_identity"`
	AccountName     string `json:"account_name"`
	PermissionLevel uint8  `json:"permission_level"`
	IsCreator       bool   `json:"is_creator"`
	JoinedUnixTime  int64  `json:"joined_unix_time"`
}

type FamilyPinnedTripData struct {
	TripIdentity          string `json:"trip_identity"`
	TripOwnerUserIdentity string `json:"trip_owner_user_identity"`
	PinnedByUserIdentity  string `json:"pinned_by_user_identity"`
	CreatedUnixTime       int64  `json:"created_unix_time"`
}

type FamilyTripData struct {
	TripIdentity          string `json:"trip_identity"`
	TripOwnerUserIdentity string `json:"trip_owner_user_identity"`
	TripName              string `json:"trip_name"`
	TripDate              string `json:"trip_date"`
	TravelMode            uint8  `json:"travel_mode"`
	TripStatus            uint8  `json:"trip_status"`
	CurrentVersion        uint32 `json:"current_version"`
}

type FamilyData struct {
	FamilyIdentity string                 `json:"family_identity"`
	FamilyName     string                 `json:"family_name"`
	IsAdmin        bool                   `json:"is_admin"`
	Members        []FamilyMemberData     `json:"members"`
	PinnedTrips    []FamilyPinnedTripData `json:"pinned_trips"`
	Trips          []FamilyTripData       `json:"trips"`
}

type CreateFamilyRequest struct {
	general.BasicSessionInfo
	FamilyName string `json:"family_name"`
}
type CreateFamilyResponse struct {
	general.BasicResponseInfo
	FamilyData FamilyData `json:"family_data"`
	InviteCode string     `json:"invite_code"`
}
type QueryFamilyRequest struct{ general.BasicSessionInfo }
type QueryFamilyResponse struct {
	general.BasicResponseInfo
	HasFamily  bool       `json:"has_family"`
	FamilyData FamilyData `json:"family_data"`
}
type UpdateFamilyNameRequest struct {
	general.BasicSessionInfo
	FamilyName string `json:"family_name"`
}
type UpdateFamilyNameResponse struct {
	general.BasicResponseInfo
	FamilyName string `json:"family_name"`
}
type GenerateInviteCodeRequest struct{ general.BasicSessionInfo }
type GenerateInviteCodeResponse struct {
	general.BasicResponseInfo
	InviteCode     string `json:"invite_code"`
	ExpireUnixTime int64  `json:"expire_unix_time"`
}
type JoinFamilyRequest struct {
	general.BasicSessionInfo
	InviteCode string `json:"invite_code"`
}
type JoinFamilyResponse struct {
	general.BasicResponseInfo
	FamilyData FamilyData `json:"family_data"`
}
type LeaveFamilyRequest struct{ general.BasicSessionInfo }
type LeaveFamilyResponse struct{ general.BasicResponseInfo }
type UpdateMemberPermissionRequest struct {
	general.BasicSessionInfo
	TargetUserIdentity string `json:"target_user_identity"`
	PermissionLevel    uint8  `json:"permission_level"`
}
type UpdateMemberPermissionResponse struct{ general.BasicResponseInfo }
type RemoveMemberRequest struct {
	general.BasicSessionInfo
	TargetUserIdentity string `json:"target_user_identity"`
}
type RemoveMemberResponse struct{ general.BasicResponseInfo }
type PinTripRequest struct {
	general.BasicSessionInfo
	TripIdentity string `json:"trip_identity"`
}
type PinTripResponse struct {
	general.BasicResponseInfo
	PinnedTrips []FamilyPinnedTripData `json:"pinned_trips"`
}
type UnpinTripRequest struct {
	general.BasicSessionInfo
	TripIdentity string `json:"trip_identity"`
}
type UnpinTripResponse struct {
	general.BasicResponseInfo
	PinnedTrips []FamilyPinnedTripData `json:"pinned_trips"`
}
