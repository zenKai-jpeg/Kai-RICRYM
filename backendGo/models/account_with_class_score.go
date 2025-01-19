package models

// AccountWithClassAndScore struct
type AccountWithClassAndScore struct {
	AccID    uint64 `json:"AccID"`
	UserName string `json:"Username"`
	Email    string `json:"Email"`
	ClassID  int    `json:"ClassID"`
	Score    int    `json:"Score"`
	Rank     int    `json:"Rank"`
}
