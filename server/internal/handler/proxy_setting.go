package handler

import (
	"context"
	"fmt"
	"net/http"
	"qdl/server/internal/model"
	proxyservice "qdl/server/internal/service/proxypool"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProxySettingHandler struct {
	*ResourceHandler[model.ProxySetting]
	db *gorm.DB
}

func NewProxySettingHandler(db *gorm.DB) *ProxySettingHandler {
	return &ProxySettingHandler{
		ResourceHandler: NewResourceHandler[model.ProxySetting](db),
		db:              db,
	}
}

func (h *ProxySettingHandler) Create(c *gin.Context) {
	var row model.ProxySetting
	if err := c.ShouldBindJSON(&row); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := proxyservice.Validate(&row); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.Create(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建失败")
		return
	}
	ok(c, row)
}

func (h *ProxySettingHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var row model.ProxySetting
	if err := h.db.First(&row, id).Error; err != nil {
		fail(c, http.StatusNotFound, "数据不存在")
		return
	}
	if err := c.ShouldBindJSON(&row); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := proxyservice.Validate(&row); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.Save(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, row)
}

func (h *ProxySettingHandler) Test(c *gin.Context) {
	var row model.ProxySetting
	if err := h.db.First(&row, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "数据不存在")
		return
	}
	if err := proxyservice.Validate(&row); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	startedAt := time.Now()
	client, err := proxyservice.Client(&row, 8*time.Second)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.baidu.com", nil)
	res, err := client.Do(req)
	if err != nil {
		fail(c, http.StatusBadGateway, "代理检测失败："+err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		fail(c, http.StatusBadGateway, fmt.Sprintf("代理检测失败：目标返回 HTTP %d", res.StatusCode))
		return
	}

	row.LastTestAt = time.Now().Format(time.RFC3339)
	_ = h.db.Model(&row).Update("last_test_at", row.LastTestAt).Error
	ok(c, gin.H{
		"message": fmt.Sprintf("检测通过，耗时 %dms", time.Since(startedAt).Milliseconds()),
	})
}
