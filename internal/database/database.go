package database

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Subscription struct {
	UserID    int64     `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type Chat struct {
	ChatID  int64     `gorm:"primaryKey"`
	Title   string    `gorm:"default:''"`
	AddedAt time.Time `gorm:"autoCreateTime"`
}

type DB struct {
	db *gorm.DB
}

func New(path string) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.AutoMigrate(&Subscription{}, &Chat{}); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return &DB{db: db}, nil
}

func (db *DB) Subscribe(userID int64) error {
	result := db.db.FirstOrCreate(&Subscription{UserID: userID})
	if result.Error != nil {
		return fmt.Errorf("subscribe user %d: %w", userID, result.Error)
	}
	return nil
}

func (db *DB) Unsubscribe(userID int64) error {
	result := db.db.Delete(&Subscription{}, "user_id = ?", userID)
	if result.Error != nil {
		return fmt.Errorf("unsubscribe user %d: %w", userID, result.Error)
	}
	return nil
}

func (db *DB) IsSubscribed(userID int64) (bool, error) {
	var count int64
	result := db.db.Model(&Subscription{}).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check subscription for user %d: %w", userID, result.Error)
	}
	return count > 0, nil
}

func (db *DB) GetAllSubscribers() ([]int64, error) {
	var subs []Subscription
	result := db.db.Find(&subs)
	if result.Error != nil {
		return nil, fmt.Errorf("get all subscribers: %w", result.Error)
	}
	users := make([]int64, len(subs))
	for i, s := range subs {
		users[i] = s.UserID
	}
	return users, nil
}

func (db *DB) AddChat(chatID int64, title string) error {
	result := db.db.FirstOrCreate(&Chat{ChatID: chatID, Title: title})
	if result.Error != nil {
		return fmt.Errorf("add chat %d: %w", chatID, result.Error)
	}
	return nil
}

func (db *DB) RemoveChat(chatID int64) error {
	result := db.db.Delete(&Chat{}, "chat_id = ?", chatID)
	if result.Error != nil {
		return fmt.Errorf("remove chat %d: %w", chatID, result.Error)
	}
	return nil
}

func (db *DB) GetAllChats() ([]Chat, error) {
	var chats []Chat
	result := db.db.Find(&chats)
	if result.Error != nil {
		return nil, fmt.Errorf("get all chats: %w", result.Error)
	}
	return chats, nil
}

func (db *DB) ChatExists(chatID int64) (bool, error) {
	var count int64
	result := db.db.Model(&Chat{}).Where("chat_id = ?", chatID).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check chat %d exists: %w", chatID, result.Error)
	}
	return count > 0, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
