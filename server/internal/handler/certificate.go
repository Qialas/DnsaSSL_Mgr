package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"qdl/server/internal/model"
	dnsservice "qdl/server/internal/service/dns"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/acme"
	"gorm.io/gorm"
)

const (
	certStatusPending   = "pending"
	certStatusApplying  = "applying"
	certStatusDNSAdded  = "dns_added"
	certStatusSubmitted = "submitted"
	certStatusIssued    = "issued"
	certStatusFailed    = "failed"
	certStatusExpired   = "expired"
	certStatusCanceled  = "canceled"
	certStatusRevoking  = "revoking"
	certStatusRevoked   = "revoked"
	tencentSSLHost      = "ssl.tencentcloudapi.com"
	tencentSSLVersion   = "2019-12-05"
	certFlowInterval    = 10 * time.Second
	certFlowMaxChecks   = 8640
)

type CertificateHandler struct {
	*ResourceHandler[model.Certificate]
	db *gorm.DB
}

func NewCertificateHandler(db *gorm.DB) *CertificateHandler {
	return &CertificateHandler{ResourceHandler: NewResourceHandler[model.Certificate](db), db: db}
}

type certificateRequest struct {
	DomainID       uint   `json:"domainId" binding:"required"`
	SSLAccountID   uint   `json:"sslAccountId" binding:"required"`
	CommonName     string `json:"commonName"`
	SANs           string `json:"sans"`
	RenewBeforeDay int    `json:"renewBeforeDay"`
}

type certificateDeployRequest struct {
	DeployAccountID uint   `json:"deployAccountId" binding:"required"`
	SiteName        string `json:"siteName"`
}

func (h *CertificateHandler) Create(c *gin.Context) {
	var req certificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	domain, account, valid := h.validateRequest(c, req)
	if !valid {
		return
	}

	commonName := strings.TrimSpace(req.CommonName)
	if commonName == "" {
		commonName = domain.Name
	}
	renewBeforeDay := req.RenewBeforeDay
	if renewBeforeDay <= 0 {
		renewBeforeDay = 30
	}

	row := model.Certificate{
		DomainID:       domain.ID,
		SSLAccountID:   account.ID,
		CommonName:     commonName,
		SANs:           strings.TrimSpace(req.SANs),
		RenewBeforeDay: renewBeforeDay,
		Status:         certStatusApplying,
	}
	if err := h.db.Create(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建证书申请失败")
		return
	}

	trace, err := h.startCertificateFlow(c.Request.Context(), &row, domain, &account)
	if err != nil {
		h.db.Model(&row).Updates(map[string]any{"status": certStatusFailed, "provider_status_msg": err.Error()})
		h.logCertificate(row, account, "apply", "failed", err.Error(), traceFromError(err))
		fail(c, http.StatusBadRequest, "证书申请失败："+err.Error())
		return
	}
	h.db.First(&row, row.ID)
	h.logCertificate(row, account, "apply", "success", "证书申请已创建，DNS验证记录已添加", trace)
	h.startCertificateIssueFlow(row.ID, account.ID)
	ok(c, row)
}

func (h *CertificateHandler) Update(c *gin.Context) {
	var row model.Certificate
	if err := h.db.First(&row, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "证书不存在")
		return
	}

	var req certificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	domain, account, valid := h.validateRequest(c, req)
	if !valid {
		return
	}

	commonName := strings.TrimSpace(req.CommonName)
	if commonName == "" {
		commonName = domain.Name
	}
	renewBeforeDay := req.RenewBeforeDay
	if renewBeforeDay <= 0 {
		renewBeforeDay = row.RenewBeforeDay
	}
	if renewBeforeDay <= 0 {
		renewBeforeDay = 30
	}

	updates := model.Certificate{
		DomainID:       domain.ID,
		SSLAccountID:   account.ID,
		CommonName:     commonName,
		SANs:           strings.TrimSpace(req.SANs),
		RenewBeforeDay: renewBeforeDay,
	}
	if err := h.db.Model(&row).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存证书申请失败")
		return
	}
	h.db.First(&row, row.ID)
	ok(c, row)
}

func (h *CertificateHandler) Submit(c *gin.Context) {
	row, account, found := h.certificateAccount(c)
	if !found {
		return
	}
	if !strings.EqualFold(account.Provider, "tencent_free") && !isACMEProvider(account.Provider) {
		fail(c, http.StatusBadRequest, "当前只支持腾讯云免费证书和ACME证书提交")
		return
	}

	if needsCertificateOrder(row, account) {
		var domain model.Domain
		if row.DomainID == 0 {
			fail(c, http.StatusBadRequest, "证书未关联域名，无法创建腾讯云申请单")
			return
		}
		if err := h.db.First(&domain, row.DomainID).Error; err != nil {
			fail(c, http.StatusNotFound, "域名不存在")
			return
		}
		trace, err := h.startCertificateFlow(c.Request.Context(), &row, domain, &account)
		if err != nil {
			h.db.Model(&row).Updates(map[string]any{"status": certStatusFailed, "provider_status_msg": err.Error()})
			h.logCertificate(row, account, "apply", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, "创建腾讯云申请单失败："+err.Error())
			return
		}
		h.db.First(&row, row.ID)
		h.logCertificate(row, account, "apply", "success", "证书申请已创建，DNS验证记录已添加", trace)
	}

	if row.Status == certStatusIssued || row.Status == certStatusRevoked || row.Status == certStatusCanceled {
		_, _ = h.refreshCertificate(c.Request.Context(), &row, account)
		h.db.First(&row, row.ID)
		ok(c, row)
		return
	}

	if isACMEProvider(account.Provider) {
		trace, err := h.submitACMECertificate(c.Request.Context(), &row, &account)
		if err != nil {
			h.logCertificate(row, account, "submit", "failed", err.Error(), trace)
			fail(c, http.StatusBadRequest, "提交ACME验证失败："+err.Error())
			return
		}
		h.db.First(&row, row.ID)
		h.logCertificate(row, account, "submit", "success", "DNS验证记录已提交，等待CA验证", trace)
		h.startCertificateIssueFlow(row.ID, account.ID)
		ok(c, row)
		return
	}

	if row.AuthRecordID == "" && row.Status != certStatusSubmitted {
		if _, err := h.ensureTencentAuthRecord(c.Request.Context(), &row, account); err != nil {
			h.logCertificate(row, account, "apply", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, "补全DNS验证记录失败："+err.Error())
			return
		}
		h.db.First(&row, row.ID)
	}

	h.db.Model(&row).Updates(map[string]any{"status": certStatusSubmitted, "provider_status_msg": "DNS验证记录已添加，等待CA验证"})
	h.db.First(&row, row.ID)
	h.logCertificate(row, account, "submit", "success", "DNS验证记录已添加，等待CA验证", nil)
	h.startCertificateIssueFlow(row.ID, account.ID)
	ok(c, row)
}

func (h *CertificateHandler) Revoke(c *gin.Context) {
	row, account, found := h.certificateAccount(c)
	if !found {
		return
	}
	if !strings.EqualFold(account.Provider, "tencent_free") {
		fail(c, http.StatusBadRequest, "当前只支持腾讯云免费证书吊销")
		return
	}
	if strings.TrimSpace(row.ProviderCertificateID) == "" {
		fail(c, http.StatusBadRequest, "证书缺少腾讯云证书ID")
		return
	}
	ctx, err := h.contextWithSSLHTTPClient(c.Request.Context(), account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if row.Status == certStatusIssued {
		auths, trace, err := tencentRevokeCertificate(ctx, account, row.ProviderCertificateID, "revoke")
		if err != nil {
			h.logCertificate(row, account, "revoke", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.createTencentRevokeAuthRecords(ctx, &row, auths); err != nil {
			h.logCertificate(row, account, "revoke_verify", "failed", "添加吊销DNS验证记录失败："+err.Error(), trace)
			fail(c, http.StatusBadRequest, "添加吊销DNS验证记录失败："+err.Error())
			return
		}
		h.db.Model(&row).Updates(map[string]any{"status": certStatusRevoking, "provider_status_msg": "已提交吊销请求，等待域名验证"})
		h.logCertificate(row, account, "revoke", "success", "已提交吊销请求", trace)
		h.startTencentRevokeFlow(row.ID, account.ID)
	} else {
		trace, err := tencentSimpleCertificateAction(ctx, account, "CancelCertificateOrder", row.ProviderCertificateID)
		if err != nil {
			h.logCertificate(row, account, "revoke", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		h.db.Model(&row).Updates(map[string]any{"status": certStatusCanceled, "provider_status_msg": "已取消证书订单"})
		h.logCertificate(row, account, "revoke", "success", "已取消证书订单", trace)
	}
	h.db.First(&row, row.ID)
	ok(c, row)
}

func (h *CertificateHandler) Deploy(c *gin.Context) {
	row, _, found := h.certificateAccount(c)
	if !found {
		return
	}
	var req certificateDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	var account model.DeployAccount
	if err := h.db.First(&account, req.DeployAccountID).Error; err != nil {
		fail(c, http.StatusNotFound, "部署账号不存在")
		return
	}
	if account.Status == model.StatusDisabled {
		fail(c, http.StatusBadRequest, "部署账号已停用")
		return
	}
	if row.Status != certStatusIssued {
		fail(c, http.StatusBadRequest, "证书未签发，无法部署")
		return
	}
	certPEM := strings.TrimSpace(row.CertPEM)
	if chainPEM := strings.TrimSpace(row.ChainPEM); chainPEM != "" {
		certPEM = strings.TrimSpace(certPEM + "\n" + chainPEM)
	}
	if certPEM == "" || strings.TrimSpace(row.KeyPEM) == "" {
		fail(c, http.StatusBadRequest, "证书内容不完整，请先在详情中下载证书内容")
		return
	}
	targetName := firstNonEmpty(req.SiteName, row.CommonName)
	client, err := deployHTTPClient(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	trace, err := deployCertificate(c.Request.Context(), account, client, targetName, certPEM, row.KeyPEM)
	status := "success"
	message := "证书已部署到目标服务"
	if err != nil {
		status = "failed"
		message = err.Error()
	}
	h.logCertificateDeployment(row, account, targetName, status, message, trace)
	if err != nil {
		fail(c, http.StatusBadRequest, "部署证书失败："+err.Error())
		return
	}
	ok(c, gin.H{"deployed": true, "targetName": targetName})
}

func (h *CertificateHandler) Detail(c *gin.Context) {
	row, account, found := h.certificateAccount(c)
	if !found {
		return
	}
	if shouldRefreshCertificate(row, account) && isTruthyQuery(c.Query("refresh")) {
		trace, err := h.refreshCertificate(c.Request.Context(), &row, account)
		if err != nil {
			h.logCertificate(row, account, "verify", "failed", "刷新证书状态失败："+err.Error(), traceFromError(err))
		} else {
			h.logCertificate(row, account, "verify", "success", verifyLogMessage(account), trace)
		}
		h.db.First(&row, row.ID)
		if row.Status == certStatusIssued && strings.TrimSpace(row.CertPEM) == "" {
			if err := h.downloadCertificateContent(c.Request.Context(), &row, account); err != nil {
				h.logCertificate(row, account, "detail", "failed", "下载证书内容失败："+err.Error(), traceFromError(err))
			}
			h.db.First(&row, row.ID)
		}
	}
	ok(c, row)
}

func (h *CertificateHandler) startCertificateFlow(ctx context.Context, row *model.Certificate, domain model.Domain, account *model.SSLAccount) (*tencentRequestTrace, error) {
	var err error
	ctx, err = h.contextWithSSLHTTPClient(ctx, *account)
	if err != nil {
		return nil, err
	}
	if isACMEProvider(account.Provider) {
		return h.startACMECertificateFlow(ctx, row, domain, account)
	}
	if !strings.EqualFold(account.Provider, "tencent_free") {
		return nil, fmt.Errorf("当前只支持腾讯云免费证书自动申请")
	}
	if strings.TrimSpace(account.Email) == "" {
		return nil, fmt.Errorf("腾讯云SSL账号需要填写联系人邮箱")
	}

	certificateID, trace, err := tencentApplyCertificate(ctx, *account, row.CommonName)
	if err != nil {
		return trace, err
	}
	row.ProviderCertificateID = certificateID
	h.db.Model(row).Updates(map[string]any{
		"provider_certificate_id": certificateID,
		"verify_type":             "DNS",
		"status":                  certStatusApplying,
	})

	detail, detailTrace, err := h.waitTencentDVDetail(ctx, *account, certificateID)
	if detailTrace != nil {
		trace = detailTrace
	}
	if err != nil {
		return trace, err
	}
	if detail != nil {
		h.applyTencentDetail(row, detail)
	}
	if detail == nil || detail.DvAuthDetail == nil {
		return trace, fmt.Errorf("腾讯云未返回DNS验证信息")
	}
	authName, authValue := pickTencentAuthRecord(detail.DvAuthDetail)
	if authName == "" || authValue == "" {
		return trace, fmt.Errorf("腾讯云DNS验证信息不完整")
	}
	recordID, rr, err := h.createAuthRecord(ctx, domain, authName, authValue)
	if err != nil {
		return trace, err
	}
	status := certStatusDNSAdded
	if detail.Status != nil {
		status = tencentCertStatus(*detail.Status, status)
	}
	h.db.Model(row).Updates(map[string]any{
		"auth_record_id":      recordID,
		"auth_record_name":    rr,
		"auth_record_value":   authValue,
		"provider_status_msg": "DNS验证记录已添加",
		"status":              status,
	})
	return trace, nil
}

func (h *CertificateHandler) createAuthRecord(ctx context.Context, domain model.Domain, authName, authValue string) (string, string, error) {
	var domainAccount model.DomainAccount
	if err := h.db.First(&domainAccount, domain.DomainAccountID).Error; err != nil {
		return "", "", fmt.Errorf("域名账号不存在")
	}
	rr := normalizeAuthRR(authName, domain.Name)
	record := dnsservice.RecordInput{
		DomainName: domain.Name,
		RR:         rr,
		Type:       "TXT",
		Value:      authValue,
		Line:       "default",
		TTL:        600,
		Status:     "enabled",
		Remark:     "QDL SSL验证",
	}
	dnsAccount, err := dnsAccountWithProxy(h.db, domainAccount)
	if err != nil {
		return "", "", err
	}
	item, err := dnsservice.CreateRecord(ctx, dnsAccount, record)
	trace := dnsTrace(domainAccount, "CreateRecord", record, item, err)
	if err != nil {
		logDomainAccount(h.db, domainAccount, domain.ID, domain.Name, "create_record", "failed", "添加SSL验证记录失败："+err.Error(), trace)
		return "", "", fmt.Errorf("添加DNS验证记录失败：%w", err)
	}
	logDomainAccount(h.db, domainAccount, domain.ID, domain.Name, "create_record", "success", "已添加SSL验证记录", trace)
	return item.ID, rr, nil
}

type acmeAuthRecord struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Value            string `json:"value"`
	Domain           string `json:"domain"`
	AuthorizationURL string `json:"authorizationUrl"`
	ChallengeURL     string `json:"challengeUrl"`
}

func (h *CertificateHandler) startACMECertificateFlow(ctx context.Context, row *model.Certificate, domain model.Domain, account *model.SSLAccount) (*tencentRequestTrace, error) {
	if strings.TrimSpace(account.Email) == "" {
		return nil, fmt.Errorf("ACME账号需要填写注册邮箱")
	}
	client, trace, err := h.acmeClient(ctx, account)
	if err != nil {
		return trace, err
	}
	names := certificateNames(row, domain.Name)
	if len(names) == 0 {
		return trace, fmt.Errorf("证书域名不能为空")
	}
	_, keyPEM, err := generateRSAPrivateKeyPEM()
	if err != nil {
		return trace, err
	}
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(names...))
	trace = acmeTrace(account.Provider, "AuthorizeOrder", gin.H{"identifiers": names}, order, err)
	if err != nil {
		return trace, err
	}
	records := make([]acmeAuthRecord, 0, len(order.AuthzURLs))
	for _, authzURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authzURL)
		stepTrace := acmeTrace(account.Provider, "GetAuthorization", gin.H{"authorizationUrl": authzURL}, authz, err)
		if err != nil {
			return stepTrace, err
		}
		if authz.Status == acme.StatusValid {
			continue
		}
		challenge := pickDNS01Challenge(authz)
		if challenge == nil {
			return stepTrace, fmt.Errorf("ACME未返回DNS-01验证信息：%s", authz.Identifier.Value)
		}
		value, err := client.DNS01ChallengeRecord(challenge.Token)
		if err != nil {
			return stepTrace, err
		}
		authName := "_acme-challenge." + strings.TrimPrefix(authz.Identifier.Value, "*.")
		recordID, rr, err := h.createAuthRecord(ctx, domain, authName, value)
		if err != nil {
			return stepTrace, err
		}
		records = append(records, acmeAuthRecord{
			ID:               recordID,
			Name:             rr,
			Value:            value,
			Domain:           authz.Identifier.Value,
			AuthorizationURL: authzURL,
			ChallengeURL:     challenge.URI,
		})
	}
	rawRecords, _ := json.Marshal(records)
	updates := map[string]any{
		"key_pem":             keyPEM,
		"provider_order_id":   order.URI,
		"provider_status":     order.Status,
		"provider_status_msg": "DNS验证记录已添加",
		"verify_type":         "DNS",
		"auth_records":        string(rawRecords),
		"status":              certStatusDNSAdded,
	}
	if len(records) > 0 {
		updates["auth_record_id"] = records[0].ID
		updates["auth_record_name"] = records[0].Name
		updates["auth_record_value"] = records[0].Value
	}
	if len(records) == 0 {
		updates["provider_status_msg"] = "域名验证已有效，等待提交签发"
	}
	if err := h.db.Model(row).Updates(updates).Error; err != nil {
		return trace, err
	}
	return trace, nil
}

func (h *CertificateHandler) ensureTencentAuthRecord(ctx context.Context, row *model.Certificate, account model.SSLAccount) (*tencentRequestTrace, error) {
	var err error
	ctx, err = h.contextWithSSLHTTPClient(ctx, account)
	if err != nil {
		return nil, err
	}
	if row.DomainID == 0 {
		return nil, fmt.Errorf("证书未关联域名")
	}
	var domain model.Domain
	if err := h.db.First(&domain, row.DomainID).Error; err != nil {
		return nil, fmt.Errorf("域名不存在")
	}
	detail, trace, err := h.waitTencentDVDetail(ctx, account, row.ProviderCertificateID)
	if err != nil {
		return trace, err
	}
	if detail == nil || detail.DvAuthDetail == nil {
		return trace, fmt.Errorf("腾讯云未返回DNS验证信息")
	}
	h.applyTencentDetail(row, detail)
	authName, authValue := pickTencentAuthRecord(detail.DvAuthDetail)
	if authName == "" || authValue == "" {
		return trace, fmt.Errorf("腾讯云DNS验证信息不完整")
	}
	recordID, rr, err := h.createAuthRecord(ctx, domain, authName, authValue)
	if err != nil {
		return trace, err
	}
	status := certStatusDNSAdded
	if detail.Status != nil {
		status = tencentCertStatus(*detail.Status, status)
	}
	h.db.Model(row).Updates(map[string]any{
		"auth_record_id":      recordID,
		"auth_record_name":    rr,
		"auth_record_value":   authValue,
		"provider_status_msg": "DNS验证记录已添加",
		"status":              status,
	})
	return trace, nil
}

func (h *CertificateHandler) waitTencentDVDetail(ctx context.Context, account model.SSLAccount, certificateID string) (*tencentDetail, *tencentRequestTrace, error) {
	var lastErr error
	var lastTrace *tencentRequestTrace
	for i := 0; i < 3; i++ {
		detail, trace, err := tencentDescribeCertificate(ctx, account, certificateID)
		if trace != nil {
			lastTrace = trace
		}
		if err == nil && detail != nil && detail.DvAuthDetail != nil {
			return detail, trace, nil
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if lastErr != nil {
		return nil, lastTrace, lastErr
	}
	return tencentDescribeCertificate(ctx, account, certificateID)
}

func (h *CertificateHandler) startCertificateIssueFlow(rowID uint, accountID uint) {
	go func() {
		if err := h.runCertificateIssueFlow(context.Background(), rowID, accountID); err != nil {
			var row model.Certificate
			var account model.SSLAccount
			if h.db.First(&row, rowID).Error == nil && h.db.First(&account, accountID).Error == nil {
				h.logCertificate(row, account, "verify", "failed", "证书自动验证流程失败："+err.Error(), nil)
			}
		}
	}()
}

func (h *CertificateHandler) runCertificateIssueFlow(ctx context.Context, rowID uint, accountID uint) error {
	var row model.Certificate
	if err := h.db.First(&row, rowID).Error; err != nil {
		return err
	}
	var account model.SSLAccount
	if err := h.db.First(&account, accountID).Error; err != nil {
		return err
	}
	ctx, err := h.contextWithSSLHTTPClient(ctx, account)
	if err != nil {
		return err
	}
	if isACMEProvider(account.Provider) {
		return h.runACMEIssueFlow(ctx, &row, &account)
	}
	if strings.EqualFold(account.Provider, "tencent_free") {
		return h.runTencentIssueFlow(ctx, &row, account)
	}
	return fmt.Errorf("暂不支持该证书服务：%s", account.Provider)
}

func (h *CertificateHandler) runTencentIssueFlow(ctx context.Context, row *model.Certificate, account model.SSLAccount) error {
	if strings.TrimSpace(row.ProviderCertificateID) == "" {
		return fmt.Errorf("证书缺少腾讯云证书ID")
	}
	trace, err := tencentCompleteCertificate(ctx, account, row.ProviderCertificateID)
	if err != nil {
		h.logCertificate(*row, account, "verify", "failed", "主动触发域名验证失败："+err.Error(), traceFromError(err))
		return err
	}
	h.logCertificate(*row, account, "verify", "success", "已主动触发域名验证", trace)

	verified := false
	for i := 0; i < certFlowMaxChecks; i++ {
		if i > 0 {
			time.Sleep(certFlowInterval)
		}
		results, trace, err := tencentCheckCertificateDomainVerification(ctx, account, row.ProviderCertificateID)
		if err != nil {
			h.logCertificate(*row, account, "verify", "failed", "检查域名验证失败："+err.Error(), traceFromError(err))
			continue
		}
		if tencentVerificationPassed(results) {
			h.logCertificate(*row, account, "verify", "success", "域名验证已通过", trace)
			verified = true
			break
		}
		h.logCertificate(*row, account, "verify", "success", "域名验证暂未生效，继续等待", trace)
	}
	if !verified {
		_ = h.db.Model(row).Updates(map[string]any{"provider_status_msg": "域名验证暂未生效，请稍后查看日志"})
		return fmt.Errorf("域名验证暂未生效")
	}

	for i := 0; i < certFlowMaxChecks; i++ {
		if i > 0 {
			time.Sleep(certFlowInterval)
		}
		detail, trace, err := tencentDescribeCertificate(ctx, account, row.ProviderCertificateID)
		if err != nil {
			h.logCertificate(*row, account, "verify", "failed", "检查签发状态失败："+err.Error(), traceFromError(err))
			continue
		}
		if detail != nil {
			h.applyTencentDetail(row, detail)
			_ = h.db.First(row, row.ID).Error
		}
		h.logCertificate(*row, account, "verify", "success", "已检查签发状态", trace)
		switch row.Status {
		case certStatusIssued:
			if strings.TrimSpace(row.CertPEM) == "" {
				if err := h.downloadTencentCertificate(ctx, row, account); err != nil {
					h.logCertificate(*row, account, "detail", "failed", "下载证书内容失败："+err.Error(), traceFromError(err))
				}
				_ = h.db.First(row, row.ID).Error
			}
			cleaned, err := h.cleanupAuthRecord(ctx, row)
			if err != nil {
				h.logCertificate(*row, account, "cleanup", "failed", "删除DNS验证记录失败："+err.Error(), nil)
			} else if cleaned {
				h.logCertificate(*row, account, "cleanup", "success", "证书已签发，DNS验证记录已删除", nil)
			}
			return nil
		case certStatusFailed, certStatusExpired, certStatusCanceled, certStatusRevoked:
			return fmt.Errorf("证书签发进入终止状态：%s", row.Status)
		}
	}
	return fmt.Errorf("签发状态仍在处理中")
}

func (h *CertificateHandler) runACMEIssueFlow(ctx context.Context, row *model.Certificate, account *model.SSLAccount) error {
	client, trace, err := h.acmeClient(ctx, account)
	if err != nil {
		return err
	}
	submitTrace, err := h.submitACMECertificate(ctx, row, account)
	if submitTrace != nil {
		trace = submitTrace
	}
	if err != nil {
		return err
	}
	h.logCertificate(*row, *account, "submit", "success", "已提交ACME DNS验证，等待生效", trace)
	if err := h.waitACMEAuthorizations(ctx, row, account, client); err != nil {
		return err
	}
	for i := 0; i < certFlowMaxChecks; i++ {
		if i > 0 {
			time.Sleep(certFlowInterval)
		}
		order, err := client.GetOrder(ctx, row.ProviderOrderID)
		trace = acmeTrace(account.Provider, "GetOrder", gin.H{"orderUrl": row.ProviderOrderID}, order, err)
		if err != nil {
			h.logCertificate(*row, *account, "verify", "failed", "检查ACME签发状态失败："+err.Error(), trace)
			continue
		}
		h.logCertificate(*row, *account, "verify", "success", "已检查ACME签发状态", trace)
		if order.Status == acme.StatusReady {
			if err := h.finalizeACMECertificate(ctx, row, account, client, order); err != nil {
				return err
			}
			_ = h.db.First(row, row.ID).Error
		}
		if order.Status == acme.StatusValid || row.Status == certStatusIssued {
			certURL := firstNonEmpty(order.CertURL, row.ProviderCertificateID)
			if strings.TrimSpace(row.CertPEM) == "" && certURL != "" {
				if err := h.fetchACMECertificate(ctx, row, account, client, certURL); err != nil {
					return err
				}
				_ = h.db.First(row, row.ID).Error
			}
			cleaned, err := h.cleanupAuthRecord(ctx, row)
			if err != nil {
				h.logCertificate(*row, *account, "cleanup", "failed", "删除DNS验证记录失败："+err.Error(), nil)
			} else if cleaned {
				h.logCertificate(*row, *account, "cleanup", "success", "证书已签发，DNS验证记录已删除", nil)
			}
			return nil
		}
		if order.Status == acme.StatusInvalid {
			return fmt.Errorf("ACME订单无效")
		}
	}
	return fmt.Errorf("ACME签发状态仍在处理中")
}

func (h *CertificateHandler) waitACMEAuthorizations(ctx context.Context, row *model.Certificate, account *model.SSLAccount, client *acme.Client) error {
	records := parseACMEAuthRecords(row.AuthRecords)
	for i := 0; i < certFlowMaxChecks; i++ {
		if i > 0 {
			time.Sleep(certFlowInterval)
		}
		allValid := len(records) > 0
		for _, record := range records {
			authz, err := client.GetAuthorization(ctx, record.AuthorizationURL)
			trace := acmeTrace(account.Provider, "GetAuthorization", gin.H{"authorizationUrl": record.AuthorizationURL}, authz, err)
			if err != nil {
				h.logCertificate(*row, *account, "verify", "failed", "检查ACME域名验证失败："+err.Error(), trace)
				allValid = false
				continue
			}
			if authz.Status == acme.StatusInvalid {
				h.logCertificate(*row, *account, "verify", "failed", "ACME域名验证失败", trace)
				return fmt.Errorf("ACME域名验证失败：%s", record.Domain)
			}
			if authz.Status != acme.StatusValid {
				allValid = false
			}
			h.logCertificate(*row, *account, "verify", "success", "已检查ACME域名验证状态", trace)
		}
		if allValid {
			_ = h.db.Model(row).Updates(map[string]any{"provider_status_msg": "ACME域名验证已生效"}).Error
			return nil
		}
	}
	return fmt.Errorf("ACME域名验证暂未生效")
}

func (h *CertificateHandler) startTencentRevokeFlow(rowID uint, accountID uint) {
	go func() {
		if err := h.runTencentRevokeFlow(context.Background(), rowID, accountID); err != nil {
			var row model.Certificate
			var account model.SSLAccount
			if h.db.First(&row, rowID).Error == nil && h.db.First(&account, accountID).Error == nil {
				h.logCertificate(row, account, "revoke", "failed", "吊销自动验证流程失败："+err.Error(), nil)
			}
		}
	}()
}

func (h *CertificateHandler) runTencentRevokeFlow(ctx context.Context, rowID uint, accountID uint) error {
	var row model.Certificate
	if err := h.db.First(&row, rowID).Error; err != nil {
		return err
	}
	var account model.SSLAccount
	if err := h.db.First(&account, accountID).Error; err != nil {
		return err
	}
	ctx, err := h.contextWithSSLHTTPClient(ctx, account)
	if err != nil {
		return err
	}
	if strings.TrimSpace(row.ProviderCertificateID) == "" {
		return fmt.Errorf("证书缺少腾讯云证书ID")
	}
	h.db.First(&row, row.ID)
	trace, err := tencentCompleteCertificate(ctx, account, row.ProviderCertificateID)
	if err != nil {
		h.logCertificate(row, account, "revoke_verify", "failed", "主动触发吊销域名验证失败："+err.Error(), traceFromError(err))
		return err
	}
	h.logCertificate(row, account, "revoke_verify", "success", "已主动触发吊销域名验证", trace)

	verified := false
	for i := 0; i < certFlowMaxChecks; i++ {
		if i > 0 {
			time.Sleep(certFlowInterval)
		}
		results, trace, err := tencentCheckCertificateDomainVerification(ctx, account, row.ProviderCertificateID)
		if err != nil {
			h.logCertificate(row, account, "revoke_check", "failed", "检查吊销域名验证失败："+err.Error(), traceFromError(err))
			continue
		}
		if tencentVerificationPassed(results) {
			h.logCertificate(row, account, "revoke_check", "success", "吊销域名验证已通过", trace)
			verified = true
			break
		}
		h.logCertificate(row, account, "revoke_check", "success", "吊销域名验证暂未通过", trace)
	}
	if !verified {
		_ = h.db.Model(&row).Updates(map[string]any{"provider_status_msg": "吊销域名验证暂未通过，请稍后查看日志"}).Error
		return fmt.Errorf("吊销域名验证暂未通过")
	}

	for i := 0; i < certFlowMaxChecks; i++ {
		if i > 0 {
			time.Sleep(certFlowInterval)
		}
		detail, trace, err := tencentDescribeCertificate(ctx, account, row.ProviderCertificateID)
		if err != nil {
			h.logCertificate(row, account, "revoke_status", "failed", "检查吊销状态失败："+err.Error(), traceFromError(err))
			continue
		}
		if detail != nil {
			h.applyTencentDetail(&row, detail)
			_ = h.db.First(&row, row.ID).Error
		}
		h.logCertificate(row, account, "revoke_status", "success", "已检查吊销状态", trace)
		if row.Status == certStatusRevoked {
			cleaned, err := h.cleanupAuthRecordWithMessage(ctx, &row, "certificate_revoke_cleanup", "证书吊销完成，已删除SSL验证记录", "证书已吊销，DNS验证记录已删除")
			if err != nil {
				h.logCertificate(row, account, "cleanup", "failed", "删除吊销DNS验证记录失败："+err.Error(), nil)
			} else if cleaned {
				h.logCertificate(row, account, "cleanup", "success", "证书已吊销，DNS验证记录已删除", nil)
			}
			return nil
		}
	}
	return fmt.Errorf("吊销状态仍在处理中")
}

func (h *CertificateHandler) createTencentRevokeAuthRecords(ctx context.Context, row *model.Certificate, auths []tencentRevokeAuth) error {
	if len(auths) == 0 {
		return fmt.Errorf("腾讯云未返回吊销DNS验证信息")
	}
	if row.DomainID == 0 {
		return fmt.Errorf("证书未关联域名")
	}
	var domain model.Domain
	if err := h.db.First(&domain, row.DomainID).Error; err != nil {
		return fmt.Errorf("域名不存在")
	}
	records := make([]acmeAuthRecord, 0, len(auths))
	for _, auth := range auths {
		authName := tencentRevokeAuthRecordName(auth)
		authValue := strings.TrimSpace(auth.Value)
		if authName == "" || authValue == "" {
			return fmt.Errorf("腾讯云吊销DNS验证信息不完整")
		}
		recordID, rr, err := h.createAuthRecord(ctx, domain, authName, authValue)
		if err != nil {
			return err
		}
		records = append(records, acmeAuthRecord{
			ID:     recordID,
			Name:   rr,
			Value:  authValue,
			Domain: firstNonEmpty(auth.Domain, row.CommonName),
		})
	}
	rawRecords, _ := json.Marshal(records)
	updates := map[string]any{
		"auth_records":        string(rawRecords),
		"provider_status_msg": "吊销DNS验证记录已添加",
		"status":              certStatusRevoking,
	}
	if len(records) > 0 {
		updates["auth_record_id"] = records[0].ID
		updates["auth_record_name"] = records[0].Name
		updates["auth_record_value"] = records[0].Value
	}
	h.db.Model(row).Updates(map[string]any{
		"auth_record_id":      updates["auth_record_id"],
		"auth_record_name":    updates["auth_record_name"],
		"auth_record_value":   updates["auth_record_value"],
		"auth_records":        updates["auth_records"],
		"provider_status_msg": updates["provider_status_msg"],
		"status":              updates["status"],
	})
	return nil
}

func (h *CertificateHandler) submitACMECertificate(ctx context.Context, row *model.Certificate, account *model.SSLAccount) (*tencentRequestTrace, error) {
	var proxyErr error
	ctx, proxyErr = h.contextWithSSLHTTPClient(ctx, *account)
	if proxyErr != nil {
		return nil, proxyErr
	}
	client, trace, err := h.acmeClient(ctx, account)
	if err != nil {
		return trace, err
	}
	records := parseACMEAuthRecords(row.AuthRecords)
	if len(records) == 0 && strings.TrimSpace(row.AuthRecordID) != "" {
		records = append(records, acmeAuthRecord{
			ID:     row.AuthRecordID,
			Name:   row.AuthRecordName,
			Value:  row.AuthRecordValue,
			Domain: row.CommonName,
		})
	}
	for _, record := range records {
		if strings.TrimSpace(record.ChallengeURL) == "" {
			continue
		}
		challenge, err := client.GetChallenge(ctx, record.ChallengeURL)
		trace = acmeTrace(account.Provider, "GetChallenge", gin.H{"challengeUrl": record.ChallengeURL}, challenge, err)
		if err != nil {
			return trace, err
		}
		if challenge.Status == acme.StatusValid {
			continue
		}
		if challenge.Status == acme.StatusPending {
			accepted, err := client.Accept(ctx, challenge)
			trace = acmeTrace(account.Provider, "AcceptChallenge", gin.H{"challengeUrl": record.ChallengeURL}, accepted, err)
			if err != nil {
				return trace, err
			}
		}
	}
	if err := h.db.Model(row).Updates(map[string]any{
		"status":              certStatusSubmitted,
		"provider_status_msg": "DNS验证记录已提交，等待CA验证",
	}).Error; err != nil {
		return trace, err
	}
	return trace, nil
}

func (h *CertificateHandler) refreshCertificate(ctx context.Context, row *model.Certificate, account model.SSLAccount) (*tencentRequestTrace, error) {
	var err error
	ctx, err = h.contextWithSSLHTTPClient(ctx, account)
	if err != nil {
		return nil, err
	}
	if isACMEProvider(account.Provider) {
		return h.refreshACMECertificate(ctx, row, &account)
	}
	return h.refreshTencentCertificate(ctx, row, account)
}

func (h *CertificateHandler) refreshACMECertificate(ctx context.Context, row *model.Certificate, account *model.SSLAccount) (*tencentRequestTrace, error) {
	client, trace, err := h.acmeClient(ctx, account)
	if err != nil {
		return trace, err
	}
	if strings.TrimSpace(row.ProviderOrderID) == "" {
		return trace, fmt.Errorf("证书缺少ACME订单URL")
	}
	order, err := client.GetOrder(ctx, row.ProviderOrderID)
	trace = acmeTrace(account.Provider, "GetOrder", gin.H{"orderUrl": row.ProviderOrderID}, order, err)
	if err != nil {
		return trace, err
	}
	if order.Status == acme.StatusPending && hasACMEChallengeRecords(row) {
		submitTrace, err := h.submitACMECertificate(ctx, row, account)
		if submitTrace != nil {
			trace = submitTrace
		}
		if err != nil {
			return trace, err
		}
		h.logCertificate(*row, *account, "submit", "success", "已提交ACME DNS验证", trace)
		order, err = client.GetOrder(ctx, row.ProviderOrderID)
		trace = acmeTrace(account.Provider, "GetOrder", gin.H{"orderUrl": row.ProviderOrderID}, order, err)
		if err != nil {
			return trace, err
		}
	}
	localIssued := isLocalCertificateIssued(*row)
	status := acmeOrderStatus(order.Status, row.Status)
	statusMsg := "ACME订单状态：" + order.Status
	if localIssued {
		status = certStatusIssued
		statusMsg = "证书已签发，本地证书内容已保存"
	}
	updates := map[string]any{
		"provider_status":     order.Status,
		"provider_status_msg": statusMsg,
		"status":              status,
	}
	if order.CertURL != "" {
		updates["provider_certificate_id"] = order.CertURL
	}
	if err := h.db.Model(row).Updates(updates).Error; err != nil {
		return trace, err
	}
	h.db.First(row, row.ID)
	if order.Status == acme.StatusReady {
		if err := h.finalizeACMECertificate(ctx, row, account, client, order); err != nil {
			return trace, err
		}
		h.db.First(row, row.ID)
	}
	if order.Status == acme.StatusValid || row.Status == certStatusIssued {
		certURL := firstNonEmpty(order.CertURL, row.ProviderCertificateID)
		if strings.TrimSpace(row.CertPEM) == "" && certURL != "" {
			if err := h.fetchACMECertificate(ctx, row, account, client, certURL); err != nil {
				return trace, err
			}
			h.db.First(row, row.ID)
		}
		if row.Status == certStatusIssued {
			cleaned, err := h.cleanupAuthRecord(ctx, row)
			if err != nil {
				h.logCertificate(*row, *account, "cleanup", "failed", "删除DNS验证记录失败："+err.Error(), nil)
			} else if cleaned {
				h.logCertificate(*row, *account, "cleanup", "success", "证书已签发，DNS验证记录已删除", nil)
			}
		}
	}
	return trace, nil
}

func (h *CertificateHandler) finalizeACMECertificate(ctx context.Context, row *model.Certificate, account *model.SSLAccount, client *acme.Client, order *acme.Order) error {
	key, err := parseRSAPrivateKeyPEM(row.KeyPEM)
	if err != nil {
		return err
	}
	names := certificateNames(row, row.CommonName)
	csrDER, err := createCSRDER(key, names)
	if err != nil {
		return err
	}
	chain, certURL, err := client.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	trace := acmeTrace(account.Provider, "CreateOrderCert", gin.H{"finalizeUrl": order.FinalizeURL}, gin.H{"certUrl": certURL, "chainLength": len(chain)}, err)
	if err != nil {
		h.logCertificate(*row, *account, "verify", "failed", "ACME证书签发失败："+err.Error(), trace)
		return err
	}
	if err := h.saveACMECertificateChain(row, certURL, chain); err != nil {
		return err
	}
	h.logCertificate(*row, *account, "verify", "success", "ACME证书已签发并保存到本地", trace)
	return nil
}

func (h *CertificateHandler) fetchACMECertificate(ctx context.Context, row *model.Certificate, account *model.SSLAccount, client *acme.Client, certURL string) error {
	chain, err := client.FetchCert(ctx, certURL, true)
	trace := acmeTrace(account.Provider, "FetchCert", gin.H{"certUrl": certURL}, gin.H{"chainLength": len(chain)}, err)
	if err != nil {
		h.logCertificate(*row, *account, "detail", "failed", "拉取ACME证书失败："+err.Error(), trace)
		return err
	}
	if err := h.saveACMECertificateChain(row, certURL, chain); err != nil {
		return err
	}
	h.logCertificate(*row, *account, "detail", "success", "已拉取ACME证书内容到本地", trace)
	return nil
}

func (h *CertificateHandler) downloadCertificateContent(ctx context.Context, row *model.Certificate, account model.SSLAccount) error {
	var err error
	ctx, err = h.contextWithSSLHTTPClient(ctx, account)
	if err != nil {
		return err
	}
	if isACMEProvider(account.Provider) {
		certURL := strings.TrimSpace(row.ProviderCertificateID)
		if certURL == "" {
			return fmt.Errorf("证书缺少ACME证书URL")
		}
		client, _, err := h.acmeClient(ctx, &account)
		if err != nil {
			return err
		}
		return h.fetchACMECertificate(ctx, row, &account, client, certURL)
	}
	return h.downloadTencentCertificate(ctx, row, account)
}

func (h *CertificateHandler) saveACMECertificateChain(row *model.Certificate, certURL string, chain [][]byte) error {
	if len(chain) == 0 {
		return fmt.Errorf("ACME未返回证书内容")
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: chain[0]}))
	chainPEMs := make([]string, 0, len(chain)-1)
	for _, der := range chain[1:] {
		chainPEMs = append(chainPEMs, strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))))
	}
	updates := map[string]any{
		"provider_certificate_id": certURL,
		"provider_status":         acme.StatusValid,
		"provider_status_msg":     "证书已签发",
		"cert_pem":                strings.TrimSpace(certPEM),
		"chain_pem":               strings.TrimSpace(strings.Join(chainPEMs, "\n")),
		"status":                  certStatusIssued,
	}
	if cert, err := x509.ParseCertificate(chain[0]); err == nil {
		updates["issuer"] = cert.Issuer.CommonName
		updates["serial_number"] = cert.SerialNumber.String()
		updates["expires_at"] = cert.NotAfter
	}
	return h.db.Model(row).Updates(updates).Error
}

func (h *CertificateHandler) refreshTencentCertificate(ctx context.Context, row *model.Certificate, account model.SSLAccount) (*tencentRequestTrace, error) {
	detail, trace, err := tencentDescribeCertificate(ctx, account, row.ProviderCertificateID)
	if err != nil || detail == nil {
		if err != nil {
			return trace, err
		}
		return trace, fmt.Errorf("腾讯云未返回证书详情")
	}
	h.applyTencentDetail(row, detail)
	h.db.First(row, row.ID)
	if row.Status == certStatusIssued {
		cleaned, err := h.cleanupAuthRecord(ctx, row)
		if err != nil {
			h.logCertificate(*row, account, "cleanup", "failed", "删除DNS验证记录失败："+err.Error(), nil)
		} else if cleaned {
			h.logCertificate(*row, account, "cleanup", "success", "证书已签发，DNS验证记录已删除", nil)
		}
	}
	return trace, nil
}

func (h *CertificateHandler) cleanupAuthRecord(ctx context.Context, row *model.Certificate) (bool, error) {
	return h.cleanupAuthRecordWithMessage(ctx, row, "certificate_issued_cleanup", "证书签发成功，已删除SSL验证记录", "证书已签发，DNS验证记录已删除")
}

func (h *CertificateHandler) cleanupAuthRecordWithMessage(ctx context.Context, row *model.Certificate, reason, domainLogMessage, statusMessage string) (bool, error) {
	records := parseACMEAuthRecords(row.AuthRecords)
	if len(records) == 0 && strings.TrimSpace(row.AuthRecordID) != "" {
		records = append(records, acmeAuthRecord{
			ID:     row.AuthRecordID,
			Name:   row.AuthRecordName,
			Value:  row.AuthRecordValue,
			Domain: row.CommonName,
		})
	}
	if len(records) == 0 {
		return false, nil
	}
	if row.DomainID == 0 {
		return false, fmt.Errorf("证书未关联域名，无法删除DNS验证记录")
	}
	var domain model.Domain
	if err := h.db.First(&domain, row.DomainID).Error; err != nil {
		return false, fmt.Errorf("域名不存在")
	}
	var domainAccount model.DomainAccount
	if err := h.db.First(&domainAccount, domain.DomainAccountID).Error; err != nil {
		return false, fmt.Errorf("域名账号不存在")
	}
	deleted := 0
	for _, record := range records {
		recordID := strings.TrimSpace(record.ID)
		if recordID == "" {
			continue
		}
		request := gin.H{
			"domainName": domain.Name,
			"recordId":   recordID,
			"rr":         record.Name,
			"type":       "TXT",
			"reason":     reason,
		}
		dnsAccount, proxyErr := dnsAccountWithProxy(h.db, domainAccount)
		var err error
		if proxyErr != nil {
			err = proxyErr
		} else {
			err = dnsservice.DeleteRecord(ctx, dnsAccount, domain.Name, recordID)
		}
		trace := dnsTrace(domainAccount, "DeleteRecord", request, gin.H{"deleted": err == nil}, err)
		if err != nil {
			logDomainAccount(h.db, domainAccount, domain.ID, domain.Name, "delete_record", "failed", "删除SSL验证记录失败："+err.Error(), trace)
			return deleted > 0, err
		}
		logDomainAccount(h.db, domainAccount, domain.ID, domain.Name, "delete_record", "success", domainLogMessage, trace)
		_ = h.db.Where("domain_id = ? AND provider_record_id = ?", domain.ID, recordID).Delete(&model.DomainRecord{}).Error
		deleted++
	}
	if deleted == 0 {
		return false, nil
	}
	updates := map[string]any{
		"auth_record_id":      "",
		"auth_records":        "",
		"provider_status_msg": statusMessage,
	}
	if err := h.db.Model(row).Updates(updates).Error; err != nil {
		return false, err
	}
	row.AuthRecordID = ""
	row.AuthRecords = ""
	row.ProviderStatusMsg = updates["provider_status_msg"].(string)
	return true, nil
}

func (h *CertificateHandler) downloadTencentCertificate(ctx context.Context, row *model.Certificate, account model.SSLAccount) error {
	bundle, trace, err := tencentDownloadCertificate(ctx, account, row.ProviderCertificateID)
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if strings.TrimSpace(bundle.CertPEM) != "" {
		updates["cert_pem"] = bundle.CertPEM
	}
	if strings.TrimSpace(bundle.KeyPEM) != "" {
		updates["key_pem"] = bundle.KeyPEM
	}
	if strings.TrimSpace(bundle.ChainPEM) != "" {
		updates["chain_pem"] = bundle.ChainPEM
	}
	if len(updates) == 0 {
		return fmt.Errorf("腾讯云下载包中未识别到证书内容")
	}
	if err := h.db.Model(row).Updates(updates).Error; err != nil {
		return err
	}
	h.logCertificate(*row, account, "detail", "success", "已下载证书内容到本地", trace)
	return nil
}

func (h *CertificateHandler) applyTencentDetail(row *model.Certificate, detail *tencentDetail) {
	updates := map[string]any{}
	if detail.CertificateID != "" {
		updates["provider_certificate_id"] = detail.CertificateID
	}
	if detail.OrderID != "" {
		updates["provider_order_id"] = detail.OrderID
	}
	if detail.Issuer != "" {
		updates["issuer"] = detail.Issuer
	}
	if detail.Status != nil {
		updates["provider_status"] = strconv.FormatUint(*detail.Status, 10)
		updates["status"] = tencentCertStatus(*detail.Status, row.Status)
	}
	if detail.StatusMsg != "" {
		updates["provider_status_msg"] = detail.StatusMsg
	}
	if detail.VerifyType != "" {
		updates["verify_type"] = detail.VerifyType
	}
	if detail.CertEndTime != "" {
		if expiresAt := parseTencentTime(detail.CertEndTime); expiresAt != nil {
			updates["expires_at"] = expiresAt
		}
	}
	if detail.DvAuthDetail != nil {
		name, value := pickTencentAuthRecord(detail.DvAuthDetail)
		if name != "" {
			updates["auth_record_name"] = name
		}
		if value != "" {
			updates["auth_record_value"] = value
		}
	}
	if len(updates) > 0 {
		h.db.Model(row).Updates(updates)
	}
}

func (h *CertificateHandler) certificateAccount(c *gin.Context) (model.Certificate, model.SSLAccount, bool) {
	var row model.Certificate
	if err := h.db.First(&row, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "证书不存在")
		return row, model.SSLAccount{}, false
	}
	var account model.SSLAccount
	if err := h.db.First(&account, row.SSLAccountID).Error; err != nil {
		fail(c, http.StatusNotFound, "SSL账号不存在")
		return row, account, false
	}
	return row, account, true
}

func (h *CertificateHandler) validateRequest(c *gin.Context, req certificateRequest) (model.Domain, model.SSLAccount, bool) {
	var domain model.Domain
	if err := h.db.First(&domain, req.DomainID).Error; err != nil {
		fail(c, http.StatusNotFound, "域名不存在")
		return domain, model.SSLAccount{}, false
	}
	var account model.SSLAccount
	if err := h.db.First(&account, req.SSLAccountID).Error; err != nil {
		fail(c, http.StatusNotFound, "SSL账号不存在")
		return domain, account, false
	}
	if account.Status == model.StatusDisabled {
		fail(c, http.StatusBadRequest, "SSL账号已停用")
		return domain, account, false
	}
	if strings.TrimSpace(account.Provider) == "" {
		fail(c, http.StatusBadRequest, "SSL账号缺少证书服务")
		return domain, account, false
	}
	return domain, account, true
}

func (h *CertificateHandler) logCertificate(row model.Certificate, account model.SSLAccount, action, status, message string, trace *tencentRequestTrace) {
	log := model.CertificateLog{
		CertificateID:         row.ID,
		ProviderCertificateID: row.ProviderCertificateID,
		CommonName:            row.CommonName,
		Action:                action,
		Provider:              account.Provider,
		Status:                status,
		Message:               message,
	}
	if trace != nil {
		log.RequestURL = trace.RequestURL
		log.RequestMethod = trace.RequestMethod
		log.RequestHeaders = trace.RequestHeaders
		log.RequestBody = trace.RequestBody
		log.ResponseBody = trace.ResponseBody
	}
	_ = h.db.Create(&log).Error
}

func (h *CertificateHandler) logCertificateDeployment(row model.Certificate, account model.DeployAccount, targetName, status, message string, trace *deployRequestTrace) {
	log := model.CertificateDeployLog{
		CertificateID:   row.ID,
		DeployAccountID: account.ID,
		CommonName:      row.CommonName,
		Provider:        account.Provider,
		TargetName:      targetName,
		Status:          status,
		Message:         message,
	}
	if trace != nil {
		log.RequestURL = trace.RequestURL
		log.RequestMethod = trace.RequestMethod
		log.RequestHeaders = trace.RequestHeaders
		log.RequestBody = trace.RequestBody
		log.ResponseBody = trace.ResponseBody
	}
	_ = h.db.Create(&log).Error
}

type tencentDetail struct {
	CertificateID string
	OrderID       string
	Issuer        string
	Status        *uint64
	StatusMsg     string
	VerifyType    string
	CertEndTime   string
	DvAuthDetail  *tencentDVAuthDetail
}

type tencentDVAuthDetail struct {
	DvAuthKey          string              `json:"DvAuthKey"`
	DvAuthValue        string              `json:"DvAuthValue"`
	DvAuthDomain       string              `json:"DvAuthDomain"`
	DvAuthPath         string              `json:"DvAuthPath"`
	DvAuthKeySubDomain string              `json:"DvAuthKeySubDomain"`
	DvAuths            []tencentDVAuthItem `json:"DvAuths"`
}

type tencentDVAuthItem struct {
	DvAuthKey        string `json:"DvAuthKey"`
	DvAuthValue      string `json:"DvAuthValue"`
	DvAuthDomain     string `json:"DvAuthDomain"`
	DvAuthPath       string `json:"DvAuthPath"`
	DvAuthSubDomain  string `json:"DvAuthSubDomain"`
	DvAuthVerifyType string `json:"DvAuthVerifyType"`
}

type tencentRevokeAuth struct {
	Domain string `json:"domain"`
	Key    string `json:"key"`
	Value  string `json:"value"`
	Path   string `json:"path"`
}

type tencentDomainVerificationResult struct {
	Domain       string `json:"domain"`
	VerifyType   string `json:"verifyType"`
	LocalCheck   *int   `json:"localCheck"`
	CACheck      *int   `json:"caCheck"`
	CheckValue   any    `json:"checkValue"`
	FailReason   string `json:"failReason"`
	Issued       *bool  `json:"issued"`
	NeedVerify   *bool  `json:"needVerify"`
	RecordName   string `json:"recordName"`
	RecordValue  string `json:"recordValue"`
	ResourceType string `json:"resourceType"`
}

type tencentError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type tencentRequestTrace struct {
	RequestURL     string
	RequestMethod  string
	RequestHeaders string
	RequestBody    string
	ResponseBody   string
}

type tencentCertificateBundle struct {
	CertPEM  string
	KeyPEM   string
	ChainPEM string
}

type deployRequestTrace struct {
	RequestURL     string
	RequestMethod  string
	RequestHeaders string
	RequestBody    string
	ResponseBody   string
}

type sslHTTPClientContextKey struct{}

type tencentTracedError struct {
	err   error
	trace *tencentRequestTrace
}

func (e *tencentTracedError) Error() string {
	return e.err.Error()
}

func (e *tencentTracedError) Unwrap() error {
	return e.err
}

func traceFromError(err error) *tencentRequestTrace {
	var traced *tencentTracedError
	if errors.As(err, &traced) {
		return traced.trace
	}
	return nil
}

func withTencentTrace(err error, trace *tencentRequestTrace) error {
	if err == nil {
		return nil
	}
	return &tencentTracedError{err: err, trace: trace}
}

func (h *CertificateHandler) contextWithSSLHTTPClient(ctx context.Context, account model.SSLAccount) (context.Context, error) {
	client, err := sslHTTPClient(h.db, account, 30*time.Second)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, sslHTTPClientContextKey{}, client), nil
}

func tencentApplyCertificate(ctx context.Context, account model.SSLAccount, domain string) (string, *tencentRequestTrace, error) {
	var out struct {
		Response struct {
			Error         *tencentError `json:"Error"`
			CertificateID string        `json:"CertificateId"`
			RequestID     string        `json:"RequestId"`
		} `json:"Response"`
	}
	payload := map[string]any{
		"DvAuthMethod":   "DNS",
		"DomainName":     domain,
		"PackageType":    "83",
		"ContactEmail":   account.Email,
		"ValidityPeriod": "3",
		"Alias":          "QDL-" + domain,
	}
	trace, err := tencentSSLRequest(ctx, account, "ApplyCertificate", payload, &out)
	if err != nil {
		return "", trace, err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return "", trace, err
	}
	if out.Response.CertificateID == "" {
		return "", trace, fmt.Errorf("腾讯云未返回证书ID")
	}
	return out.Response.CertificateID, trace, nil
}

func tencentSimpleCertificateAction(ctx context.Context, account model.SSLAccount, action, certificateID string) (*tencentRequestTrace, error) {
	var out struct {
		Response struct {
			Error     *tencentError `json:"Error"`
			RequestID string        `json:"RequestId"`
		} `json:"Response"`
	}
	trace, err := tencentSSLRequest(ctx, account, action, map[string]any{"CertificateId": certificateID}, &out)
	if err != nil {
		return trace, err
	}
	return trace, tencentResponseErr(out.Response.Error, trace)
}

func tencentRevokeCertificate(ctx context.Context, account model.SSLAccount, certificateID, reason string) ([]tencentRevokeAuth, *tencentRequestTrace, error) {
	var out struct {
		Response struct {
			Error                     *tencentError `json:"Error"`
			RequestID                 string        `json:"RequestId"`
			RevokeDomainValidateAuths []struct {
				DomainValidateAuthDomain string `json:"DomainValidateAuthDomain"`
				DomainValidateAuthKey    string `json:"DomainValidateAuthKey"`
				DomainValidateAuthValue  string `json:"DomainValidateAuthValue"`
				DomainValidateAuthPath   string `json:"DomainValidateAuthPath"`
			} `json:"RevokeDomainValidateAuths"`
		} `json:"Response"`
	}
	payload := map[string]any{
		"CertificateId": certificateID,
	}
	if strings.TrimSpace(reason) != "" {
		payload["Reason"] = strings.TrimSpace(reason)
	}
	trace, err := tencentSSLRequest(ctx, account, "RevokeCertificate", payload, &out)
	if err != nil {
		return nil, trace, err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return nil, trace, err
	}
	auths := make([]tencentRevokeAuth, 0, len(out.Response.RevokeDomainValidateAuths))
	for _, item := range out.Response.RevokeDomainValidateAuths {
		auths = append(auths, tencentRevokeAuth{
			Domain: item.DomainValidateAuthDomain,
			Key:    item.DomainValidateAuthKey,
			Value:  item.DomainValidateAuthValue,
			Path:   item.DomainValidateAuthPath,
		})
	}
	return auths, trace, nil
}

func tencentCompleteCertificate(ctx context.Context, account model.SSLAccount, certificateID string) (*tencentRequestTrace, error) {
	var out struct {
		Response struct {
			Error     *tencentError `json:"Error"`
			RequestID string        `json:"RequestId"`
		} `json:"Response"`
	}
	trace, err := tencentSSLRequest(ctx, account, "CompleteCertificate", map[string]any{"CertificateId": certificateID}, &out)
	if err != nil {
		return trace, err
	}
	return trace, tencentResponseErr(out.Response.Error, trace)
}

func tencentCheckCertificateDomainVerification(ctx context.Context, account model.SSLAccount, certificateID string) ([]tencentDomainVerificationResult, *tencentRequestTrace, error) {
	var out struct {
		Response struct {
			Error        *tencentError `json:"Error"`
			RequestID    string        `json:"RequestId"`
			Verification []struct {
				Domain       string `json:"Domain"`
				VerifyType   string `json:"VerifyType"`
				LocalCheck   *int   `json:"LocalCheck"`
				CACheck      *int   `json:"CaCheck"`
				CheckValue   any    `json:"CheckValue"`
				FailReason   string `json:"FailReason"`
				Issued       *bool  `json:"Issued"`
				NeedVerify   *bool  `json:"NeedVerify"`
				RecordName   string `json:"RecordName"`
				RecordValue  string `json:"RecordValue"`
				ResourceType string `json:"ResourceType"`
			} `json:"VerificationResults"`
		} `json:"Response"`
	}
	trace, err := tencentSSLRequest(ctx, account, "CheckCertificateDomainVerification", map[string]any{"CertificateId": certificateID}, &out)
	if err != nil {
		return nil, trace, err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return nil, trace, err
	}
	results := make([]tencentDomainVerificationResult, 0, len(out.Response.Verification))
	for _, item := range out.Response.Verification {
		results = append(results, tencentDomainVerificationResult{
			Domain:       item.Domain,
			VerifyType:   item.VerifyType,
			LocalCheck:   item.LocalCheck,
			CACheck:      item.CACheck,
			CheckValue:   item.CheckValue,
			FailReason:   item.FailReason,
			Issued:       item.Issued,
			NeedVerify:   item.NeedVerify,
			RecordName:   item.RecordName,
			RecordValue:  item.RecordValue,
			ResourceType: item.ResourceType,
		})
	}
	return results, trace, nil
}

func tencentDescribeCertificate(ctx context.Context, account model.SSLAccount, certificateID string) (*tencentDetail, *tencentRequestTrace, error) {
	var out struct {
		Response struct {
			Error             *tencentError        `json:"Error"`
			CertificateID     string               `json:"CertificateId"`
			OrderID           string               `json:"OrderId"`
			ProductZhName     string               `json:"ProductZhName"`
			Status            *uint64              `json:"Status"`
			StatusMsg         string               `json:"StatusMsg"`
			VerifyType        string               `json:"VerifyType"`
			CertEndTime       string               `json:"CertEndTime"`
			DvAuthDetail      *tencentDVAuthDetail `json:"DvAuthDetail"`
			CertificatePublic string               `json:"CertificatePublicKey"`
			RequestID         string               `json:"RequestId"`
		} `json:"Response"`
	}
	trace, err := tencentSSLRequest(ctx, account, "DescribeCertificate", map[string]any{"CertificateId": certificateID}, &out)
	if err != nil {
		return nil, trace, err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return nil, trace, err
	}
	return &tencentDetail{
		CertificateID: out.Response.CertificateID,
		OrderID:       out.Response.OrderID,
		Issuer:        out.Response.ProductZhName,
		Status:        out.Response.Status,
		StatusMsg:     out.Response.StatusMsg,
		VerifyType:    out.Response.VerifyType,
		CertEndTime:   out.Response.CertEndTime,
		DvAuthDetail:  out.Response.DvAuthDetail,
	}, trace, nil
}

func tencentDownloadCertificate(ctx context.Context, account model.SSLAccount, certificateID string) (*tencentCertificateBundle, *tencentRequestTrace, error) {
	var out struct {
		Response struct {
			Error           *tencentError `json:"Error"`
			Content         string        `json:"Content"`
			CertificateInfo string        `json:"CertificateInfo"`
			RequestID       string        `json:"RequestId"`
		} `json:"Response"`
	}
	trace, err := tencentSSLRequest(ctx, account, "DownloadCertificate", map[string]any{"CertificateId": certificateID}, &out)
	if err != nil {
		return nil, trace, err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return nil, trace, err
	}
	content := strings.TrimSpace(out.Response.Content)
	if content == "" {
		content = strings.TrimSpace(out.Response.CertificateInfo)
	}
	if content == "" {
		return nil, trace, withTencentTrace(fmt.Errorf("腾讯云未返回证书下载内容"), trace)
	}
	bundle, err := parseTencentCertificateContent(content)
	if err != nil {
		return nil, trace, withTencentTrace(err, trace)
	}
	return bundle, trace, nil
}

func deployCertificate(ctx context.Context, account model.DeployAccount, client *http.Client, siteName, certPEM, keyPEM string) (*deployRequestTrace, error) {
	if isBTPanelProvider(account.Provider) {
		return deployBTPanelCertificate(ctx, account, client, siteName, certPEM, keyPEM)
	}
	return nil, fmt.Errorf("暂不支持该部署服务：%s", account.Provider)
}

func deployBTPanelCertificate(ctx context.Context, account model.DeployAccount, client *http.Client, siteName, certPEM, keyPEM string) (*deployRequestTrace, error) {
	if strings.TrimSpace(siteName) == "" {
		return nil, fmt.Errorf("宝塔站点名称不能为空")
	}
	form := url.Values{}
	form.Set("action", "SetSSL")
	form.Set("siteName", siteName)
	form.Set("key", strings.TrimSpace(keyPEM))
	form.Set("csr", strings.TrimSpace(certPEM))
	var out map[string]any
	trace, err := btPanelRequest(ctx, account, client, "/site", form, &out, "request_token", "key", "csr")
	if err != nil {
		return trace, err
	}
	if !btPanelSuccess(out) {
		return trace, fmt.Errorf("宝塔面板返回失败：%s", trace.ResponseBody)
	}
	return trace, nil
}

func btPanelSuccess(out map[string]any) bool {
	for _, key := range []string{"status", "success"} {
		value, ok := out[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case bool:
			return v
		case float64:
			return v == 1
		case string:
			text := strings.ToLower(strings.TrimSpace(v))
			return text == "true" || text == "1" || text == "success"
		}
	}
	if code, ok := out["code"].(float64); ok {
		return code == 0 || code == 200
	}
	return true
}

func tencentSSLRequest(ctx context.Context, account model.SSLAccount, action string, payload any, out any) (*tencentRequestTrace, error) {
	if strings.TrimSpace(account.AccessKey) == "" || strings.TrimSpace(account.SecretKey) == "" {
		return nil, fmt.Errorf("请先填写腾讯云SecretId和SecretKey")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	hashedPayload := sha256Hex(raw)
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + tencentSSLHost + "\n"
	signedHeaders := "content-type;host"
	canonicalRequest := strings.Join([]string{"POST", "/", "", canonicalHeaders, signedHeaders, hashedPayload}, "\n")
	credentialScope := date + "/ssl/tc3_request"
	stringToSign := strings.Join([]string{"TC3-HMAC-SHA256", strconv.FormatInt(timestamp, 10), credentialScope, sha256Hex([]byte(canonicalRequest))}, "\n")
	secretDate := hmacSHA256([]byte("TC3"+account.SecretKey), date)
	secretService := hmacSHA256(secretDate, "ssl")
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	authorization := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", account.AccessKey, credentialScope, signedHeaders, signature)
	trace := &tencentRequestTrace{
		RequestURL:    "https://" + tencentSSLHost,
		RequestMethod: http.MethodPost,
		RequestBody:   string(raw),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+tencentSSLHost, bytes.NewReader(raw))
	if err != nil {
		return trace, withTencentTrace(err, trace)
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", tencentSSLHost)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Version", tencentSSLVersion)
	trace.RequestHeaders = mustJSON(map[string]string{
		"Authorization":  maskTencentAuthorization(authorization),
		"Content-Type":   "application/json; charset=utf-8",
		"Host":           tencentSSLHost,
		"X-TC-Action":    action,
		"X-TC-Timestamp": strconv.FormatInt(timestamp, 10),
		"X-TC-Version":   tencentSSLVersion,
	})

	client, ok := ctx.Value(sslHTTPClientContextKey{}).(*http.Client)
	if !ok || client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return trace, withTencentTrace(err, trace)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	trace.ResponseBody = string(body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return trace, withTencentTrace(fmt.Errorf("腾讯云SSL请求失败，HTTP状态码：%d，响应：%s", resp.StatusCode, string(body)), trace)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return trace, withTencentTrace(fmt.Errorf("解析腾讯云SSL响应失败：%w", err), trace)
	}
	return trace, nil
}

func tencentResponseErr(err *tencentError, trace *tencentRequestTrace) error {
	if err == nil || err.Code == "" {
		return nil
	}
	return withTencentTrace(fmt.Errorf("%s：%s", err.Code, err.Message), trace)
}

func parseTencentCertificateContent(content string) (*tencentCertificateBundle, error) {
	raw, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("解码腾讯云证书下载内容失败：%w", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("读取腾讯云证书下载包失败：%w", err)
	}
	bundle := &tencentCertificateBundle{}
	seenCerts := map[string]bool{}
	chainParts := []string{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		body, err := readZipFile(file)
		if err != nil {
			return nil, err
		}
		name := strings.ToLower(file.Name)
		for {
			block, rest := pem.Decode(body)
			if block == nil {
				break
			}
			pemText := string(pem.EncodeToMemory(block))
			switch {
			case strings.Contains(block.Type, "PRIVATE KEY") && bundle.KeyPEM == "":
				bundle.KeyPEM = strings.TrimSpace(pemText)
			case block.Type == "CERTIFICATE":
				if seenCerts[pemText] {
					body = rest
					continue
				}
				seenCerts[pemText] = true
				if bundle.CertPEM == "" && !looksLikeChainFile(name) {
					bundle.CertPEM = strings.TrimSpace(pemText)
				} else {
					chainParts = append(chainParts, strings.TrimSpace(pemText))
				}
			}
			body = rest
		}
	}
	if bundle.CertPEM == "" && len(chainParts) > 0 {
		bundle.CertPEM = chainParts[0]
		chainParts = chainParts[1:]
	}
	bundle.ChainPEM = strings.TrimSpace(strings.Join(chainParts, "\n"))
	return bundle, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开腾讯云证书文件失败：%w", err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("读取腾讯云证书文件失败：%w", err)
	}
	return body, nil
}

func looksLikeChainFile(name string) bool {
	return strings.Contains(name, "chain") ||
		strings.Contains(name, "ca") ||
		strings.Contains(name, "root") ||
		strings.Contains(name, "bundle")
}

func (h *CertificateHandler) acmeClient(ctx context.Context, account *model.SSLAccount) (*acme.Client, *tencentRequestTrace, error) {
	key, keyPEM, err := accountPrivateKey(account.AccountKeyPEM)
	if err != nil {
		return nil, nil, err
	}
	directoryURL := acmeDirectoryURL(*account)
	client := &acme.Client{
		Key:          key,
		DirectoryURL: directoryURL,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		UserAgent:    "QDL-DaSSLm/1.0",
	}
	if httpClient, ok := ctx.Value(sslHTTPClientContextKey{}).(*http.Client); ok && httpClient != nil {
		client.HTTPClient = httpClient
	}
	if strings.TrimSpace(account.AccountURI) != "" && strings.TrimSpace(account.AccountKeyPEM) != "" {
		client.KID = acme.KeyID(account.AccountURI)
		return client, acmeTrace(account.Provider, "Account", gin.H{"directoryUrl": directoryURL}, gin.H{"accountUri": account.AccountURI}, nil), nil
	}
	acct := &acme.Account{Contact: []string{"mailto:" + strings.TrimSpace(account.Email)}}
	if strings.TrimSpace(account.EABKid) != "" && strings.TrimSpace(account.EABHmacKey) != "" {
		acct.ExternalAccountBinding = &acme.ExternalAccountBinding{
			KID: account.EABKid,
			Key: []byte(account.EABHmacKey),
		}
	}
	registered, err := client.Register(ctx, acct, acme.AcceptTOS)
	if errors.Is(err, acme.ErrAccountAlreadyExists) {
		registered, err = client.GetReg(ctx, "")
	}
	trace := acmeTrace(account.Provider, "RegisterAccount", gin.H{"directoryUrl": directoryURL, "email": account.Email}, registered, err)
	if err != nil {
		return nil, trace, err
	}
	updates := map[string]any{"account_key_pem": keyPEM}
	if registered != nil && registered.URI != "" {
		updates["account_uri"] = registered.URI
		account.AccountURI = registered.URI
		client.KID = acme.KeyID(registered.URI)
	}
	if strings.TrimSpace(account.AccountKeyPEM) == "" {
		account.AccountKeyPEM = keyPEM
	}
	if err := h.db.Model(account).Updates(updates).Error; err != nil {
		return nil, trace, err
	}
	return client, trace, nil
}

func accountPrivateKey(value string) (*rsa.PrivateKey, string, error) {
	if strings.TrimSpace(value) != "" {
		key, err := parseRSAPrivateKeyPEM(value)
		return key, value, err
	}
	return generateRSAPrivateKeyPEM()
}

func generateRSAPrivateKeyPEM() (*rsa.PrivateKey, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}
	raw := x509.MarshalPKCS1PrivateKey(key)
	return key, strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: raw}))), nil
}

func parseRSAPrivateKeyPEM(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, fmt.Errorf("PEM私钥内容无效")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("仅支持RSA私钥")
	}
	return key, nil
}

func createCSRDER(key *rsa.PrivateKey, names []string) ([]byte, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("CSR域名不能为空")
	}
	req := &x509.CertificateRequest{
		Subject:  pkixName(names[0]),
		DNSNames: names,
	}
	return x509.CreateCertificateRequest(rand.Reader, req, key)
}

func pkixName(commonName string) pkix.Name {
	return pkix.Name{CommonName: commonName}
}

func acmeDirectoryURL(account model.SSLAccount) string {
	if strings.TrimSpace(account.DirectoryURL) != "" {
		return strings.TrimSpace(account.DirectoryURL)
	}
	return acme.LetsEncryptURL
}

func isACMEProvider(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	return provider == "letsencrypt" || provider == "custom_acme" || provider == "zerossl"
}

func certificateNames(row *model.Certificate, fallback string) []string {
	items := []string{firstNonEmpty(row.CommonName, fallback)}
	text := strings.TrimSpace(row.SANs)
	if text != "" {
		var arr []string
		if err := json.Unmarshal([]byte(text), &arr); err == nil {
			items = append(items, arr...)
		} else {
			items = append(items, strings.FieldsFunc(text, func(r rune) bool {
				return r == '\n' || r == '\r' || r == ',' || r == ';'
			})...)
		}
	}
	seen := map[string]bool{}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(item, ".")))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func pickDNS01Challenge(authz *acme.Authorization) *acme.Challenge {
	if authz == nil {
		return nil
	}
	for _, challenge := range authz.Challenges {
		if challenge != nil && challenge.Type == "dns-01" {
			return challenge
		}
	}
	return nil
}

func parseACMEAuthRecords(value string) []acmeAuthRecord {
	var records []acmeAuthRecord
	if strings.TrimSpace(value) == "" {
		return records
	}
	_ = json.Unmarshal([]byte(value), &records)
	return records
}

func hasACMEChallengeRecords(row *model.Certificate) bool {
	records := parseACMEAuthRecords(row.AuthRecords)
	if len(records) == 0 && strings.TrimSpace(row.AuthRecordID) != "" {
		return true
	}
	for _, record := range records {
		if strings.TrimSpace(record.ChallengeURL) != "" {
			return true
		}
	}
	return false
}

func acmeOrderStatus(status, fallback string) string {
	switch status {
	case acme.StatusValid:
		return certStatusIssued
	case acme.StatusInvalid:
		return certStatusFailed
	case acme.StatusReady, acme.StatusProcessing:
		return certStatusSubmitted
	case acme.StatusPending:
		return firstNonEmpty(fallback, certStatusDNSAdded)
	default:
		return firstNonEmpty(fallback, certStatusPending)
	}
}

func acmeTrace(provider, action string, request any, response any, err error) *tencentRequestTrace {
	trace := &tencentRequestTrace{
		RequestURL:     "acme://" + firstNonEmpty(provider, "letsencrypt") + "/" + action,
		RequestMethod:  "ACME",
		RequestHeaders: mustJSON(map[string]string{"Provider": provider}),
		RequestBody:    mustJSON(request),
	}
	if err != nil {
		trace.ResponseBody = mustJSON(map[string]string{"error": err.Error()})
	} else {
		trace.ResponseBody = mustJSON(response)
	}
	return trace
}

func tencentVerificationPassed(results []tencentDomainVerificationResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, item := range results {
		if item.Issued != nil {
			if !*item.Issued {
				return false
			}
			continue
		}
		if item.LocalCheck != nil && item.CACheck != nil {
			if *item.LocalCheck != 1 || *item.CACheck != 1 {
				return false
			}
			continue
		}
		if item.NeedVerify != nil && !*item.NeedVerify {
			continue
		}
		return false
	}
	return true
}

func tencentRevokeAuthRecordName(auth tencentRevokeAuth) string {
	key := strings.TrimSpace(strings.TrimSuffix(auth.Key, "."))
	domain := strings.TrimSpace(strings.TrimSuffix(auth.Domain, "."))
	if key == "" {
		return ""
	}
	if domain == "" || strings.Contains(key, ".") {
		return key
	}
	return key + "." + domain
}

func needsCertificateOrder(row model.Certificate, account model.SSLAccount) bool {
	if isACMEProvider(account.Provider) {
		return strings.TrimSpace(row.ProviderOrderID) == ""
	}
	return strings.TrimSpace(row.ProviderCertificateID) == ""
}

func shouldRefreshCertificate(row model.Certificate, account model.SSLAccount) bool {
	if isACMEProvider(account.Provider) {
		return strings.TrimSpace(row.ProviderOrderID) != ""
	}
	return strings.EqualFold(account.Provider, "tencent_free") && strings.TrimSpace(row.ProviderCertificateID) != ""
}

func isTruthyQuery(value string) bool {
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func isLocalCertificateIssued(row model.Certificate) bool {
	if row.Status == certStatusIssued {
		return true
	}
	if strings.TrimSpace(row.CertPEM) != "" {
		return true
	}
	return strings.TrimSpace(row.ProviderCertificateID) != "" && row.ExpiresAt != nil
}

func verifyLogMessage(account model.SSLAccount) string {
	if isACMEProvider(account.Provider) {
		return "已提交并检查ACME验证状态"
	}
	return "已刷新证书状态"
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func maskTencentAuthorization(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "Signature=")
	if len(parts) != 2 {
		return "TC3-HMAC-SHA256 ***"
	}
	return parts[0] + "Signature=***"
}

func pickTencentAuthRecord(detail *tencentDVAuthDetail) (string, string) {
	if detail == nil {
		return "", ""
	}
	for _, item := range detail.DvAuths {
		name := firstNonEmpty(item.DvAuthSubDomain, item.DvAuthKey, item.DvAuthDomain)
		value := strings.TrimSpace(item.DvAuthValue)
		if name != "" && value != "" {
			return name, value
		}
	}
	return firstNonEmpty(detail.DvAuthKeySubDomain, detail.DvAuthKey, detail.DvAuthDomain), strings.TrimSpace(detail.DvAuthValue)
}

func normalizeAuthRR(name, domainName string) string {
	rr := strings.TrimSpace(strings.TrimSuffix(name, "."))
	domain := strings.TrimSpace(strings.TrimSuffix(domainName, "."))
	if rr == "" || rr == domain {
		return "@"
	}
	suffix := "." + domain
	if strings.HasSuffix(rr, suffix) {
		rr = strings.TrimSuffix(rr, suffix)
	}
	if rr == "" {
		return "@"
	}
	return rr
}

func tencentCertStatus(status uint64, fallback string) string {
	switch status {
	case 0, 8:
		return certStatusSubmitted
	case 1:
		return certStatusIssued
	case 2:
		return certStatusFailed
	case 3:
		return certStatusExpired
	case 4:
		return certStatusDNSAdded
	case 6, 7:
		return certStatusCanceled
	case 9:
		return certStatusRevoking
	case 10:
		return certStatusRevoked
	case 12:
		return certStatusRevoking
	default:
		if fallback != "" {
			return fallback
		}
		return certStatusPending
	}
}

func parseTencentTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}
