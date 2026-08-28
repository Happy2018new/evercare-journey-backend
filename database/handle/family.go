package handle

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	QueryFamilyActionByUniqueID uint8 = iota
	QueryFamilyActionByIdentity
)

type FamilyHandle struct{}

func NewFamilyHandle() *FamilyHandle { return new(FamilyHandle) }

func (h *FamilyHandle) CreateFamily(tx *gorm.DB, ownerUserUniqueID uint32, familyName string) (define.FamilyInfo, *define.GeneralError) {
	family := define.FamilyInfo{FamilyIdentity: uuid.NewString(), FamilyName: strings.TrimSpace(familyName), OwnerUserUniqueID: ownerUserUniqueID, CreatedUnixTime: time.Now().Unix(), UpdateUnixTime: time.Now().Unix()}
	err := tx.Transaction(func(tx *gorm.DB) error {
		if result := tx.Where("user_unique_id = ?", ownerUserUniqueID).First(&define.FamilyMember{}); result.Error == nil {
			return define.NewGeneralError("CreateFamily", fmt.Errorf("user already belongs to a family"), define.LangKeyFamilyAlreadyJoined)
		} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		if result := tx.Create(&family); result.Error != nil {
			return result.Error
		}
		member := define.FamilyMember{FamilyUniqueID: family.FamilyUniqueID, UserUniqueID: ownerUserUniqueID, PermissionLevel: define.FamilyMemberPermissionAdmin, JoinedUnixTime: time.Now().Unix()}
		if result := tx.Create(&member); result.Error != nil {
			return result.Error
		}
		return nil
	})
	if err == nil {
		return family, nil
	}
	if ge, ok := err.(*define.GeneralError); ok {
		return family, ge.AppendSource("CreateFamily")
	}
	return family, define.NewGeneralError("CreateFamily", err, define.LangKeyFamilyCreateUnknown)
}

func (h *FamilyHandle) QueryFamily(tx *gorm.DB, action uint8, keyword any) (define.FamilyInfo, bool, *define.GeneralError) {
	query := tx
	switch action {
	case QueryFamilyActionByUniqueID:
		query = query.Where("family_unique_id = ?", keyword)
	case QueryFamilyActionByIdentity:
		query = query.Where("family_identity = ?", keyword)
	default:
		return define.FamilyInfo{}, false, define.NewGeneralError("QueryFamily", fmt.Errorf("unsupported action %d", action), define.LangKeyFamilyQueryUnknown)
	}
	var family define.FamilyInfo
	result := query.First(&family)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return family, false, nil
	}
	if result.Error != nil {
		return family, false, define.NewGeneralError("QueryFamily", result.Error, define.LangKeyFamilyQueryUnknown)
	}
	return family, true, nil
}

func (h *FamilyHandle) QueryFamilyByUser(tx *gorm.DB, userUniqueID uint32) (define.FamilyInfo, define.FamilyMember, bool, *define.GeneralError) {
	var member define.FamilyMember
	result := tx.Where("user_unique_id = ?", userUniqueID).First(&member)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return define.FamilyInfo{}, member, false, nil
	}
	if result.Error != nil {
		return define.FamilyInfo{}, member, false, define.NewGeneralError("QueryFamilyByUser", result.Error, define.LangKeyFamilyQueryUnknown)
	}
	family, found, ge := h.QueryFamily(tx, QueryFamilyActionByUniqueID, member.FamilyUniqueID)
	return family, member, found, ge
}

func (h *FamilyHandle) UpdateFamilyName(tx *gorm.DB, familyUniqueID uint64, familyName string) *define.GeneralError {
	result := tx.Model(&define.FamilyInfo{}).
		Where("family_unique_id = ?", familyUniqueID).
		Updates(map[string]any{
			"family_name":      strings.TrimSpace(familyName),
			"update_unix_time": time.Now().Unix(),
		})
	if result.Error != nil {
		return define.NewGeneralError("UpdateFamilyName", result.Error, define.LangKeyFamilyUpdateUnknown)
	}
	if result.RowsAffected != 1 {
		return define.NewGeneralError("UpdateFamilyName", fmt.Errorf("family not found"), define.LangKeyFamilyNotFound)
	}
	return nil
}

func (h *FamilyHandle) QueryMembers(tx *gorm.DB, familyUniqueID uint64) ([]define.FamilyMember, *define.GeneralError) {
	var members []define.FamilyMember
	result := tx.Where("family_unique_id = ?", familyUniqueID).Order("joined_unix_time ASC, family_member_unique_id ASC").Find(&members)
	if result.Error != nil {
		return nil, define.NewGeneralError("QueryFamilyMembers", result.Error, define.LangKeyFamilyMemberQueryUnknown)
	}
	return members, nil
}

func (h *FamilyHandle) QueryMember(tx *gorm.DB, familyUniqueID uint64, userUniqueID uint32) (define.FamilyMember, bool, *define.GeneralError) {
	var member define.FamilyMember
	result := tx.Where("family_unique_id = ? AND user_unique_id = ?", familyUniqueID, userUniqueID).First(&member)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return member, false, nil
	}
	if result.Error != nil {
		return member, false, define.NewGeneralError("QueryFamilyMember", result.Error, define.LangKeyFamilyMemberQueryUnknown)
	}
	return member, true, nil
}

func (h *FamilyHandle) UpdateMemberPermission(tx *gorm.DB, familyUniqueID uint64, userUniqueID uint32, permission uint8) *define.GeneralError {
	result := tx.Model(&define.FamilyMember{}).Where("family_unique_id = ? AND user_unique_id = ?", familyUniqueID, userUniqueID).Update("permission_level", permission)
	if result.Error != nil {
		return define.NewGeneralError("UpdateFamilyMember", result.Error, define.LangKeyFamilyMemberUpdateUnknown)
	}
	if result.RowsAffected != 1 {
		return define.NewGeneralError("UpdateFamilyMember", fmt.Errorf("member not found"), define.LangKeyFamilyMemberNotFound)
	}
	return nil
}

func (h *FamilyHandle) RemoveMember(tx *gorm.DB, familyUniqueID uint64, userUniqueID uint32) *define.GeneralError {
	err := tx.Transaction(func(tx *gorm.DB) error {
		if result := tx.Where("family_unique_id = ? AND trip_owner_user_unique_id = ?", familyUniqueID, userUniqueID).Delete(&define.FamilyPinnedTrip{}); result.Error != nil {
			return result.Error
		}
		result := tx.Where("family_unique_id = ? AND user_unique_id = ?", familyUniqueID, userUniqueID).Delete(&define.FamilyMember{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return define.NewGeneralError("RemoveFamilyMember", fmt.Errorf("member not found"), define.LangKeyFamilyMemberNotFound)
		}
		return define.NewGeneralError("RemoveFamilyMember", err, define.LangKeyFamilyMemberUpdateUnknown)
	}
	return nil
}

func (h *FamilyHandle) LeaveFamily(tx *gorm.DB, family define.FamilyInfo, member define.FamilyMember) *define.GeneralError {
	err := tx.Transaction(func(tx *gorm.DB) error {
		if result := tx.Where("family_unique_id = ? AND trip_owner_user_unique_id = ?", family.FamilyUniqueID, member.UserUniqueID).Delete(&define.FamilyPinnedTrip{}); result.Error != nil { return result.Error }
		if result := tx.Where("family_unique_id = ? AND user_unique_id = ?", family.FamilyUniqueID, member.UserUniqueID).Delete(&define.FamilyMember{}); result.Error != nil {
			return result.Error
		}
		if member.UserUniqueID != family.OwnerUserUniqueID {
			return nil
		}
		var next define.FamilyMember
		result := tx.Where("family_unique_id = ?", family.FamilyUniqueID).Order("joined_unix_time ASC, family_member_unique_id ASC").First(&next)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			if result := tx.Where("family_unique_id = ?", family.FamilyUniqueID).Delete(&define.FamilyPinnedTrip{}); result.Error != nil {
				return result.Error
			}
			if result := tx.Delete(&family); result.Error != nil {
				return result.Error
			}
			return nil
		}
		if result.Error != nil {
			return result.Error
		}
		if result := tx.Model(&family).Update("owner_user_unique_id", next.UserUniqueID); result.Error != nil {
			return result.Error
		}
		if result := tx.Model(&next).Update("permission_level", define.FamilyMemberPermissionAdmin); result.Error != nil {
			return result.Error
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if ge, ok := err.(*define.GeneralError); ok {
		return ge.AppendSource("LeaveFamily")
	}
	return define.NewGeneralError("LeaveFamily", err, define.LangKeyFamilyLeaveUnknown)
}

func (h *FamilyHandle) AddMember(tx *gorm.DB, familyUniqueID uint64, userUniqueID uint32) *define.GeneralError {
	member := define.FamilyMember{FamilyUniqueID: familyUniqueID, UserUniqueID: userUniqueID, PermissionLevel: define.FamilyMemberPermissionNormal, JoinedUnixTime: time.Now().Unix()}
	if result := tx.Create(&member); result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return define.NewGeneralError("AddFamilyMember", result.Error, define.LangKeyFamilyAlreadyJoined)
		}
		return define.NewGeneralError("AddFamilyMember", result.Error, define.LangKeyFamilyMemberUpdateUnknown)
	}
	return nil
}

func (h *FamilyHandle) PinTrip(tx *gorm.DB, familyUniqueID uint64, tripIdentity string, tripOwnerUserUniqueID uint32, pinnedByUserUniqueID uint32) *define.GeneralError {
	pin := define.FamilyPinnedTrip{FamilyUniqueID: familyUniqueID, TripIdentity: tripIdentity, TripOwnerUserUniqueID: tripOwnerUserUniqueID, PinnedByUserUniqueID: pinnedByUserUniqueID, CreatedUnixTime: time.Now().Unix()}
	if result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&pin); result.Error != nil {
		return define.NewGeneralError("PinFamilyTrip", result.Error, define.LangKeyFamilyPinnedTripUpdateUnknown)
	}
	return nil
}

func (h *FamilyHandle) UnpinTrip(tx *gorm.DB, familyUniqueID uint64, tripIdentity string) *define.GeneralError {
	result := tx.Where("family_unique_id = ? AND trip_identity = ?", familyUniqueID, tripIdentity).Delete(&define.FamilyPinnedTrip{})
	if result.Error != nil {
		return define.NewGeneralError("UnpinFamilyTrip", result.Error, define.LangKeyFamilyPinnedTripUpdateUnknown)
	}
	if result.RowsAffected != 1 {
		return define.NewGeneralError("UnpinFamilyTrip", fmt.Errorf("pinned trip not found"), define.LangKeyFamilyPinnedTripNotFound)
	}
	return nil
}

func (h *FamilyHandle) QueryPinnedTrips(tx *gorm.DB, familyUniqueID uint64) ([]define.FamilyPinnedTrip, *define.GeneralError) {
	var pins []define.FamilyPinnedTrip
	result := tx.Where("family_unique_id = ?", familyUniqueID).Order("created_unix_time ASC, family_pinned_trip_unique_id ASC").Find(&pins)
	if result.Error != nil {
		return nil, define.NewGeneralError("QueryFamilyPinnedTrips", result.Error, define.LangKeyFamilyPinnedTripQueryUnknown)
	}
	return pins, nil
}

func (h *FamilyHandle) QueryFamilyTrips(tx *gorm.DB, familyUniqueID uint64) ([]define.TripInfo, *define.GeneralError) {
	var trips []define.TripInfo
	result := tx.Table("trip_infos").Joins("JOIN family_members ON family_members.user_unique_id = trip_infos.user_unique_id").Where("family_members.family_unique_id = ?", familyUniqueID).Order("trip_infos.trip_date ASC, trip_infos.trip_unique_id ASC").Find(&trips)
	if result.Error != nil {
		return nil, define.NewGeneralError("QueryFamilyTrips", result.Error, define.LangKeyFamilyQueryUnknown)
	}
	return trips, nil
}

func (h *FamilyHandle) CanReadTrip(tx *gorm.DB, readerUserUniqueID uint32, ownerUserUniqueID uint32) (bool, *define.GeneralError) {
	if readerUserUniqueID == ownerUserUniqueID {
		return true, nil
	}
	var familyID uint64
	result := tx.Table("family_members AS reader").Select("reader.family_unique_id").Joins("JOIN family_members AS owner ON owner.family_unique_id = reader.family_unique_id").Where("reader.user_unique_id = ? AND owner.user_unique_id = ?", readerUserUniqueID, ownerUserUniqueID).Limit(1).Scan(&familyID)
	if result.Error != nil {
		return false, define.NewGeneralError("CanReadTrip", result.Error, define.LangKeyFamilyQueryUnknown)
	}
	return familyID != 0, nil
}
