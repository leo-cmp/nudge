package db

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	tmpFile, err := os.CreateTemp("", "nudge_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	database, err := Init(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

func TestCreateAndGetPendingReminders(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC()
	rem := &Reminder{
		Message:     "Test reminder",
		Type:        TypeInstant,
		ScheduledAt: &now,
	}

	if err := database.CreateReminder(rem); err != nil {
		t.Fatalf("failed to create reminder: %v", err)
	}

	pending, err := database.GetPendingDueReminders(now.Add(time.Second))
	if err != nil {
		t.Fatalf("failed to get pending reminders: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 pending reminder, got %d", len(pending))
	}

	if pending[0].Message != "Test reminder" {
		t.Errorf("expected message 'Test reminder', got '%s'", pending[0].Message)
	}
}

func TestCancelReminder(t *testing.T) {
	database := setupTestDB(t)

	now := time.Now().UTC()
	rem := &Reminder{
		Message:     "Reminder to cancel",
		Type:        TypeScheduled,
		ScheduledAt: &now,
	}

	if err := database.CreateReminder(rem); err != nil {
		t.Fatalf("failed to create reminder: %v", err)
	}

	if err := database.CancelReminder(rem.ID); err != nil {
		t.Fatalf("failed to cancel reminder: %v", err)
	}

	list, err := database.ListReminders("pending")
	if err != nil {
		t.Fatalf("failed to list reminders: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected 0 pending reminders after cancellation, got %d", len(list))
	}
}
