package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// setupAppDB creates a fresh SqlStore with InitDB for app-related tests.
func setupAppDB(t *testing.T) *SqlStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewSqlStore()
	if err := s.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestSaveApp verifies that a new app record can be saved and retrieved.
func TestSaveApp(t *testing.T) {
	s := setupAppDB(t)

	rec := AppRecord{
		AppID:            "app-weather-bot-a1b2c3",
		Name:             "weather-bot",
		ClientID:         "wb-a1b2c3d4e5f6",
		ClientSecretHash: "bcrypthashhere",
		ScopeCeiling:     []string{"read:weather:*", "write:logs:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}

	if err := s.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp failed: %v", err)
	}
}

// TestSaveApp_DuplicateName verifies that saving two apps with the same name fails.
func TestSaveApp_DuplicateName(t *testing.T) {
	s := setupAppDB(t)

	rec1 := AppRecord{
		AppID:            "app-weather-bot-a1b2c3",
		Name:             "weather-bot",
		ClientID:         "wb-a1b2c3d4e5f6",
		ClientSecretHash: "hash1",
		ScopeCeiling:     []string{"read:weather:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}
	rec2 := AppRecord{
		AppID:            "app-weather-bot-b2c3d4",
		Name:             "weather-bot", // same name
		ClientID:         "wb-b2c3d4e5f6",
		ClientSecretHash: "hash2",
		ScopeCeiling:     []string{"read:weather:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}

	if err := s.SaveApp(rec1); err != nil {
		t.Fatalf("SaveApp first record failed: %v", err)
	}
	if err := s.SaveApp(rec2); err == nil {
		t.Error("expected error on duplicate name, got nil")
	}
}

// TestSaveApp_DuplicateClientID verifies that saving two apps with the same client_id fails.
func TestSaveApp_DuplicateClientID(t *testing.T) {
	s := setupAppDB(t)

	rec1 := AppRecord{
		AppID:            "app-app-one-a1b2c3",
		Name:             "app-one",
		ClientID:         "ao-a1b2c3d4e5f6",
		ClientSecretHash: "hash1",
		ScopeCeiling:     []string{"read:data:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}
	rec2 := AppRecord{
		AppID:            "app-app-two-b2c3d4",
		Name:             "app-two",
		ClientID:         "ao-a1b2c3d4e5f6", // same client_id
		ClientSecretHash: "hash2",
		ScopeCeiling:     []string{"read:data:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}

	if err := s.SaveApp(rec1); err != nil {
		t.Fatalf("SaveApp first record failed: %v", err)
	}
	if err := s.SaveApp(rec2); err == nil {
		t.Error("expected error on duplicate client_id, got nil")
	}
}

// TestGetAppByClientID verifies lookup by client_id returns the correct record.
func TestGetAppByClientID(t *testing.T) {
	s := setupAppDB(t)

	rec := AppRecord{
		AppID:            "app-my-app-d4e5f6",
		Name:             "my-app",
		ClientID:         "ma-d4e5f6g7h8i9",
		ClientSecretHash: "bcrypthashherexx",
		ScopeCeiling:     []string{"read:data:*", "write:data:records"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}

	if err := s.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp failed: %v", err)
	}

	got, err := s.GetAppByClientID(rec.ClientID)
	if err != nil {
		t.Fatalf("GetAppByClientID failed: %v", err)
	}

	if got.AppID != rec.AppID {
		t.Errorf("AppID: want %q, got %q", rec.AppID, got.AppID)
	}
	if got.Name != rec.Name {
		t.Errorf("Name: want %q, got %q", rec.Name, got.Name)
	}
	if got.ClientSecretHash != rec.ClientSecretHash {
		t.Errorf("ClientSecretHash: want %q, got %q", rec.ClientSecretHash, got.ClientSecretHash)
	}
	if len(got.ScopeCeiling) != len(rec.ScopeCeiling) {
		t.Errorf("ScopeCeiling length: want %d, got %d", len(rec.ScopeCeiling), len(got.ScopeCeiling))
	}
	if got.Status != "active" {
		t.Errorf("Status: want %q, got %q", "active", got.Status)
	}
}

// TestGetAppByClientID_NotFound verifies the sentinel error for unknown client_id.
func TestGetAppByClientID_NotFound(t *testing.T) {
	s := setupAppDB(t)

	_, err := s.GetAppByClientID("nonexistent-client-id")
	if !errors.Is(err, ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

// TestGetAppByID verifies lookup by app_id returns the correct record.
func TestGetAppByID(t *testing.T) {
	s := setupAppDB(t)

	rec := AppRecord{
		AppID:            "app-my-svc-f6g7h8",
		Name:             "my-svc",
		ClientID:         "ms-f6g7h8i9j0k1",
		ClientSecretHash: "hashvalue",
		ScopeCeiling:     []string{"read:svc:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}

	if err := s.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp failed: %v", err)
	}

	got, err := s.GetAppByID(rec.AppID)
	if err != nil {
		t.Fatalf("GetAppByID failed: %v", err)
	}
	if got.ClientID != rec.ClientID {
		t.Errorf("ClientID: want %q, got %q", rec.ClientID, got.ClientID)
	}
}

// TestGetAppByID_NotFound verifies the sentinel error for unknown app_id.
func TestGetAppByID_NotFound(t *testing.T) {
	s := setupAppDB(t)

	_, err := s.GetAppByID("app-nonexistent-000000")
	if !errors.Is(err, ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

// TestListApps verifies all saved apps are returned ordered by created_at DESC.
func TestListApps(t *testing.T) {
	s := setupAppDB(t)

	// Insert two apps with different times.
	older := AppRecord{
		AppID:            "app-old-app-111111",
		Name:             "old-app",
		ClientID:         "oa-111111111111",
		ClientSecretHash: "hashA",
		ScopeCeiling:     []string{"read:data:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC().Add(-1 * time.Hour),
		UpdatedAt:        time.Now().UTC().Add(-1 * time.Hour),
		CreatedBy:        "admin",
	}
	newer := AppRecord{
		AppID:            "app-new-app-222222",
		Name:             "new-app",
		ClientID:         "na-222222222222",
		ClientSecretHash: "hashB",
		ScopeCeiling:     []string{"write:data:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}

	if err := s.SaveApp(older); err != nil {
		t.Fatalf("SaveApp older failed: %v", err)
	}
	if err := s.SaveApp(newer); err != nil {
		t.Fatalf("SaveApp newer failed: %v", err)
	}

	apps, err := s.ListApps()
	if err != nil {
		t.Fatalf("ListApps failed: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Ordered by created_at DESC — newer first.
	if apps[0].AppID != newer.AppID {
		t.Errorf("first result: want %q, got %q", newer.AppID, apps[0].AppID)
	}
}

// TestListApps_Empty verifies an empty slice (not nil, not error) when no apps exist.
func TestListApps_Empty(t *testing.T) {
	s := setupAppDB(t)

	apps, err := s.ListApps()
	if err != nil {
		t.Fatalf("ListApps failed: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

// TestUpdateAppCeiling verifies the scope ceiling can be replaced.
func TestUpdateAppCeiling(t *testing.T) {
	s := setupAppDB(t)

	rec := AppRecord{
		AppID:            "app-ceiling-app-c3d4e5",
		Name:             "ceiling-app",
		ClientID:         "ca-c3d4e5f6g7h8",
		ClientSecretHash: "hash",
		ScopeCeiling:     []string{"read:data:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}
	if err := s.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp failed: %v", err)
	}

	newCeiling := []string{"read:data:*", "write:data:*", "read:alerts:*"}
	if err := s.UpdateAppCeiling(rec.AppID, newCeiling); err != nil {
		t.Fatalf("UpdateAppCeiling failed: %v", err)
	}

	got, err := s.GetAppByID(rec.AppID)
	if err != nil {
		t.Fatalf("GetAppByID after update failed: %v", err)
	}
	if len(got.ScopeCeiling) != 3 {
		t.Errorf("expected 3 scopes after update, got %d", len(got.ScopeCeiling))
	}
}

// TestUpdateAppCeiling_NotFound verifies the sentinel error for unknown app_id.
func TestUpdateAppCeiling_NotFound(t *testing.T) {
	s := setupAppDB(t)

	err := s.UpdateAppCeiling("app-nonexistent-000000", []string{"read:x:*"})
	if !errors.Is(err, ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

// TestUpdateAppStatus verifies status transitions (active → inactive).
func TestUpdateAppStatus(t *testing.T) {
	s := setupAppDB(t)

	rec := AppRecord{
		AppID:            "app-status-app-d4e5f6",
		Name:             "status-app",
		ClientID:         "sa-d4e5f6g7h8i9",
		ClientSecretHash: "hash",
		ScopeCeiling:     []string{"read:data:*"},
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}
	if err := s.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp failed: %v", err)
	}

	if err := s.UpdateAppStatus(rec.AppID, "inactive"); err != nil {
		t.Fatalf("UpdateAppStatus failed: %v", err)
	}

	got, err := s.GetAppByID(rec.AppID)
	if err != nil {
		t.Fatalf("GetAppByID after status update failed: %v", err)
	}
	if got.Status != "inactive" {
		t.Errorf("Status: want %q, got %q", "inactive", got.Status)
	}
}

// TestUpdateAppStatus_NotFound verifies the sentinel error for unknown app_id.
func TestUpdateAppStatus_NotFound(t *testing.T) {
	s := setupAppDB(t)

	err := s.UpdateAppStatus("app-nonexistent-000000", "inactive")
	if !errors.Is(err, ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

// TestSaveApp_TokenTTL verifies that TokenTTL round-trips through save and get.
func TestSaveApp_TokenTTL(t *testing.T) {
	s := setupAppDB(t)

	rec := AppRecord{
		AppID:            "app-test-ttl-aaa111",
		Name:             "test-ttl",
		ClientID:         "tt-aaa111bbb222",
		ClientSecretHash: "$2a$12$fakehash",
		ScopeCeiling:     []string{"read:data:*"},
		TokenTTL:         3600,
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}
	if err := s.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp: %v", err)
	}

	got, err := s.GetAppByID("app-test-ttl-aaa111")
	if err != nil {
		t.Fatalf("GetAppByID: %v", err)
	}
	if got.TokenTTL != 3600 {
		t.Fatalf("expected TokenTTL 3600, got %d", got.TokenTTL)
	}
}

// TestScopesCeiling_RoundTrip verifies JSON marshaling of scope arrays survives a DB round-trip.
func TestScopesCeiling_RoundTrip(t *testing.T) {
	s := setupAppDB(t)

	scopes := []string{"read:weather:*", "write:logs:events", "read:alerts:critical"}
	rec := AppRecord{
		AppID:            "app-scope-rt-e5f6g7",
		Name:             "scope-rt",
		ClientID:         "sr-e5f6g7h8i9j0",
		ClientSecretHash: "hash",
		ScopeCeiling:     scopes,
		Status:           "active",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		CreatedBy:        "admin",
	}
	if err := s.SaveApp(rec); err != nil {
		t.Fatalf("SaveApp failed: %v", err)
	}

	got, err := s.GetAppByID(rec.AppID)
	if err != nil {
		t.Fatalf("GetAppByID failed: %v", err)
	}
	if len(got.ScopeCeiling) != len(scopes) {
		t.Fatalf("scope count: want %d, got %d", len(scopes), len(got.ScopeCeiling))
	}
	for i, s := range scopes {
		if got.ScopeCeiling[i] != s {
			t.Errorf("scope[%d]: want %q, got %q", i, s, got.ScopeCeiling[i])
		}
	}
}
