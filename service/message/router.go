package message

import "github.com/Happy2018new/evercare-journey-backend/service/general"

type MessageData struct {
	MessageIdentity     string `json:"message_identity"`
	MessageType         uint8  `json:"message_type"`
	SenderUserIdentity  string `json:"sender_user_identity"`
	SenderAccountName   string `json:"sender_account_name"`
	Title               string `json:"title"`
	Content             string `json:"content"`
	RelatedTripIdentity string `json:"related_trip_identity,omitempty"`
	CreatedUnixTime     int64  `json:"created_unix_time"`
	Read                bool   `json:"read"`
}

type QueryRequest struct {
	general.BasicSessionInfo
	MessageType *uint8 `json:"message_type"`
	Limit       int    `json:"limit"`
}
type QueryResponse struct {
	general.BasicResponseInfo
	Messages    []MessageData `json:"messages"`
	UnreadCount int64         `json:"unread_count"`
}
type SendAnnouncementRequest struct {
	general.BasicSessionInfo
	Title               string `json:"title"`
	Content             string `json:"content"`
	RelatedTripIdentity string `json:"related_trip_identity"`
}
type SendAnnouncementResponse struct {
	general.BasicResponseInfo
	Message MessageData `json:"message"`
}
type SendSOSRequest struct {
	general.BasicSessionInfo
	Content             string `json:"content"`
	RelatedTripIdentity string `json:"related_trip_identity"`
}
type SendSOSResponse struct {
	general.BasicResponseInfo
	Message MessageData `json:"message"`
}
type SendChatRequest struct {
	general.BasicSessionInfo
	Content             string `json:"content"`
	RelatedTripIdentity string `json:"related_trip_identity"`
}
type SendChatResponse struct {
	general.BasicResponseInfo
	Message MessageData `json:"message"`
}
type ReadRequest struct {
	general.BasicSessionInfo
	MessageIdentity string `json:"message_identity"`
}
type ReadResponse struct{ general.BasicResponseInfo }
type ReadAllRequest struct{ general.BasicSessionInfo }
type ReadAllResponse struct{ general.BasicResponseInfo }
