package models

type Account struct {
	AccID             uint64 `json:"AccID"`
	UserName          string `json:"Username"`
	Email             string `json:"Email"`
	EncryptedPassword string `json:"EncryptedPassword"`
	TwoFACode         string `json:"SecretKey2FA"`
	IsEmailVerified   bool   `json:"IsEmailVerified"` // Add this field
}
