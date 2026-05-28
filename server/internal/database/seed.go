package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"qdl/server/internal/model"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultAdminUsername = "admin"
	defaultAdminPassword = "123456"
)

func EnsureInitialInstall(db *gorm.DB, lockPath string) error {
	if lockPath == "" {
		return nil
	}
	if _, err := os.Stat(lockPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := createDefaultAdminOnce(db); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf("installed_at=%s\n", time.Now().Format(time.RFC3339))
	return os.WriteFile(lockPath, []byte(content), 0644)
}

func createDefaultAdminOnce(db *gorm.DB) error {
	var user model.User
	err := db.Where("username = ?", defaultAdminUsername).First(&user).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return db.Create(&model.User{
		Username:     defaultAdminUsername,
		PasswordHash: string(hash),
		Nickname:     "系统管理员",
		Role:         model.RoleAdmin,
		Status:       model.StatusEnabled,
	}).Error
}
