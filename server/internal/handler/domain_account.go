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

	dnsAccount, err := dnsAccountWithProxy(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := dnsservice.TestAccount(ctx, dnsservice.Account{
		Provider:   dnsAccount.Provider,
		AccessKey:  dnsAccount.AccessKey,
		SecretKey:  dnsAccount.SecretKey,
		ProxyURL:   dnsAccount.ProxyURL,
		HTTPClient: dnsAccount.HTTPClient,
	})
	trace := dnsTrace(account, "TestAccount", gin.H{"provider": account.Provider}, result, err)
	if err != nil {
		logDomainAccount(h.db, account, 0, account.Name, "test", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, 0, account.Name, "test", "success", result.Message, trace)

	now := time.Now().Format(time.RFC3339)
	account.LastTestAt = &now
	if err := h.db.Model(&account).Update("last_test_at", now).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存检测时间失败")
		return
	}

	ok(c, result)
}
