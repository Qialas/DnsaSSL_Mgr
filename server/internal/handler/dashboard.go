package handler

import (
	"time"

	"qdl/server/internal/model"
	systemservice "qdl/server/internal/service/system"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

func (h *DashboardHandler) Overview(c *gin.Context) {
	var domains, certs, tasks, expiringCerts int64
	h.db.Model(&model.Domain{}).Count(&domains)
	h.db.Model(&model.Certificate{}).Count(&certs)
	h.db.Model(&model.AutoTask{}).Count(&tasks)
	now := time.Now()
	sevenDaysLater := now.AddDate(0, 0, 7)
	h.db.Model(&model.Certificate{}).
		Where("expires_at IS NOT NULL AND expires_at >= ? AND expires_at <= ?", now, sevenDaysLater).
		Count(&expiringCerts)

	ok(c, gin.H{
		"domains":         domains,
		"certificates":    certs,
		"tasks":           tasks,
		"expiringCerts7d": expiringCerts,
		"server":          systemservice.GetMetrics(),
		"serverUpdatedAt": time.Now(),
	})
}
