package telegram

import (
	"testing"

	"github.com/kxrxh/logram/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *SubscriptionManager {
	t.Helper()

	db, err := database.New(":memory:")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = db.Close()
	})

	return NewSubscriptionManager(db)
}

func TestSubscriptionManager_InitialState(t *testing.T) {
	sm := setupTestDB(t)

	assert.Equal(t, 0, sm.SubscriberCount())
	assert.False(t, sm.IsSubscribed(123))
	assert.False(t, sm.IsSubscribed(456))
}

func TestSubscriptionManager_AddChat(t *testing.T) {
	sm := setupTestDB(t)

	err := sm.AddChat(123, "Test Chat")
	assert.NoError(t, err)
	assert.Equal(t, 1, sm.SubscriberCount())
	assert.True(t, sm.IsSubscribed(123))

	err = sm.AddChat(123, "Test Chat")
	assert.NoError(t, err)
	assert.Equal(t, 1, sm.SubscriberCount())
	assert.True(t, sm.IsSubscribed(123))
}

func TestSubscriptionManager_AddMultipleChats(t *testing.T) {
	sm := setupTestDB(t)

	err := sm.AddChat(123, "Test Chat")
	assert.NoError(t, err)

	err = sm.AddChat(456, "Another Chat")
	assert.NoError(t, err)

	err = sm.AddChat(789, "Third Chat")
	assert.NoError(t, err)

	// Verify all chats were added
	assert.Equal(t, 3, sm.SubscriberCount())
	assert.True(t, sm.IsSubscribed(123))
	assert.True(t, sm.IsSubscribed(456))
	assert.True(t, sm.IsSubscribed(789))
}

func TestSubscriptionManager_GetAllSubscribers(t *testing.T) {
	sm := setupTestDB(t)

	err := sm.AddChat(123, "Test Chat")
	assert.NoError(t, err)

	err = sm.AddChat(456, "Another Chat")
	assert.NoError(t, err)

	subscribers := sm.GetAllSubscribers()
	assert.Equal(t, 2, len(subscribers))
	assert.Contains(t, subscribers, int64(123))
	assert.Contains(t, subscribers, int64(456))
}

func TestSubscriptionManager_RemoveChat(t *testing.T) {
	sm := setupTestDB(t)

	err := sm.AddChat(123, "Test Chat")
	assert.NoError(t, err)

	err = sm.AddChat(456, "Another Chat")
	assert.NoError(t, err)

	err = sm.RemoveChat(123)
	assert.NoError(t, err)
	assert.Equal(t, 1, sm.SubscriberCount())
	assert.False(t, sm.IsSubscribed(123))
	assert.True(t, sm.IsSubscribed(456))
}

func TestSubscriptionManager_RemoveNonExistentChat(t *testing.T) {
	sm := setupTestDB(t)

	err := sm.AddChat(123, "Test Chat")
	assert.NoError(t, err)

	err = sm.RemoveChat(999)
	assert.NoError(t, err)
	assert.Equal(t, 1, sm.SubscriberCount())
	assert.True(t, sm.IsSubscribed(123))
}

func TestSubscriptionManager_RemoveAllChats(t *testing.T) {
	sm := setupTestDB(t)

	err := sm.AddChat(123, "Test Chat")
	assert.NoError(t, err)

	err = sm.AddChat(456, "Another Chat")
	assert.NoError(t, err)

	err = sm.RemoveChat(123)
	assert.NoError(t, err)

	err = sm.RemoveChat(456)
	assert.NoError(t, err)

	assert.Equal(t, 0, sm.SubscriberCount())
	assert.False(t, sm.IsSubscribed(123))
	assert.False(t, sm.IsSubscribed(456))
	assert.Empty(t, sm.GetAllSubscribers())
}
