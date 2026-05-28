package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qdl/server/internal/model"
	dnsservice "qdl/server/internal/service/dns"
	whoisservice "qdl/server/internal/service/whois"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DomainHandler struct {
	*ResourceHandler[model.Domain]
	db *gorm.DB
}

func NewDomainHandler(db *gorm.DB) *DomainHandler {
	return &DomainHandler{ResourceHandler: NewResourceHandler[model.Domain](db), db: db}
}

type createDomainRequest struct {
	DomainAccountID uint     `json:"domainAccountId" binding:"required"`
	DomainNames     []string `json:"domainNames"`
	Name            string   `json:"name"`
	Remark          string   `json:"remark"`
	Status          string   `json:"status"`
}

func (h *DomainHandler) Create(c *gin.Context) {
	var req createDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if req.Status == "" {
		req.Status = model.StatusEnabled
	}
	if len(req.DomainNames) == 0 && req.Name != "" {
		req.DomainNames = []string{req.Name}
	}
	if len(req.DomainNames) == 0 {
		fail(c, http.StatusBadRequest, "请选择域名")
		return
	}

	var account model.DomainAccount
	if err := h.db.First(&account, req.DomainAccountID).Error; err != nil {
		fail(c, http.StatusNotFound, "域名账号不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	providerDomains, err := dnsservice.ListDomains(ctx, dnsAccount(account))
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	providerMap := make(map[string]dnsservice.DomainItem, len(providerDomains))
	for _, item := range providerDomains {
		providerMap[item.Name] = item
	}

	rows := make([]model.Domain, 0, len(req.DomainNames))
	for _, name := range req.DomainNames {
		item := providerMap[name]
		whoisCtx, whoisCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		expiresAt, _ := whoisservice.LookupExpiresAt(whoisCtx, name)
		whoisCancel()
		row := model.Domain{
			Name:             name,
			DomainAccountID:  account.ID,
			ProviderDomainID: item.ID,
			ProviderGrade:    item.Grade,
			DNSProvider:      account.Provider,
			RecordCount:      item.RecordCount,
			ExpiresAt:        expiresAt,
			Remark:           req.Remark,
			Status:           req.Status,
		}
		if err := h.db.Where("name = ? AND domain_account_id = ?", name, account.ID).Assign(row).FirstOrCreate(&row).Error; err != nil {
			fail(c, http.StatusInternalServerError, "保存域名失败")
			return
		}
		rows = append(rows, row)
	}

	ok(c, rows)
}

func (h *DomainHandler) ProviderDomains(c *gin.Context) {
	var account model.DomainAccount
	if err := h.db.First(&account, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "域名账号不存在")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	items, err := dnsservice.ListDomains(ctx, dnsAccount(account))
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var saved []model.Domain
	h.db.Where("domain_account_id = ?", account.ID).Find(&saved)
	savedMap := make(map[string]bool, len(saved))
	for _, item := range saved {
		savedMap[item.Name] = true
	}
	ok(c, gin.H{"items": items, "selected": savedMap})
}

func (h *DomainHandler) RefreshExpires(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var domain model.Domain
	if err := h.db.First(&domain, id).Error; err != nil {
		fail(c, http.StatusNotFound, "域名不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	expiresAt, err := whoisservice.LookupExpiresAt(ctx, domain.Name)
	if err != nil {
		fail(c, http.StatusBadRequest, "查询域名到期时间失败："+err.Error())
		return
	}
	if expiresAt == nil {
		fail(c, http.StatusBadRequest, "WHOIS未返回域名到期时间")
		return
	}

	if err := h.db.Model(&domain).Update("expires_at", expiresAt).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存域名到期时间失败")
		return
	}
	domain.ExpiresAt = expiresAt
	ok(c, domain)
}

func (h *DomainHandler) Records(c *gin.Context) {
	domain, account, found := h.domainAccount(c)
	if !found {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	items, err := dnsservice.ListRecords(ctx, dnsAccount(account), domain.Name)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	rows := make([]model.DomainRecord, 0, len(items))
	for _, item := range items {
		row := recordModel(domain.ID, item)
		if err := h.db.Where("domain_id = ? AND provider_record_id = ?", domain.ID, item.ID).Assign(row).FirstOrCreate(&row).Error; err != nil {
			fail(c, http.StatusInternalServerError, "同步解析记录失败")
			return
		}
		rows = append(rows, row)
	}
	h.db.Model(&domain).Update("record_count", len(rows))
	ok(c, gin.H{"items": rows, "total": len(rows)})
}

func (h *DomainHandler) RecordLines(c *gin.Context) {
	domain, account, found := h.domainAccount(c)
	if !found {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	items, err := dnsservice.ListRecordLines(ctx, dnsAccount(account), domain.Name, domain.ProviderGrade)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"items": items, "total": len(items)})
}

func (h *DomainHandler) CreateRecord(c *gin.Context) {
	domain, account, found := h.domainAccount(c)
	if !found {
		return
	}
	var req dnsservice.RecordInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if isTencentDNS(account.Provider) && strings.EqualFold(req.Type, "NS") {
		fail(c, http.StatusBadRequest, "腾讯云NS记录不允许在系统内管理")
		return
	}
	req.DomainName = domain.Name
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	item, err := dnsservice.CreateRecord(ctx, dnsAccount(account), req)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	row := recordModel(domain.ID, *item)
	if err := h.db.Create(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存解析记录失败")
		return
	}
	ok(c, row)
}

func (h *DomainHandler) UpdateRecord(c *gin.Context) {
	domain, account, found := h.domainAccount(c)
	if !found {
		return
	}
	var row model.DomainRecord
	if err := h.db.Where("domain_id = ?", domain.ID).First(&row, c.Param("recordId")).Error; err != nil {
		fail(c, http.StatusNotFound, "解析记录不存在")
		return
	}
	if isTencentDNS(account.Provider) && strings.EqualFold(row.Type, "NS") {
		fail(c, http.StatusBadRequest, "腾讯云NS记录不可编辑")
		return
	}
	var req dnsservice.RecordInput
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if isTencentDNS(account.Provider) && strings.EqualFold(req.Type, "NS") {
		fail(c, http.StatusBadRequest, "腾讯云NS记录不允许在系统内管理")
		return
	}
	req.DomainName = domain.Name
	req.RecordID = row.ProviderRecordID
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	item, err := dnsservice.UpdateRecord(ctx, dnsAccount(account), req)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	updated := recordModel(domain.ID, *item)
	if err := h.db.Model(&row).Updates(updated).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存解析记录失败")
		return
	}
	h.db.First(&row, row.ID)
	ok(c, row)
}

func (h *DomainHandler) DeleteRecord(c *gin.Context) {
	domain, account, found := h.domainAccount(c)
	if !found {
		return
	}
	var row model.DomainRecord
	if err := h.db.Where("domain_id = ?", domain.ID).First(&row, c.Param("recordId")).Error; err != nil {
		fail(c, http.StatusNotFound, "解析记录不存在")
		return
	}
	if isTencentDNS(account.Provider) && strings.EqualFold(row.Type, "NS") {
		fail(c, http.StatusBadRequest, "腾讯云NS记录不可删除")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if err := dnsservice.DeleteRecord(ctx, dnsAccount(account), domain.Name, row.ProviderRecordID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.Delete(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除解析记录失败")
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (h *DomainHandler) domainAccount(c *gin.Context) (model.Domain, model.DomainAccount, bool) {
	id, _ := strconv.Atoi(c.Param("id"))
	var domain model.Domain
	if err := h.db.First(&domain, id).Error; err != nil {
		fail(c, http.StatusNotFound, "域名不存在")
		return domain, model.DomainAccount{}, false
	}
	var account model.DomainAccount
	if err := h.db.First(&account, domain.DomainAccountID).Error; err != nil {
		fail(c, http.StatusNotFound, "域名账号不存在")
		return domain, account, false
	}
	return domain, account, true
}

func dnsAccount(account model.DomainAccount) dnsservice.Account {
	return dnsservice.Account{Provider: account.Provider, AccessKey: account.AccessKey, SecretKey: account.SecretKey}
}

func isTencentDNS(provider string) bool {
	return strings.EqualFold(provider, "tencentcloud") || strings.EqualFold(provider, "dnspod")
}

func recordModel(domainID uint, item dnsservice.RecordItem) model.DomainRecord {
	return model.DomainRecord{
		DomainID:         domainID,
		ProviderRecordID: item.ID,
		RR:               item.RR,
		Type:             item.Type,
		Value:            item.Value,
		Line:             item.Line,
		TTL:              item.TTL,
		Priority:         item.Priority,
		Remark:           item.Remark,
		Status:           item.Status,
	}
}
