// internal/tasks/store_test.go
package tasks

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*Store, func()) {
	f, err := os.CreateTemp("", "tasks-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	db, err := sql.Open("sqlite3", f.Name())
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(f.Name())
	}

	return store, cleanup
}

func TestStoreSaveAndLoad(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	task := NewTask("Test task", "Description", 3)

	// Save
	if err := store.Save(task); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := store.GetByID(task.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if loaded.Title != task.Title {
		t.Errorf("title mismatch: %q != %q", loaded.Title, task.Title)
	}
	if loaded.Priority != task.Priority {
		t.Errorf("priority mismatch: %d != %d", loaded.Priority, task.Priority)
	}
}

func TestStoreGetByStatus(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	t1 := NewTask("Task 1", "", 3)
	time.Sleep(1 * time.Millisecond) // Ensure different ID
	t2 := NewTask("Task 2", "", 3)
	t2.Status = StatusAssigned

	if err := store.Save(t1); err != nil {
		t.Fatalf("Save t1 failed: %v", err)
	}
	if err := store.Save(t2); err != nil {
		t.Fatalf("Save t2 failed: %v", err)
	}

	pending, err := store.GetByStatus(StatusPending)
	if err != nil {
		t.Fatal(err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(pending))
	}
}
