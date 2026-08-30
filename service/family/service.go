package family

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
)

const (
	InviteCodeTTL             = 5 * time.Minute
	InviteCodeCleanupInterval = time.Minute
	MaxFamilyMembers          = 100
	MaxFamilyNameLength       = 32
)

type inviteEntry struct {
	FamilyIdentity string
	FamilyUniqueID uint64
}

var inviteCodes = cache.New(InviteCodeTTL, InviteCodeCleanupInterval)

const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func makeInviteCode() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := make([]byte, len(raw))
	for i, value := range raw {
		code[i] = inviteAlphabet[int(value)%len(inviteAlphabet)]
	}
	return string(code), nil
}

func loadUser(session general.BasicSessionInfo, source string) (*define.UserData, *define.GeneralError) {
	status, ge := auth.ValidateSession(session)
	if ge != nil {
		return nil, ge.AppendSource(source)
	}
	if status != auth.ValidateSessionStatusValidSession {
		return nil, define.NewGeneralError(source, fmt.Errorf("invalid session status %d", status), define.LangKeyGeneralInvalidSession)
	}
	user, found, ge := auth.LoadUser(session.UserIdentity, false)
	if ge != nil {
		return nil, ge.AppendSource(source)
	}
	if !found || user == nil {
		return nil, define.NewGeneralError(source, fmt.Errorf("user not found"), define.LangKeyGeneralInvalidSession)
	}
	return user, nil
}

func respond(c *gin.Context, response any, ge *define.GeneralError) {
	if ge == nil {
		c.JSON(http.StatusOK, response)
		return
	}
	info := general.FromGeneralError(ge)
	switch value := response.(type) {
	case CreateFamilyResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case QueryFamilyResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case UpdateFamilyNameResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case GenerateInviteCodeResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case JoinFamilyResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case LeaveFamilyResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case UpdateMemberPermissionResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case RemoveMemberResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case PinTripResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case UnpinTripResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	default:
		c.JSON(http.StatusOK, gin.H{"success_states": false, "debug_error_info": info.DebugErrorInfo, "public_error_msg": info.PublicErrorMsg})
	}
}

func normalizeFamilyName(source, name string) (string, *define.GeneralError) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "我的家庭"
	}
	if len([]rune(name)) > MaxFamilyNameLength {
		return "", define.NewGeneralError(source, fmt.Errorf("family name too long"), define.LangKeyFamilyNameInvalid)
	}
	return name, nil
}

func loadFamily(tx *gorm.DB, userID uint32, source string) (define.FamilyInfo, define.FamilyMember, *define.GeneralError) {
	family, member, found, ge := environment.DB.FamilyHandle().QueryFamilyByUser(tx, userID)
	if ge != nil {
		return family, member, ge.AppendSource(source)
	}
	if !found {
		return family, member, define.NewGeneralError(source, fmt.Errorf("user does not belong to a family"), define.LangKeyFamilyNotFound)
	}
	return family, member, nil
}

func requireAdmin(tx *gorm.DB, userID uint32, source string) (define.FamilyInfo, *define.GeneralError) {
	family, member, ge := loadFamily(tx, userID, source)
	if ge != nil {
		return family, ge
	}
	if member.PermissionLevel != define.FamilyMemberPermissionAdmin {
		return family, define.NewGeneralError(source, fmt.Errorf("user is not an administrator"), define.LangKeyFamilyMemberPermissionDenied)
	}
	return family, nil
}

func makeFamilyData(tx *gorm.DB, family define.FamilyInfo, currentUserID uint32) (FamilyData, *define.GeneralError) {
	members, ge := environment.DB.FamilyHandle().QueryMembers(tx, family.FamilyUniqueID)
	if ge != nil {
		return FamilyData{}, ge
	}
	pins, ge := environment.DB.FamilyHandle().QueryPinnedTrips(tx, family.FamilyUniqueID)
	if ge != nil {
		return FamilyData{}, ge
	}
	trips, ge := environment.DB.FamilyHandle().QueryFamilyTrips(tx, family.FamilyUniqueID)
	if ge != nil {
		return FamilyData{}, ge
	}
	data := FamilyData{
		FamilyIdentity: family.FamilyIdentity,
		FamilyName:     family.FamilyName,
		Members:        make([]FamilyMemberData, 0, len(members)),
		PinnedTrips:    make([]FamilyPinnedTripData, 0, len(pins)),
		Trips:          make([]FamilyTripData, 0, len(trips)),
	}
	ownerIdentities := make(map[uint32]string, len(members))
	for _, member := range members {
		user, found, userErr := environment.DB.UserHandle().QueryUser(tx, handle.QueryUserActionSearchByUniqueID, member.UserUniqueID)
		if userErr != nil {
			return FamilyData{}, userErr
		}
		if !found {
			continue
		}
		data.Members = append(data.Members, FamilyMemberData{
			UserIdentity:    user.UserIdentity,
			AccountName:     user.AccountName,
			PermissionLevel: member.PermissionLevel,
			IsCreator:       member.UserUniqueID == family.OwnerUserUniqueID,
			JoinedUnixTime:  member.JoinedUnixTime,
		})
		ownerIdentities[member.UserUniqueID] = user.UserIdentity
		if member.UserUniqueID == currentUserID {
			data.IsAdmin = member.PermissionLevel == define.FamilyMemberPermissionAdmin
		}
	}
	for _, trip := range trips {
		data.Trips = append(data.Trips, FamilyTripData{
			TripIdentity:          trip.TripIdentity,
			TripOwnerUserIdentity: ownerIdentities[trip.UserUniqueID],
			TripName:              trip.TripName,
			TripDate:              trip.TripDate.Format("2006-01-02"),
			TravelMode:            trip.TravelMode,
			TripStatus:            trip.TripStatus,
			CurrentVersion:        trip.CurrentVersion,
		})
	}
	for _, pin := range pins {
		data.PinnedTrips = append(data.PinnedTrips, FamilyPinnedTripData{TripIdentity: pin.TripIdentity, CreatedUnixTime: pin.CreatedUnixTime})
	}
	return data, nil
}

func issueInviteCode(family define.FamilyInfo) (string, *define.GeneralError) {
	for i := 0; i < 8; i++ {
		code, err := makeInviteCode()
		if err != nil {
			return "", define.NewGeneralError("IssueInviteCode", err, define.LangKeyFamilyInviteCodeCreateUnknown)
		}
		if _, found := inviteCodes.Get(code); !found {
			inviteCodes.Set(code, inviteEntry{FamilyIdentity: family.FamilyIdentity, FamilyUniqueID: family.FamilyUniqueID}, InviteCodeTTL)
			return code, nil
		}
	}
	return "", define.NewGeneralError("IssueInviteCode", fmt.Errorf("invite code collision"), define.LangKeyFamilyInviteCodeCreateUnknown)
}

func HandleCreate(c *gin.Context) {
	const source = "HandleCreateFamily"
	var request CreateFamilyRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, CreateFamilyResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, CreateFamilyResponse{}, ge)
		return
	}
	name, ge := normalizeFamilyName(source, request.FamilyName)
	if ge != nil {
		respond(c, CreateFamilyResponse{}, ge)
		return
	}
	family, ge := environment.DB.FamilyHandle().CreateFamily(environment.DB.Database(), user.UserUniqueID, name)
	if ge != nil {
		respond(c, CreateFamilyResponse{}, ge)
		return
	}
	code, ge := issueInviteCode(family)
	if ge != nil {
		respond(c, CreateFamilyResponse{}, ge)
		return
	}
	data, ge := makeFamilyData(environment.DB.Database(), family, user.UserUniqueID)
	if ge != nil {
		respond(c, CreateFamilyResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, CreateFamilyResponse{BasicResponseInfo: general.SuccResponseInfo(), FamilyData: data, InviteCode: code})
}

func HandleQuery(c *gin.Context) {
	const source = "HandleQueryFamily"
	var request QueryFamilyRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, QueryFamilyResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, QueryFamilyResponse{}, ge)
		return
	}
	family, _, found, ge := environment.DB.FamilyHandle().QueryFamilyByUser(environment.DB.Database(), user.UserUniqueID)
	if ge != nil {
		respond(c, QueryFamilyResponse{}, ge)
		return
	}
	if !found {
		c.JSON(http.StatusOK, QueryFamilyResponse{BasicResponseInfo: general.SuccResponseInfo(), HasFamily: false})
		return
	}
	data, ge := makeFamilyData(environment.DB.Database(), family, user.UserUniqueID)
	if ge != nil {
		respond(c, QueryFamilyResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, QueryFamilyResponse{BasicResponseInfo: general.SuccResponseInfo(), HasFamily: true, FamilyData: data})
}

func HandleUpdateName(c *gin.Context) {
	const source = "HandleUpdateFamilyName"
	var request UpdateFamilyNameRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, UpdateFamilyNameResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, UpdateFamilyNameResponse{}, ge)
		return
	}
	name := strings.TrimSpace(request.FamilyName)
	if name == "" {
		respond(c, UpdateFamilyNameResponse{}, define.NewGeneralError(source, fmt.Errorf("family name is empty"), define.LangKeyFamilyNameInvalid))
		return
	}
	name, ge = normalizeFamilyName(source, name)
	if ge != nil {
		respond(c, UpdateFamilyNameResponse{}, ge)
		return
	}
	family, ge := requireAdmin(environment.DB.Database(), user.UserUniqueID, source)
	if ge != nil {
		respond(c, UpdateFamilyNameResponse{}, ge)
		return
	}
	if name != family.FamilyName {
		if ge = environment.DB.FamilyHandle().UpdateFamilyName(environment.DB.Database(), family.FamilyUniqueID, name); ge != nil {
			respond(c, UpdateFamilyNameResponse{}, ge)
			return
		}
	}
	c.JSON(http.StatusOK, UpdateFamilyNameResponse{BasicResponseInfo: general.SuccResponseInfo(), FamilyName: name})
}

func HandleGenerateCode(c *gin.Context) {
	const source = "HandleGenerateFamilyInviteCode"
	var request GenerateInviteCodeRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, GenerateInviteCodeResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, GenerateInviteCodeResponse{}, ge)
		return
	}
	family, ge := requireAdmin(environment.DB.Database(), user.UserUniqueID, source)
	if ge != nil {
		respond(c, GenerateInviteCodeResponse{}, ge)
		return
	}
	code, ge := issueInviteCode(family)
	if ge != nil {
		respond(c, GenerateInviteCodeResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, GenerateInviteCodeResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		InviteCode:        code,
		ExpireUnixTime:    time.Now().Unix() + int64(InviteCodeTTL/time.Second),
	})
}

func HandleJoin(c *gin.Context) {
	const source = "HandleJoinFamily"
	var request JoinFamilyRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, JoinFamilyResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, JoinFamilyResponse{}, ge)
		return
	}
	value, found := inviteCodes.Get(strings.ToUpper(strings.TrimSpace(request.InviteCode)))
	if !found {
		respond(c, JoinFamilyResponse{}, define.NewGeneralError(source, fmt.Errorf("invite code expired"), define.LangKeyFamilyInviteCodeExpired))
		return
	}
	entry, ok := value.(inviteEntry)
	if !ok {
		respond(c, JoinFamilyResponse{}, define.NewGeneralError(source, fmt.Errorf("invalid invite entry"), define.LangKeyFamilyInviteCodeInvalid))
		return
	}
	if _, _, already, queryErr := environment.DB.FamilyHandle().QueryFamilyByUser(environment.DB.Database(), user.UserUniqueID); queryErr != nil {
		respond(c, JoinFamilyResponse{}, queryErr)
		return
	} else if already {
		respond(c, JoinFamilyResponse{}, define.NewGeneralError(source, fmt.Errorf("user already belongs to a family"), define.LangKeyFamilyAlreadyJoined))
		return
	}
	family, familyFound, ge := environment.DB.FamilyHandle().QueryFamily(environment.DB.Database(), handle.QueryFamilyActionByUniqueID, entry.FamilyUniqueID)
	if ge != nil {
		respond(c, JoinFamilyResponse{}, ge)
		return
	}
	if !familyFound {
		respond(c, JoinFamilyResponse{}, define.NewGeneralError(source, fmt.Errorf("family not found"), define.LangKeyFamilyNotFound))
		return
	}
	members, ge := environment.DB.FamilyHandle().QueryMembers(environment.DB.Database(), family.FamilyUniqueID)
	if ge != nil {
		respond(c, JoinFamilyResponse{}, ge)
		return
	}
	if len(members) >= MaxFamilyMembers {
		respond(c, JoinFamilyResponse{}, define.NewGeneralError(source, fmt.Errorf("family member limit reached"), define.LangKeyFamilyMemberUpdateUnknown))
		return
	}
	if ge = environment.DB.FamilyHandle().AddMember(environment.DB.Database(), family.FamilyUniqueID, user.UserUniqueID); ge != nil {
		respond(c, JoinFamilyResponse{}, ge)
		return
	}
	data, ge := makeFamilyData(environment.DB.Database(), family, user.UserUniqueID)
	if ge != nil {
		respond(c, JoinFamilyResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, JoinFamilyResponse{BasicResponseInfo: general.SuccResponseInfo(), FamilyData: data})
}

func HandleLeave(c *gin.Context) {
	const source = "HandleLeaveFamily"
	var request LeaveFamilyRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, LeaveFamilyResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, LeaveFamilyResponse{}, ge)
		return
	}
	family, member, ge := loadFamily(environment.DB.Database(), user.UserUniqueID, source)
	if ge != nil {
		respond(c, LeaveFamilyResponse{}, ge)
		return
	}
	if ge = environment.DB.FamilyHandle().LeaveFamily(environment.DB.Database(), family, member); ge != nil {
		respond(c, LeaveFamilyResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, LeaveFamilyResponse{BasicResponseInfo: general.SuccResponseInfo()})
}

func HandleUpdateMemberPermission(c *gin.Context) {
	const source = "HandleUpdateFamilyMemberPermission"
	var request UpdateMemberPermissionRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, UpdateMemberPermissionResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, UpdateMemberPermissionResponse{}, ge)
		return
	}
	if request.PermissionLevel > define.FamilyMemberPermissionAdmin {
		respond(c, UpdateMemberPermissionResponse{}, define.NewGeneralError(source, fmt.Errorf("invalid permission"), define.LangKeyFamilyMemberPermissionInvalid))
		return
	}
	family, ge := requireAdmin(environment.DB.Database(), user.UserUniqueID, source)
	if ge != nil {
		respond(c, UpdateMemberPermissionResponse{}, ge)
		return
	}
	targetIdentity := strings.TrimSpace(request.TargetUserIdentity)
	if _, err := uuid.Parse(targetIdentity); err != nil {
		respond(c, UpdateMemberPermissionResponse{}, define.NewGeneralError(source, fmt.Errorf("invalid target user identity"), define.LangKeyFamilyMemberNotFound))
		return
	}
	target, found, ge := environment.DB.UserHandle().QueryUser(environment.DB.Database(), handle.QueryUserActionSearchByUserIdentity, targetIdentity)
	if ge != nil {
		respond(c, UpdateMemberPermissionResponse{}, ge)
		return
	}
	if !found {
		respond(c, UpdateMemberPermissionResponse{}, define.NewGeneralError(source, fmt.Errorf("member not found"), define.LangKeyFamilyMemberNotFound))
		return
	}
	if target.UserUniqueID == user.UserUniqueID {
		respond(c, UpdateMemberPermissionResponse{}, define.NewGeneralError(source, fmt.Errorf("cannot update own family permission"), define.LangKeyFamilyMemberPermissionSelf))
		return
	}
	if target.UserUniqueID == family.OwnerUserUniqueID && request.PermissionLevel != define.FamilyMemberPermissionAdmin {
		respond(c, UpdateMemberPermissionResponse{}, define.NewGeneralError(source, fmt.Errorf("owner cannot be demoted"), define.LangKeyFamilyMemberPermissionDenied))
		return
	}
	_, memberFound, ge := environment.DB.FamilyHandle().QueryMember(environment.DB.Database(), family.FamilyUniqueID, target.UserUniqueID)
	if ge != nil {
		respond(c, UpdateMemberPermissionResponse{}, ge)
		return
	}
	if !memberFound {
		respond(c, UpdateMemberPermissionResponse{}, define.NewGeneralError(source, fmt.Errorf("member not found"), define.LangKeyFamilyMemberNotFound))
		return
	}
	if ge = environment.DB.FamilyHandle().UpdateMemberPermission(environment.DB.Database(), family.FamilyUniqueID, target.UserUniqueID, request.PermissionLevel); ge != nil {
		respond(c, UpdateMemberPermissionResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, UpdateMemberPermissionResponse{BasicResponseInfo: general.SuccResponseInfo()})
}

func HandleRemoveMember(c *gin.Context) {
	const source = "HandleRemoveFamilyMember"
	var request RemoveMemberRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, RemoveMemberResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, RemoveMemberResponse{}, ge)
		return
	}
	family, ge := requireAdmin(environment.DB.Database(), user.UserUniqueID, source)
	if ge != nil {
		respond(c, RemoveMemberResponse{}, ge)
		return
	}
	targetIdentity := strings.TrimSpace(request.TargetUserIdentity)
	if _, err := uuid.Parse(targetIdentity); err != nil {
		respond(c, RemoveMemberResponse{}, define.NewGeneralError(source, fmt.Errorf("invalid target user identity"), define.LangKeyFamilyMemberNotFound))
		return
	}
	target, found, ge := environment.DB.UserHandle().QueryUser(environment.DB.Database(), handle.QueryUserActionSearchByUserIdentity, targetIdentity)
	if ge != nil {
		respond(c, RemoveMemberResponse{}, ge)
		return
	}
	if !found {
		respond(c, RemoveMemberResponse{}, define.NewGeneralError(source, fmt.Errorf("member not found"), define.LangKeyFamilyMemberNotFound))
		return
	}
	if target.UserUniqueID == user.UserUniqueID || target.UserUniqueID == family.OwnerUserUniqueID {
		respond(c, RemoveMemberResponse{}, define.NewGeneralError(source, fmt.Errorf("cannot remove owner or self"), define.LangKeyFamilyMemberRemoveSelfInvalid))
		return
	}
	if ge = environment.DB.FamilyHandle().RemoveMember(environment.DB.Database(), family.FamilyUniqueID, target.UserUniqueID); ge != nil {
		respond(c, RemoveMemberResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, RemoveMemberResponse{BasicResponseInfo: general.SuccResponseInfo()})
}

func HandlePinTrip(c *gin.Context) {
	const source = "HandlePinFamilyTrip"
	var request PinTripRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, PinTripResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, PinTripResponse{}, ge)
		return
	}
	family, ge := requireAdmin(environment.DB.Database(), user.UserUniqueID, source)
	if ge != nil {
		respond(c, PinTripResponse{}, ge)
		return
	}
	identity, err := uuid.Parse(strings.TrimSpace(request.TripIdentity))
	if err != nil || identity == uuid.Nil {
		respond(c, PinTripResponse{}, define.NewGeneralError(source, fmt.Errorf("invalid trip identity"), define.LangKeyFamilyPinnedTripInvalid))
		return
	}
	trip, found, ge := environment.DB.TripHandle().QueryTripByIdentity(environment.DB.Database(), identity.String())
	if ge != nil {
		respond(c, PinTripResponse{}, ge)
		return
	}
	if !found {
		respond(c, PinTripResponse{}, define.NewGeneralError(source, fmt.Errorf("trip not found"), define.LangKeyFamilyPinnedTripInvalid))
		return
	}
	allowed, ge := environment.DB.FamilyHandle().CanReadTrip(environment.DB.Database(), user.UserUniqueID, trip.UserUniqueID)
	if ge != nil || !allowed {
		if ge == nil {
			ge = define.NewGeneralError(source, fmt.Errorf("trip owner is outside family"), define.LangKeyFamilyPinnedTripInvalid)
		}
		respond(c, PinTripResponse{}, ge)
		return
	}
	if ge = environment.DB.FamilyHandle().PinTrip(environment.DB.Database(), family.FamilyUniqueID, trip.TripIdentity, trip.UserUniqueID, user.UserUniqueID); ge != nil {
		respond(c, PinTripResponse{}, ge)
		return
	}
	data, ge := makeFamilyData(environment.DB.Database(), family, user.UserUniqueID)
	if ge != nil {
		respond(c, PinTripResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, PinTripResponse{BasicResponseInfo: general.SuccResponseInfo(), PinnedTrips: data.PinnedTrips})
}

func HandleUnpinTrip(c *gin.Context) {
	const source = "HandleUnpinFamilyTrip"
	var request UnpinTripRequest
	if err := c.ShouldBind(&request); err != nil {
		respond(c, UnpinTripResponse{}, define.NewGeneralError(source, err, define.LangKeyFamilyRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		respond(c, UnpinTripResponse{}, ge)
		return
	}
	family, ge := requireAdmin(environment.DB.Database(), user.UserUniqueID, source)
	if ge != nil {
		respond(c, UnpinTripResponse{}, ge)
		return
	}
	identity, err := uuid.Parse(strings.TrimSpace(request.TripIdentity))
	if err != nil || identity == uuid.Nil {
		respond(c, UnpinTripResponse{}, define.NewGeneralError(source, fmt.Errorf("invalid trip identity"), define.LangKeyFamilyPinnedTripInvalid))
		return
	}
	if ge = environment.DB.FamilyHandle().UnpinTrip(environment.DB.Database(), family.FamilyUniqueID, identity.String()); ge != nil {
		respond(c, UnpinTripResponse{}, ge)
		return
	}
	pins, ge := environment.DB.FamilyHandle().QueryPinnedTrips(environment.DB.Database(), family.FamilyUniqueID)
	if ge != nil {
		respond(c, UnpinTripResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, UnpinTripResponse{BasicResponseInfo: general.SuccResponseInfo(), PinnedTrips: makePinnedData(pins)})
}

func makePinnedData(pins []define.FamilyPinnedTrip) []FamilyPinnedTripData {
	result := make([]FamilyPinnedTripData, 0, len(pins))
	for _, pin := range pins {
		result = append(result, FamilyPinnedTripData{TripIdentity: pin.TripIdentity, CreatedUnixTime: pin.CreatedUnixTime})
	}
	return result
}
