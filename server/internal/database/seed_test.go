package database

import (
	"os"
	"path/filepath"
	"testing"

	"qdl/server/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsureInitialInstallCreatesAdminAndLock(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(t.TempDir(), "config", "install.lock")
	if err := EnsureInitialInstall(db, lockPath); err != nil {
		t.Fatal(err)
	}

	var user model.User
	if err := db.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("123456")); err != nil {
		t.Fatalf("admin password was not 123456: %v", err)
	}
	if user.Role != model.RoleAdmin || user.Status != model.StatusEnabled {
		t.Fatalf("admin role/status = %s/%s, want admin/enabled", user.Role, user.Status)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("install lock was not created: %v", err)
	}
}

func TestEnsureInitialInstallSkipsWhenLocked(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(t.TempDir(), "install.lock")
	if err := os.WriteFile(lockPath, []byte("installed_at=test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInitialInstall(db, lockPath); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Model(&model.User{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("admin count = %d, want 0 when install lock exists", count)
	}
}
