package handler

import (
	"context"
	"net/http"
	"time"

	"qdl/server/internal/model"
	dnsservice "qdl/server/internal/service/dns"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DomainAccountHandler struct {
	*ResourceHandler[model.DomainAccount]
	db *gorm.DB
}

func NewDomainAccountHandler(db *gorm.DB) *DomainAccountHandler {
	return &DomainAccountHandler{
		ResourceHandler: NewResourceHandler[model.DomainAccount](db),
		db:              db,
	}
}

func (h *DomainAccountHandler) Test(c *gin.Context) {
	var account model.DomainAccount
	if err := h.db.First(&account, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "域名账号不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	result, err := dnsservice.TestAccount(ctx, dnsservice.Account{
		Provider:  account.Provider,
		AccessKey: account.AccessKey,
		SecretKey: account.SecretKey,
	})
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now().Format(time.RFC3339)
	account.LastTestAt = &now
	if err := h.db.Model(&account).Update("last_test_at", now).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存检测时间失败")
		return
	}

	ok(c, result)
}
