package events

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *SQLiteStore {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	return store
}

func TestSQLiteStore_SaveAndGet(t *testing.T) {
	store := setupTestDB(t)

	// Create a test event
	event := NewEvent(
		EventMessage,
		"test-source",
		"test-target",
		PriorityNormal,
		map[string]interface{}{
			"message": "test message",
			"count":   42,
		},
	)

	// Save the event
	err := store.Save(event)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Retrieve pending events
	pending, err := store.GetPending("test-target", nil)
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}

	// Verify we got the event back
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending event, got %d", len(pending))
	}

	retrieved := pending[0]
	if retrieved.ID != event.ID {
		t.Errorf("expected ID %s, got %s", event.ID, retrieved.ID)
	}
	if retrieved.Type != event.Type {
		t.Errorf("expected Type %s, got %s", event.Type, retrieved.Type)
	}
	if retrieved.Source != event.Source {
		t.Errorf("expected Source %s, got %s", event.Source, retrieved.Source)
	}
	if retrieved.Target != event.Target {
		t.Errorf("expected Target %s, got %s", event.Target, retrieved.Target)
	}
	if retrieved.Priority != event.Priority {
		t.Errorf("expected Priority %d, got %d", event.Priority, retrieved.Priority)
	}

	// Verify payload
	if msg, ok := retrieved.Payload["message"].(string); !ok || msg != "test message" {
		t.Errorf("expected payload message 'test message', got %v", retrieved.Payload["message"])
	}
	if count, ok := retrieved.Payload["count"].(float64); !ok || count != 42 {
		t.Errorf("expected payload count 42, got %v", retrieved.Payload["count"])
	}
}

func TestSQLiteStore_MarkDelivered(t *testing.T) {
	store := setupTestDB(t)

	// Create and save an event
	event := NewEvent(
		EventMessage,
		"test-source",
		"test-target",
		PriorityNormal,
		map[string]interface{}{"test": "data"},
	)

	err := store.Save(event)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify it's pending
	pending, err := store.GetPending("test-target", nil)
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending event, got %d", len(pending))
	}

	// Mark as delivered
	err = store.MarkDelivered(event.ID)
	if err != nil {
		t.Fatalf("MarkDelivered failed: %v", err)
	}

	// Verify it's no longer pending
	pending, err = store.GetPending("test-target", nil)
	if err != nil {
		t.Fatalf("GetPending failed after marking delivered: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending events after marking delivered, got %d", len(pending))
	}
}

func TestSQLiteStore_FilterByType(t *testing.T) {
	store := setupTestDB(t)

	// Create events of different types
	event1 := NewEvent(EventMessage, "source1", "target1", PriorityNormal, map[string]interface{}{"msg": "one"})
	event2 := NewEvent(EventAlert, "source2", "target1", PriorityHigh, map[string]interface{}{"msg": "two"})
	event3 := NewEvent(EventTask, "source3", "target1", PriorityNormal, map[string]interface{}{"msg": "three"})

	store.Save(event1)
	store.Save(event2)
	store.Save(event3)

	// Get all pending events (no type filter)
	allPending, err := store.GetPending("target1", nil)
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}
	if len(allPending) != 3 {
		t.Errorf("expected 3 pending events, got %d", len(allPending))
	}

	// Get only message events
	messagePending, err := store.GetPending("target1", []EventType{EventMessage})
	if err != nil {
		t.Fatalf("GetPending with filter failed: %v", err)
	}
	if len(messagePending) != 1 {
		t.Errorf("expected 1 message event, got %d", len(messagePending))
	}
	if messagePending[0].Type != EventMessage {
		t.Errorf("expected EventMessage, got %s", messagePending[0].Type)
	}

	// Get alert and task events
	multiTypePending, err := store.GetPending("target1", []EventType{EventAlert, EventTask})
	if err != nil {
		t.Fatalf("GetPending with multiple type filter failed: %v", err)
	}
	if len(multiTypePending) != 2 {
		t.Errorf("expected 2 events (alert+task), got %d", len(multiTypePending))
	}

	// Verify the types
	foundAlert := false
	foundTask := false
	for _, e := range multiTypePending {
		if e.Type == EventAlert {
			foundAlert = true
		}
		if e.Type == EventTask {
			foundTask = true
		}
	}
	if !foundAlert || !foundTask {
		t.Errorf("expected both alert and task events, got alert=%v task=%v", foundAlert, foundTask)
	}
}

func TestSQLiteStore_GetPendingForAll(t *testing.T) {
	store := setupTestDB(t)

	// Create events with different targets
	event1 := NewEvent(EventMessage, "source1", "target1", PriorityNormal, map[string]interface{}{"msg": "one"})
	event2 := NewEvent(EventMessage, "source2", "target2", PriorityNormal, map[string]interface{}{"msg": "two"})
	event3 := NewEvent(EventMessage, "source3", "all", PriorityNormal, map[string]interface{}{"msg": "broadcast"})

	store.Save(event1)
	store.Save(event2)
	store.Save(event3)

	// Get pending for target1 - should get target1 and "all"
	pending1, err := store.GetPending("target1", nil)
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}
	if len(pending1) != 2 {
		t.Errorf("expected 2 events for target1 (itself + 'all'), got %d", len(pending1))
	}

	// Get pending for target2 - should get target2 and "all"
	pending2, err := store.GetPending("target2", nil)
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}
	if len(pending2) != 2 {
		t.Errorf("expected 2 events for target2 (itself + 'all'), got %d", len(pending2))
	}

	// Get pending for "all" target - should get only "all" events
	pendingAll, err := store.GetPending("all", nil)
	if err != nil {
		t.Fatalf("GetPending failed: %v", err)
	}
	if len(pendingAll) != 1 {
		t.Errorf("expected 1 event for 'all' target, got %d", len(pendingAll))
	}
}

func TestSQLiteStore_Cleanup(t *testing.T) {
	store := setupTestDB(t)

	// Create old and new events
	oldEvent := NewEvent(EventMessage, "source1", "target1", PriorityNormal, map[string]interface{}{"msg": "old"})
	// Manually set created time to 2 hours ago
	oldEvent.CreatedAt = time.Now().Add(-2 * time.Hour)

	newEvent := NewEvent(EventMessage, "source2", "target1", PriorityNormal, map[string]interface{}{"msg": "new"})

	store.Save(oldEvent)
	store.Save(newEvent)

	// Mark old event as delivered
	store.MarkDelivered(oldEvent.ID)

	// Run cleanup for events older than 1 hour
	err := store.Cleanup(1 * time.Hour)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify old delivered event is gone
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", oldEvent.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected old delivered event to be cleaned up, but it still exists")
	}

	// Verify new event still exists
	err = store.db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", newEvent.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected new event to still exist, but count is %d", count)
	}
}
