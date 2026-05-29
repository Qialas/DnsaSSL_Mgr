package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qdl/server/internal/model"
	dnsservice "qdl/server/internal/service/dns"
	proxyservice "qdl/server/internal/service/proxypool"
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
	dnsAccount, err := dnsAccountWithProxy(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	providerDomains, err := dnsservice.ListDomains(ctx, dnsAccount)
	trace := dnsTrace(account, "ListDomains", gin.H{"accountId": account.ID}, providerDomains, err)
	if err != nil {
		logDomainAccount(h.db, account, 0, account.Name, "domains", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, 0, account.Name, "domains", "success", "已获取账号域名列表", trace)
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
	dnsAccount, err := dnsAccountWithProxy(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	items, err := dnsservice.ListDomains(ctx, dnsAccount)
	trace := dnsTrace(account, "ListDomains", gin.H{"accountId": account.ID}, items, err)
	if err != nil {
		logDomainAccount(h.db, account, 0, account.Name, "domains", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, 0, account.Name, "domains", "success", "已获取账号域名列表", trace)
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
	dnsAccount, err := dnsAccountWithProxy(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	items, err := dnsservice.ListRecords(ctx, dnsAccount, domain.Name)
	trace := dnsTrace(account, "ListRecords", gin.H{"domainName": domain.Name}, items, err)
	if err != nil {
		logDomainAccount(h.db, account, domain.ID, domain.Name, "records", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, domain.ID, domain.Name, "records", "success", "已获取解析记录列表", trace)
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
	dnsAccount, err := dnsAccountWithProxy(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	items, err := dnsservice.ListRecordLines(ctx, dnsAccount, domain.Name, domain.ProviderGrade)
	trace := dnsTrace(account, "ListRecordLines", gin.H{"domainName": domain.Name, "domainGrade": domain.ProviderGrade}, items, err)
	if err != nil {
		logDomainAccount(h.db, account, domain.ID, domain.Name, "record_lines", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, domain.ID, domain.Name, "record_lines", "success", "已获取解析线路列表", trace)
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
	dnsAccount, err := dnsAccountWithProxy(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := dnsservice.CreateRecord(ctx, dnsAccount, req)
	trace := dnsTrace(account, "CreateRecord", req, item, err)
	if err != nil {
		logDomainAccount(h.db, account, domain.ID, domain.Name, "create_record", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, domain.ID, domain.Name, "create_record", "success", "已新增解析记录", trace)
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
	dnsAccount, err := dnsAccountWithProxy(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := dnsservice.UpdateRecord(ctx, dnsAccount, req)
	trace := dnsTrace(account, "UpdateRecord", req, item, err)
	if err != nil {
		logDomainAccount(h.db, account, domain.ID, domain.Name, "update_record", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, domain.ID, domain.Name, "update_record", "success", "已更新解析记录", trace)
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
	dnsAccount, proxyErr := dnsAccountWithProxy(h.db, account)
	if proxyErr != nil {
		fail(c, http.StatusBadRequest, proxyErr.Error())
		return
	}
	err := dnsservice.DeleteRecord(ctx, dnsAccount, domain.Name, row.ProviderRecordID)
	trace := dnsTrace(account, "DeleteRecord", gin.H{"domainName": domain.Name, "recordId": row.ProviderRecordID}, gin.H{"deleted": err == nil}, err)
	if err != nil {
		logDomainAccount(h.db, account, domain.ID, domain.Name, "delete_record", "failed", err.Error(), trace)
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logDomainAccount(h.db, account, domain.ID, domain.Name, "delete_record", "success", "已删除解析记录", trace)
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

func dnsAccountWithProxy(db *gorm.DB, account model.DomainAccount) (dnsservice.Account, error) {
	resolved := dnsservice.Account{Provider: account.Provider, AccessKey: account.AccessKey, SecretKey: account.SecretKey}
	proxySetting, err := domainAccountProxy(db, account)
	if err != nil {
		return resolved, err
	}
	if proxySetting == nil {
		return resolved, nil
	}
	client, err := proxyservice.Client(proxySetting, 30*time.Second)
	if err != nil {
		return resolved, err
	}
	resolved.HTTPClient = client
	resolved.ProxyURL = proxyservice.URL(*proxySetting).String()
	return resolved, nil
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
