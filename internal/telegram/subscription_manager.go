package telegram

import (
	"sync"

	"github.com/kxrxh/logram/internal/database"
)

type SubscriptionManager struct {
	chatIDs map[int64]bool
	mu      sync.RWMutex
	db      *database.DB
}

func NewSubscriptionManager(db *database.DB) *SubscriptionManager {
	sm := &SubscriptionManager{
		chatIDs: make(map[int64]bool),
		db:      db,
	}

	if db != nil {
		chats, err := db.GetAllChats()
		if err == nil {
			for _, chat := range chats {
				sm.chatIDs[chat.ChatID] = true
			}
		}
	}

	return sm
}

func (sm *SubscriptionManager) AddChat(chatID int64, title string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.chatIDs[chatID]; !exists {
		sm.chatIDs[chatID] = true
		if sm.db != nil {
			return sm.db.AddChat(chatID, title)
		}
	}
	return nil
}

func (sm *SubscriptionManager) RemoveChat(chatID int64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.chatIDs[chatID]; exists {
		delete(sm.chatIDs, chatID)
		if sm.db != nil {
			return sm.db.RemoveChat(chatID)
		}
	}
	return nil
}

func (sm *SubscriptionManager) IsSubscribed(chatID int64) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.chatIDs[chatID]
}

func (sm *SubscriptionManager) SubscriberCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.chatIDs)
}

func (sm *SubscriptionManager) GetAllSubscribers() []int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subscribers := make([]int64, 0, len(sm.chatIDs))
	for chatID := range sm.chatIDs {
		subscribers = append(subscribers, chatID)
	}
	return subscribers
}
