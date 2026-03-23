package database

import (
	"fmt"

	"gorm.io/gorm/clause"
)

func (db *DB) UpsertChatRegexRule(chatID int64, name, pattern string) error {
	rule := ChatRegexRule{
		ChatID:  chatID,
		Name:    name,
		Pattern: pattern,
	}

	result := db.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "chat_id"},
			{Name: "name"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"pattern"}),
	}).Create(&rule)
	if result.Error != nil {
		return fmt.Errorf(
			"upsert chat regex rule (chat_id=%d, name=%q): %w",
			chatID,
			name,
			result.Error,
		)
	}

	return nil
}

func (db *DB) GetChatRegexRules(chatID int64) ([]ChatRegexRule, error) {
	var rules []ChatRegexRule
	result := db.db.Where("chat_id = ?", chatID).Order("name ASC").Find(&rules)
	if result.Error != nil {
		return nil, fmt.Errorf("get chat regex rules for chat_id=%d: %w", chatID, result.Error)
	}
	return rules, nil
}

func (db *DB) GetAllChatRegexRules() ([]ChatRegexRule, error) {
	var rules []ChatRegexRule
	result := db.db.Order("chat_id ASC, name ASC").Find(&rules)
	if result.Error != nil {
		return nil, fmt.Errorf("get all chat regex rules: %w", result.Error)
	}
	return rules, nil
}

func (db *DB) DeleteChatRegexRule(chatID int64, name string) error {
	result := db.db.Where("chat_id = ? AND name = ?", chatID, name).Delete(&ChatRegexRule{})
	if result.Error != nil {
		return fmt.Errorf(
			"delete chat regex rule (chat_id=%d, name=%q): %w",
			chatID,
			name,
			result.Error,
		)
	}
	return nil
}

func (db *DB) DeleteAllChatRegexRules(chatID int64) error {
	result := db.db.Where("chat_id = ?", chatID).Delete(&ChatRegexRule{})
	if result.Error != nil {
		return fmt.Errorf("delete all chat regex rules for chat_id=%d: %w", chatID, result.Error)
	}
	return nil
}
