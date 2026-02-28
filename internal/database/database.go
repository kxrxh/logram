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

type DB struct {
	db *gorm.DB
}

func New(path string) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.AutoMigrate(&Subscription{}); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return &DB{db: db}, nil
}

func (db *DB) Subscribe(userID int64) error {
	return db.db.FirstOrCreate(&Subscription{UserID: userID}).Error
}

func (db *DB) Unsubscribe(userID int64) error {
	return db.db.Delete(&Subscription{}, "user_id = ?", userID).Error
}

func (db *DB) IsSubscribed(userID int64) (bool, error) {
	var count int64
	err := db.db.Model(&Subscription{}).Where("user_id = ?", userID).Count(&count).Error
	return count > 0, err
}

func (db *DB) GetAllSubscribers() ([]int64, error) {
	var subs []Subscription
	err := db.db.Find(&subs).Error
	if err != nil {
		return nil, err
	}
	users := make([]int64, len(subs))
	for i, s := range subs {
		users[i] = s.UserID
	}
	return users, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
