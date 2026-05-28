package main

import (
	"log"
	"qdl/server/internal/config"
	"qdl/server/internal/database"
	"qdl/server/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("connect database failed: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("migrate database failed: %v", err)
	}

	if err := database.EnsureInitialInstall(db, cfg.InstallLockPath); err != nil {
		log.Fatalf("ensure initial install failed: %v", err)
	}

	r := router.New(cfg, db)
	if err := r.Run(cfg.Server.Addr()); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
