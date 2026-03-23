package database

import (
	"os"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	// #nosec G703 // safe: path created by os.CreateTemp in test, located in system temp dir
	defer func() {
		name := tmpFile.Name()
		if strings.HasPrefix(name, os.TempDir()) {
			_ = os.Remove(name)
		}
	}()

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
