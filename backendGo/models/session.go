package models

import "time"

// Session struct for managing sessions
type Session struct {
	SessionID      string    `json:"SessionID"`
	AccID          uint64    `json:"AccID"`
	Metadata       string    `json:"Metadata"`       // Additional metadata, like user agent, IP, etc.
	ExpiryDateTime time.Time `json:"ExpiryDateTime"` // Expiry of the session
}
