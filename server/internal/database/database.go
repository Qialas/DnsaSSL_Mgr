package database

import (
	"fmt"
	"net/url"
	"qdl/server/internal/config"
	"qdl/server/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	loc := url.QueryEscape(cfg.Loc)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
		cfg.ParseTime,
		loc,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db.Migrator().HasIndex(&model.Domain{}, "idx_qdl_domains_name") {
		_ = db.Migrator().DropIndex(&model.Domain{}, "idx_qdl_domains_name")
	}
	if db.Migrator().HasTable(&model.Domain{}) && db.Migrator().HasColumn(&model.Domain{}, "auto_ssl") {
		if err := db.Migrator().DropColumn(&model.Domain{}, "auto_ssl"); err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&model.DeployAccount{}) && db.Migrator().HasColumn(&model.DeployAccount{}, "site_name") {
		if err := db.Migrator().DropColumn(&model.DeployAccount{}, "site_name"); err != nil {
			return err
		}
	}
	return db.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='QDL系统数据表'").AutoMigrate(
		&model.User{},
		&model.DomainAccount{},
		&model.SSLAccount{},
		&model.DeployAccount{},
		&model.Domain{},
		&model.DomainRecord{},
		&model.Certificate{},
		&model.AutoTask{},
		&model.OperationLog{},
		&model.LoginLog{},
		&model.DomainAccountLog{},
		&model.CertificateLog{},
		&model.CertificateDeployLog{},
		&model.SystemSetting{},
		&model.ProxySetting{},
		&model.NotificationSetting{},
	)
}
