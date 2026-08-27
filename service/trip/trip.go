package trip

import "time"

type TripVersionData struct {
	TripIdentity string `json:"trip_identity"`
	Version      uint32 `json:"version"`
}

type TripNodeData struct {
	PlaceIdentity string `json:"place_identity"`
	NoteString    string `json:"note_string,omitempty"`
	IsCompleted   bool   `json:"is_completed"`
}

type TripData struct {
	TripIdentity   string         `json:"trip_identity"`
	TripName       string         `json:"trip_name"`
	TripDate       time.Time      `json:"trip_date"`
	TravelMode     uint8          `json:"travel_mode"`
	TripStatus     uint8          `json:"trip_status"`
	CurrentVersion uint32         `json:"current_version"`
	UpdateUnixTime int64          `json:"update_unix_time"`
	Nodes          []TripNodeData `json:"nodes"`
}
