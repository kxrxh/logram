package database

import "testing"

func TestChatRegexRule_UpsertAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	chatID := int64(1)
	name := "error"
	pattern1 := "ERROR"
	pattern2 := ".*ERROR.*"

	err := db.UpsertChatRegexRule(chatID, name, pattern1)
	if err != nil {
		t.Fatalf("UpsertChatRegexRule failed: %v", err)
	}

	rules, err := db.GetChatRegexRules(chatID)
	if err != nil {
		t.Fatalf("GetChatRegexRules failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ChatID != chatID || rules[0].Name != name || rules[0].Pattern != pattern1 {
		t.Fatalf("unexpected rule: %+v", rules[0])
	}

	err = db.UpsertChatRegexRule(chatID, name, pattern2)
	if err != nil {
		t.Fatalf("UpsertChatRegexRule overwrite failed: %v", err)
	}

	rules, err = db.GetChatRegexRules(chatID)
	if err != nil {
		t.Fatalf("GetChatRegexRules after overwrite failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule after overwrite, got %d", len(rules))
	}
	if rules[0].Pattern != pattern2 {
		t.Fatalf("expected pattern %q, got %q", pattern2, rules[0].Pattern)
	}
}

func TestChatRegexRule_DeleteOne(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	chatID := int64(1)

	err := db.UpsertChatRegexRule(chatID, "a", "^a")
	if err != nil {
		t.Fatalf("UpsertChatRegexRule(a) failed: %v", err)
	}
	err = db.UpsertChatRegexRule(chatID, "b", "^b")
	if err != nil {
		t.Fatalf("UpsertChatRegexRule(b) failed: %v", err)
	}

	err = db.DeleteChatRegexRule(chatID, "a")
	if err != nil {
		t.Fatalf("DeleteChatRegexRule failed: %v", err)
	}

	rules, err := db.GetChatRegexRules(chatID)
	if err != nil {
		t.Fatalf("GetChatRegexRules failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "b" {
		t.Fatalf("expected remaining rule name %q, got %q", "b", rules[0].Name)
	}
}

func TestChatRegexRule_DeleteAllAndGetAll(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	chatID1 := int64(1)
	chatID2 := int64(2)

	err := db.UpsertChatRegexRule(chatID1, "a", "^a")
	if err != nil {
		t.Fatalf("UpsertChatRegexRule(chatID1,a) failed: %v", err)
	}
	err = db.UpsertChatRegexRule(chatID1, "b", "^b")
	if err != nil {
		t.Fatalf("UpsertChatRegexRule(chatID1,b) failed: %v", err)
	}
	err = db.UpsertChatRegexRule(chatID2, "c", "^c")
	if err != nil {
		t.Fatalf("UpsertChatRegexRule(chatID2,c) failed: %v", err)
	}

	all, err := db.GetAllChatRegexRules()
	if err != nil {
		t.Fatalf("GetAllChatRegexRules failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(all))
	}

	err = db.DeleteAllChatRegexRules(chatID1)
	if err != nil {
		t.Fatalf("DeleteAllChatRegexRules failed: %v", err)
	}

	rules1, err := db.GetChatRegexRules(chatID1)
	if err != nil {
		t.Fatalf("GetChatRegexRules(chatID1) failed: %v", err)
	}
	if len(rules1) != 0 {
		t.Fatalf("expected 0 rules for chatID1, got %d", len(rules1))
	}

	rules2, err := db.GetChatRegexRules(chatID2)
	if err != nil {
		t.Fatalf("GetChatRegexRules(chatID2) failed: %v", err)
	}
	if len(rules2) != 1 || rules2[0].Name != "c" {
		t.Fatalf("unexpected rules for chatID2: %+v", rules2)
	}
}
