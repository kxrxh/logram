package database

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

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

func (db *DB) SetChatBatchEnabled(chatID int64, enabled bool) error {
	result := db.db.Model(&Chat{}).
		Where("chat_id = ?", chatID).
		Update("batch_enabled", enabled)
	if result.Error != nil {
		return fmt.Errorf("set chat batch_enabled (chat_id=%d): %w", chatID, result.Error)
	}

	if result.RowsAffected == 0 {
		if err := db.db.FirstOrCreate(
			&Chat{ChatID: chatID, BatchEnabled: enabled},
		).Error; err != nil {
			return fmt.Errorf("create chat for batch_enabled (chat_id=%d): %w", chatID, err)
		}
	}

	return nil
}

func (db *DB) GetChatBatchEnabled(chatID int64) (bool, error) {
	var chat Chat
	result := db.db.First(&chat, "chat_id = ?", chatID)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get chat batch_enabled (chat_id=%d): %w", chatID, result.Error)
	}
	return chat.BatchEnabled, nil
}
