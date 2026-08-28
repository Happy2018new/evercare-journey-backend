package handle

import (
	"errors"
	"fmt"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageHandle struct{}

func NewMessageHandle() *MessageHandle { return new(MessageHandle) }

func (h *MessageHandle) CreateMessage(tx *gorm.DB, familyID, senderID uint64, messageType uint8, title, content, relatedTrip string, recipients []uint32) (define.MessageInfo, *define.GeneralError) {
	createdUnixTime := time.Now().Unix()
	message := define.MessageInfo{MessageIdentity: uuid.NewString(), FamilyUniqueID: familyID, SenderUserUniqueID: uint32(senderID), MessageType: messageType, Title: title, Content: content, RelatedTripIdentity: relatedTrip, CreatedUnixTime: createdUnixTime}
	err := tx.Transaction(func(tx *gorm.DB) error {
		if result := tx.Create(&message); result.Error != nil {
			return result.Error
		}
		for _, recipient := range recipients {
			row := define.MessageRecipient{MessageUniqueID: message.MessageIdentity, RecipientUserUniqueID: recipient}
			if recipient == uint32(senderID) {
				row.ReadUnixTime = createdUnixTime
			}
			if result := tx.Create(&row); result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
	if err != nil {
		return message, define.NewGeneralError("CreateMessage", err, define.LangKeyMessageCreateUnknown)
	}
	return message, nil
}

func (h *MessageHandle) QueryMessagesForUser(tx *gorm.DB, userID uint32, familyID uint64, messageType *uint8, limit int) ([]define.MessageInfo, map[string]define.MessageRecipient, *define.GeneralError) {
	if limit <= 0 {
		limit = 50
	}
	query := tx.Table("message_infos AS m").Select("m.*").Joins("JOIN message_recipients AS r ON r.message_unique_id = m.message_identity").Where("r.recipient_user_unique_id = ? AND m.family_unique_id = ?", userID, familyID)
	if messageType != nil {
		query = query.Where("m.message_type = ?", *messageType)
	}
	var messages []define.MessageInfo
	if result := query.Order("m.created_unix_time DESC, m.message_unique_id DESC").Limit(limit).Find(&messages); result.Error != nil {
		return nil, nil, define.NewGeneralError("QueryMessagesForUser", result.Error, define.LangKeyMessageQueryUnknown)
	}
	ids := make([]string, 0, len(messages))
	for _, item := range messages {
		ids = append(ids, item.MessageIdentity)
	}
	recipients := make(map[string]define.MessageRecipient, len(ids))
	if len(ids) > 0 {
		var rows []define.MessageRecipient
		if result := tx.Where("recipient_user_unique_id = ? AND message_unique_id IN ?", userID, ids).Find(&rows); result.Error != nil {
			return nil, nil, define.NewGeneralError("QueryMessagesForUser", result.Error, define.LangKeyMessageQueryUnknown)
		}
		for _, row := range rows {
			recipients[row.MessageUniqueID] = row
		}
	}
	return messages, recipients, nil
}

func (h *MessageHandle) MarkRead(tx *gorm.DB, messageIdentity string, userID uint32) *define.GeneralError {
	result := tx.Model(&define.MessageRecipient{}).Where("message_unique_id = ? AND recipient_user_unique_id = ?", messageIdentity, userID).Update("read_unix_time", time.Now().Unix())
	if result.Error != nil {
		return define.NewGeneralError("MarkMessageRead", result.Error, define.LangKeyMessageReadUnknown)
	}
	if result.RowsAffected == 0 {
		return define.NewGeneralError("MarkMessageRead", errors.New("message recipient not found"), define.LangKeyMessageNotFound)
	}
	return nil
}

func (h *MessageHandle) CountUnread(tx *gorm.DB, userID uint32, familyID uint64) (int64, *define.GeneralError) {
	var count int64
	result := tx.Table("message_recipients AS r").Joins("JOIN message_infos AS m ON m.message_identity = r.message_unique_id").Where("r.recipient_user_unique_id = ? AND m.family_unique_id = ? AND r.read_unix_time = 0", userID, familyID).Count(&count)
	if result.Error != nil {
		return 0, define.NewGeneralError("CountUnreadMessages", fmt.Errorf("%w", result.Error), define.LangKeyMessageQueryUnknown)
	}
	return count, nil
}
