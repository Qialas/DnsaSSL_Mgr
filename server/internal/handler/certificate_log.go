package handler

import (
	"net/http"
	"strconv"

	"qdl/server/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CertificateLogHandler struct {
	*ResourceHandler[model.CertificateLog]
	db *gorm.DB
}

func NewCertificateLogHandler(db *gorm.DB) *CertificateLogHandler {
	return &CertificateLogHandler{ResourceHandler: NewResourceHandler[model.CertificateLog](db), db: db}
}

func (h *CertificateLogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := h.db.Model(&model.CertificateLog{})
	if certificateID := c.Query("certificateId"); certificateID != "" {
		query = query.Where("certificate_id = ?", certificateID)
	}

	var rows []model.CertificateLog
	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询总数失败")
		return
	}
	if err := query.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询列表失败")
		return
	}
	ok(c, gin.H{"items": rows, "total": total, "page": page, "pageSize": pageSize})
}
