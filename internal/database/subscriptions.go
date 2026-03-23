package database

import "fmt"

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
