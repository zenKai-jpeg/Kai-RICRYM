package models

import "time"

// Account struct represents the user account with verification status and 2FA secret
type Account struct {
	AccID             uint64 `json:"AccID"`
	UserName          string `json:"Username"`
	Email             string `json:"Email"`
	EncryptedPassword string `json:"EncryptedPassword"`
	SecretKey2FA      string `json:"SecretKey2FA"`
	IsEmailVerified   bool   `json:"IsEmailVerified"` // Indicates if the email is verified
}

// AccountWithClassAndScore struct includes class ID, score, and rank information for the account
type AccountWithClassAndScore struct {
	AccID    uint64 `json:"AccID"`
	UserName string `json:"Username"`
	Email    string `json:"Email"`
	ClassID  int    `json:"ClassID"`
	Score    int    `json:"Score"`
	Rank     int    `json:"Rank"`
}

// Session struct represents a user session with expiration and additional metadata
type Session struct {
	SessionID      string    `json:"SessionID"`
	AccID          uint64    `json:"AccID"`
	Metadata       string    `json:"Metadata"`       // Additional metadata (user agent, IP, etc.)
	ExpiryDateTime time.Time `json:"ExpiryDateTime"` // Expiry time of the session
}

// EmailVerification struct represents the email verification entry with token and 2FA secret
type EmailVerification struct {
	ID                uint64    `json:"ID"`
	AccID             uint64    `json:"AccID"`
	VerificationToken string    `json:"VerificationToken"`
	SecretKey2FA      string    `json:"SecretKey2FA"`
	CreatedAt         time.Time `json:"CreatedAt"` // Time when the verification was created
}
