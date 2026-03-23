package database

import "time"

type Subscription struct {
	UserID    int64     `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type Chat struct {
	ChatID  int64     `gorm:"primaryKey"`
	Title   string    `gorm:"default:''"`
	AddedAt time.Time `gorm:"autoCreateTime"`
}

type ChatRegexRule struct {
	ChatID  int64  `gorm:"primaryKey;column:chat_id"`
	Name    string `gorm:"primaryKey;column:name"`
	Pattern string `gorm:"column:pattern"`
}
