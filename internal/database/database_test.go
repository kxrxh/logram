package database

import (
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&Subscription{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return &DB{db: db}
}

func TestSubscribe(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	err := db.Subscribe(123)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	subscribed, err := db.IsSubscribed(123)
	if err != nil {
		t.Fatalf("IsSubscribed failed: %v", err)
	}
	if !subscribed {
		t.Error("expected user to be subscribed")
	}
}

func TestSubscribeDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	err := db.Subscribe(123)
	if err != nil {
		t.Fatalf("first Subscribe failed: %v", err)
	}

	err = db.Subscribe(123)
	if err != nil {
		t.Fatalf("second Subscribe failed: %v", err)
	}

	subs, err := db.GetAllSubscribers()
	if err != nil {
		t.Fatalf("GetAllSubscribers failed: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(subs))
	}
}

func TestUnsubscribe(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	_ = db.Subscribe(123)

	err := db.Unsubscribe(123)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	subscribed, err := db.IsSubscribed(123)
	if err != nil {
		t.Fatalf("IsSubscribed failed: %v", err)
	}
	if subscribed {
		t.Error("expected user to be unsubscribed")
	}
}

func TestIsSubscribed(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	subscribed, err := db.IsSubscribed(999)
	if err != nil {
		t.Fatalf("IsSubscribed failed: %v", err)
	}
	if subscribed {
		t.Error("expected user to not be subscribed")
	}

	_ = db.Subscribe(999)

	subscribed, err = db.IsSubscribed(999)
	if err != nil {
		t.Fatalf("IsSubscribed failed: %v", err)
	}
	if !subscribed {
		t.Error("expected user to be subscribed")
	}
}

func TestGetAllSubscribers(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	subs, err := db.GetAllSubscribers()
	if err != nil {
		t.Fatalf("GetAllSubscribers failed: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected 0 subscribers, got %d", len(subs))
	}

	_ = db.Subscribe(1)
	_ = db.Subscribe(2)
	_ = db.Subscribe(3)

	subs, err = db.GetAllSubscribers()
	if err != nil {
		t.Fatalf("GetAllSubscribers failed: %v", err)
	}
	if len(subs) != 3 {
		t.Errorf("expected 3 subscribers, got %d", len(subs))
	}
}

func TestNew(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	db, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	err = db.Subscribe(123)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	subscribed, err := db.IsSubscribed(123)
	if err != nil {
		t.Fatalf("IsSubscribed failed: %v", err)
	}
	if !subscribed {
		t.Error("expected user to be subscribed")
	}
}
