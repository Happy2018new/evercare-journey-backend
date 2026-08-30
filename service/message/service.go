package message

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/auth"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	MaxMessageContentLength = 2000
	MaxMessageTitleLength   = 128
	MaxMessageLimit         = 50
)

func loadUser(session general.BasicSessionInfo, source string) (*define.UserData, *define.GeneralError) {
	status, ge := auth.ValidateSession(session)
	if ge != nil {
		return nil, ge.AppendSource(source)
	}
	if status != auth.ValidateSessionStatusValidSession {
		return nil, define.NewGeneralError(source, fmt.Errorf("invalid session"), define.LangKeyGeneralInvalidSession)
	}
	user, found, ge := auth.LoadUser(session.UserIdentity, false)
	if ge != nil {
		return nil, ge.AppendSource(source)
	}
	if !found {
		return nil, define.NewGeneralError(source, fmt.Errorf("user not found"), define.LangKeyGeneralInvalidSession)
	}
	return user, nil
}

func loadFamily(userID uint32, source string) (define.FamilyInfo, define.FamilyMember, *define.GeneralError) {
	family, member, found, ge := environment.DB.FamilyHandle().QueryFamilyByUser(environment.DB.Database(), userID)
	if ge != nil {
		return family, member, ge.AppendSource(source)
	}
	if !found {
		return family, member, define.NewGeneralError(source, fmt.Errorf("family not found"), define.LangKeyMessageFamilyNotFound)
	}
	return family, member, nil
}

func write(c *gin.Context, response any, ge *define.GeneralError) {
	if ge == nil {
		c.JSON(http.StatusOK, response)
		return
	}
	info := general.FromGeneralError(ge)
	switch value := response.(type) {
	case QueryResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case SendAnnouncementResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case SendSOSResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case SendChatResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case ReadResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	case ReadAllResponse:
		value.BasicResponseInfo = info
		c.JSON(http.StatusOK, value)
	}
}

func validText(source, value string, max int, key string) (string, *define.GeneralError) {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > max {
		return "", define.NewGeneralError(source, fmt.Errorf("invalid text"), key)
	}
	return value, nil
}

func messageData(message define.MessageInfo, recipient define.MessageRecipient, sender *define.UserData) MessageData {
	result := MessageData{
		MessageIdentity:     message.MessageIdentity,
		MessageType:         message.MessageType,
		Title:               message.Title,
		Content:             message.Content,
		RelatedTripIdentity: message.RelatedTripIdentity,
		CreatedUnixTime:     message.CreatedUnixTime,
		Read:                recipient.ReadUnixTime > 0,
	}
	if sender != nil {
		result.SenderUserIdentity = sender.UserIdentity
		result.SenderAccountName = sender.AccountName
	}
	return result
}

func HandleQuery(c *gin.Context) {
	const source = "HandleQueryMessages"
	var request QueryRequest
	if err := c.ShouldBind(&request); err != nil {
		write(c, QueryResponse{}, define.NewGeneralError(source, err, define.LangKeyMessageRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		write(c, QueryResponse{}, ge)
		return
	}
	family, _, ge := loadFamily(user.UserUniqueID, source)
	if ge != nil {
		write(c, QueryResponse{}, ge)
		return
	}
	if request.MessageType != nil && *request.MessageType > define.MessageTypeChat {
		write(c, QueryResponse{}, define.NewGeneralError(source, fmt.Errorf("invalid message type"), define.LangKeyMessageTypeInvalid))
		return
	}
	limit := request.Limit
	if limit <= 0 || limit > MaxMessageLimit {
		limit = MaxMessageLimit
	}
	messages, states, ge := environment.DB.MessageHandle().QueryMessagesForUser(environment.DB.Database(), user.UserUniqueID, family.FamilyUniqueID, request.MessageType, limit)
	if ge != nil {
		write(c, QueryResponse{}, ge)
		return
	}
	result := make([]MessageData, 0, len(messages))
	for _, item := range messages {
		state := states[item.MessageIdentity]
		sender, found, senderErr := environment.DB.UserHandle().QueryUser(environment.DB.Database(), handle.QueryUserActionSearchByUniqueID, item.SenderUserUniqueID)
		if senderErr != nil || !found {
			continue
		}
		result = append(result, messageData(item, state, &sender))
	}
	count, ge := environment.DB.MessageHandle().CountUnread(environment.DB.Database(), user.UserUniqueID, family.FamilyUniqueID)
	if ge != nil {
		write(c, QueryResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, QueryResponse{BasicResponseInfo: general.SuccResponseInfo(), Messages: result, UnreadCount: count})
}

func HandleSendAnnouncement(c *gin.Context) {
	const source = "HandleSendAnnouncement"
	var request SendAnnouncementRequest
	if err := c.ShouldBind(&request); err != nil {
		write(c, SendAnnouncementResponse{}, define.NewGeneralError(source, err, define.LangKeyMessageRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		write(c, SendAnnouncementResponse{}, ge)
		return
	}
	family, member, ge := loadFamily(user.UserUniqueID, source)
	if ge != nil {
		write(c, SendAnnouncementResponse{}, ge)
		return
	}
	if member.PermissionLevel != define.FamilyMemberPermissionAdmin {
		write(c, SendAnnouncementResponse{}, define.NewGeneralError(source, fmt.Errorf("permission denied"), define.LangKeyMessagePermissionDenied))
		return
	}
	title, ge := validText(source, request.Title, MaxMessageTitleLength, define.LangKeyMessageTitleInvalid)
	if ge != nil {
		write(c, SendAnnouncementResponse{}, ge)
		return
	}
	content, ge := validText(source, request.Content, MaxMessageContentLength, define.LangKeyMessageContentInvalid)
	if ge != nil {
		write(c, SendAnnouncementResponse{}, ge)
		return
	}
	members, ge := environment.DB.FamilyHandle().QueryMembers(environment.DB.Database(), family.FamilyUniqueID)
	if ge != nil {
		write(c, SendAnnouncementResponse{}, ge)
		return
	}
	recipients := make([]uint32, 0, len(members))
	for _, item := range members {
		recipients = append(recipients, item.UserUniqueID)
	}
	message, ge := environment.DB.MessageHandle().CreateMessage(
		environment.DB.Database(),
		family.FamilyUniqueID,
		uint64(user.UserUniqueID),
		define.MessageTypeAnnouncement,
		title,
		content,
		strings.TrimSpace(request.RelatedTripIdentity),
		recipients,
	)
	if ge != nil {
		write(c, SendAnnouncementResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, SendAnnouncementResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		Message: messageData(message, define.MessageRecipient{
			RecipientUserUniqueID: user.UserUniqueID,
			ReadUnixTime:          message.CreatedUnixTime,
		}, user),
	})
}

func HandleSendSOS(c *gin.Context) {
	const source = "HandleSendSOS"
	var request SendSOSRequest
	if err := c.ShouldBind(&request); err != nil {
		write(c, SendSOSResponse{}, define.NewGeneralError(source, err, define.LangKeyMessageRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		write(c, SendSOSResponse{}, ge)
		return
	}
	family, _, ge := loadFamily(user.UserUniqueID, source)
	if ge != nil {
		write(c, SendSOSResponse{}, ge)
		return
	}
	content, ge := validText(source, request.Content, MaxMessageContentLength, define.LangKeyMessageContentInvalid)
	if ge != nil {
		write(c, SendSOSResponse{}, ge)
		return
	}
	members, ge := environment.DB.FamilyHandle().QueryMembers(environment.DB.Database(), family.FamilyUniqueID)
	if ge != nil {
		write(c, SendSOSResponse{}, ge)
		return
	}
	recipients := make([]uint32, 0, len(members))
	for _, item := range members {
		recipients = append(recipients, item.UserUniqueID)
	}
	message, ge := environment.DB.MessageHandle().CreateMessage(
		environment.DB.Database(),
		family.FamilyUniqueID,
		uint64(user.UserUniqueID),
		define.MessageTypeSOS,
		"SOS",
		content,
		strings.TrimSpace(request.RelatedTripIdentity),
		recipients,
	)
	if ge != nil {
		write(c, SendSOSResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, SendSOSResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		Message: messageData(message, define.MessageRecipient{
			RecipientUserUniqueID: user.UserUniqueID,
			ReadUnixTime:          message.CreatedUnixTime,
		}, user),
	})
}

func HandleSendChat(c *gin.Context) {
	const source = "HandleSendChat"
	var request SendChatRequest
	if err := c.ShouldBind(&request); err != nil {
		write(c, SendChatResponse{}, define.NewGeneralError(source, err, define.LangKeyMessageRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		write(c, SendChatResponse{}, ge)
		return
	}
	family, _, ge := loadFamily(user.UserUniqueID, source)
	if ge != nil {
		write(c, SendChatResponse{}, ge)
		return
	}
	content, ge := validText(source, request.Content, MaxMessageContentLength, define.LangKeyMessageContentInvalid)
	if ge != nil {
		write(c, SendChatResponse{}, ge)
		return
	}
	members, ge := environment.DB.FamilyHandle().QueryMembers(environment.DB.Database(), family.FamilyUniqueID)
	if ge != nil {
		write(c, SendChatResponse{}, ge)
		return
	}
	recipients := make([]uint32, 0, len(members))
	for _, item := range members {
		recipients = append(recipients, item.UserUniqueID)
	}
	message, ge := environment.DB.MessageHandle().CreateMessage(
		environment.DB.Database(),
		family.FamilyUniqueID,
		uint64(user.UserUniqueID),
		define.MessageTypeChat,
		"",
		content,
		strings.TrimSpace(request.RelatedTripIdentity),
		recipients,
	)
	if ge != nil {
		write(c, SendChatResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, SendChatResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		Message: messageData(message, define.MessageRecipient{
			RecipientUserUniqueID: user.UserUniqueID,
			ReadUnixTime:          message.CreatedUnixTime,
		}, user),
	})
}

func HandleRead(c *gin.Context) {
	const source = "HandleReadMessage"
	var request ReadRequest
	if err := c.ShouldBind(&request); err != nil {
		write(c, ReadResponse{}, define.NewGeneralError(source, err, define.LangKeyMessageRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		write(c, ReadResponse{}, ge)
		return
	}
	identity := strings.TrimSpace(request.MessageIdentity)
	if _, err := uuid.Parse(identity); err != nil {
		write(c, ReadResponse{}, define.NewGeneralError(source, err, define.LangKeyMessageNotFound))
		return
	}
	if ge = environment.DB.MessageHandle().MarkRead(environment.DB.Database(), identity, user.UserUniqueID); ge != nil {
		write(c, ReadResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, ReadResponse{BasicResponseInfo: general.SuccResponseInfo()})
}

func HandleReadAll(c *gin.Context) {
	const source = "HandleReadAllMessages"
	var request ReadAllRequest
	if err := c.ShouldBind(&request); err != nil {
		write(c, ReadAllResponse{}, define.NewGeneralError(source, err, define.LangKeyMessageRequestBodyInvalid))
		return
	}
	user, ge := loadUser(request.BasicSessionInfo, source)
	if ge != nil {
		write(c, ReadAllResponse{}, ge)
		return
	}
	family, _, ge := loadFamily(user.UserUniqueID, source)
	if ge != nil {
		write(c, ReadAllResponse{}, ge)
		return
	}
	if ge = environment.DB.MessageHandle().MarkAllRead(environment.DB.Database(), user.UserUniqueID, family.FamilyUniqueID); ge != nil {
		write(c, ReadAllResponse{}, ge)
		return
	}
	c.JSON(http.StatusOK, ReadAllResponse{BasicResponseInfo: general.SuccResponseInfo()})
}
