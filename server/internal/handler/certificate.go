package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qdl/server/internal/model"
	dnsservice "qdl/server/internal/service/dns"

	"github.com/gin-gonic/gin"
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
	certStatusRevoked   = "revoked"
	tencentSSLHost      = "ssl.tencentcloudapi.com"
	tencentSSLVersion   = "2019-12-05"
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

	if err := h.startCertificateFlow(c.Request.Context(), &row, domain, account); err != nil {
		h.db.Model(&row).Updates(map[string]any{"status": certStatusFailed, "provider_status_msg": err.Error()})
		h.logCertificate(row, account, "apply", "failed", err.Error(), traceFromError(err))
		fail(c, http.StatusBadRequest, "证书申请失败："+err.Error())
		return
	}
	h.db.First(&row, row.ID)
	h.logCertificate(row, account, "apply", "success", "证书申请已创建，DNS验证记录已添加", nil)
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
	if !strings.EqualFold(account.Provider, "tencent_free") {
		fail(c, http.StatusBadRequest, "当前只支持腾讯云免费证书提交")
		return
	}

	if strings.TrimSpace(row.ProviderCertificateID) == "" {
		var domain model.Domain
		if row.DomainID == 0 {
			fail(c, http.StatusBadRequest, "证书未关联域名，无法创建腾讯云申请单")
			return
		}
		if err := h.db.First(&domain, row.DomainID).Error; err != nil {
			fail(c, http.StatusNotFound, "域名不存在")
			return
		}
		if err := h.startCertificateFlow(c.Request.Context(), &row, domain, account); err != nil {
			h.db.Model(&row).Updates(map[string]any{"status": certStatusFailed, "provider_status_msg": err.Error()})
			h.logCertificate(row, account, "apply", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, "创建腾讯云申请单失败："+err.Error())
			return
		}
		h.db.First(&row, row.ID)
		h.logCertificate(row, account, "apply", "success", "证书申请已创建，DNS验证记录已添加", nil)
	}

	if row.Status == certStatusIssued || row.Status == certStatusRevoked || row.Status == certStatusCanceled {
		h.refreshTencentCertificate(c.Request.Context(), &row, account)
		h.db.First(&row, row.ID)
		ok(c, row)
		return
	}

	if row.AuthRecordID == "" && row.Status != certStatusSubmitted {
		if err := h.ensureTencentAuthRecord(c.Request.Context(), &row, account); err != nil {
			h.logCertificate(row, account, "apply", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, "补全DNS验证记录失败："+err.Error())
			return
		}
		h.db.First(&row, row.ID)
	}

	h.refreshTencentCertificate(c.Request.Context(), &row, account)
	h.db.First(&row, row.ID)
	if row.Status == certStatusDNSAdded || row.Status == certStatusApplying || row.Status == certStatusPending {
		h.db.Model(&row).Updates(map[string]any{"status": certStatusSubmitted, "provider_status_msg": "DNS验证记录已添加，等待CA验证"})
		h.db.First(&row, row.ID)
	}
	h.logCertificate(row, account, "submit", "success", "DNS验证记录已添加，等待CA验证", nil)
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
	if row.Status == certStatusIssued {
		if err := tencentSimpleCertificateAction(c.Request.Context(), account, "RevokeCertificate", row.ProviderCertificateID); err != nil {
			h.logCertificate(row, account, "revoke", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		h.db.Model(&row).Updates(map[string]any{"status": certStatusRevoked, "provider_status_msg": "已提交吊销请求"})
		h.logCertificate(row, account, "revoke", "success", "已提交吊销请求", nil)
	} else {
		if err := tencentSimpleCertificateAction(c.Request.Context(), account, "CancelCertificateOrder", row.ProviderCertificateID); err != nil {
			h.logCertificate(row, account, "revoke", "failed", err.Error(), traceFromError(err))
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		h.db.Model(&row).Updates(map[string]any{"status": certStatusCanceled, "provider_status_msg": "已取消证书订单"})
		h.logCertificate(row, account, "revoke", "success", "已取消证书订单", nil)
	}
	h.db.First(&row, row.ID)
	ok(c, row)
}

func (h *CertificateHandler) Detail(c *gin.Context) {
	row, account, found := h.certificateAccount(c)
	if !found {
		return
	}
	if strings.EqualFold(account.Provider, "tencent_free") && row.ProviderCertificateID != "" {
		h.refreshTencentCertificate(c.Request.Context(), &row, account)
		h.db.First(&row, row.ID)
	}
	ok(c, row)
}

func (h *CertificateHandler) startCertificateFlow(ctx context.Context, row *model.Certificate, domain model.Domain, account model.SSLAccount) error {
	if !strings.EqualFold(account.Provider, "tencent_free") {
		return fmt.Errorf("当前只支持腾讯云免费证书自动申请")
	}
	if strings.TrimSpace(account.Email) == "" {
		return fmt.Errorf("腾讯云SSL账号需要填写联系人邮箱")
	}

	certificateID, err := tencentApplyCertificate(ctx, account, row.CommonName)
	if err != nil {
		return err
	}
	row.ProviderCertificateID = certificateID
	h.db.Model(row).Updates(map[string]any{
		"provider_certificate_id": certificateID,
		"verify_type":             "DNS",
		"status":                  certStatusApplying,
	})

	detail, err := h.waitTencentDVDetail(ctx, account, certificateID)
	if err != nil {
		return err
	}
	if detail != nil {
		h.applyTencentDetail(row, detail)
	}
	if detail == nil || detail.DvAuthDetail == nil {
		return fmt.Errorf("腾讯云未返回DNS验证信息")
	}
	authName, authValue := pickTencentAuthRecord(detail.DvAuthDetail)
	if authName == "" || authValue == "" {
		return fmt.Errorf("腾讯云DNS验证信息不完整")
	}
	recordID, rr, err := h.createAuthRecord(ctx, domain, authName, authValue)
	if err != nil {
		return err
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
	return nil
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
	item, err := dnsservice.CreateRecord(ctx, dnsAccount(domainAccount), record)
	if err != nil {
		return "", "", fmt.Errorf("添加DNS验证记录失败：%w", err)
	}
	return item.ID, rr, nil
}

func (h *CertificateHandler) ensureTencentAuthRecord(ctx context.Context, row *model.Certificate, account model.SSLAccount) error {
	if row.DomainID == 0 {
		return fmt.Errorf("证书未关联域名")
	}
	var domain model.Domain
	if err := h.db.First(&domain, row.DomainID).Error; err != nil {
		return fmt.Errorf("域名不存在")
	}
	detail, err := h.waitTencentDVDetail(ctx, account, row.ProviderCertificateID)
	if err != nil {
		return err
	}
	if detail == nil || detail.DvAuthDetail == nil {
		return fmt.Errorf("腾讯云未返回DNS验证信息")
	}
	h.applyTencentDetail(row, detail)
	authName, authValue := pickTencentAuthRecord(detail.DvAuthDetail)
	if authName == "" || authValue == "" {
		return fmt.Errorf("腾讯云DNS验证信息不完整")
	}
	recordID, rr, err := h.createAuthRecord(ctx, domain, authName, authValue)
	if err != nil {
		return err
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
	return nil
}

func (h *CertificateHandler) waitTencentDVDetail(ctx context.Context, account model.SSLAccount, certificateID string) (*tencentDetail, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		detail, err := tencentDescribeCertificate(ctx, account, certificateID)
		if err == nil && detail != nil && detail.DvAuthDetail != nil {
			return detail, nil
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return tencentDescribeCertificate(ctx, account, certificateID)
}

func (h *CertificateHandler) refreshTencentCertificate(ctx context.Context, row *model.Certificate, account model.SSLAccount) {
	detail, err := tencentDescribeCertificate(ctx, account, row.ProviderCertificateID)
	if err != nil || detail == nil {
		return
	}
	h.applyTencentDetail(row, detail)
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

func tencentApplyCertificate(ctx context.Context, account model.SSLAccount, domain string) (string, error) {
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
		return "", err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return "", err
	}
	if out.Response.CertificateID == "" {
		return "", fmt.Errorf("腾讯云未返回证书ID")
	}
	return out.Response.CertificateID, nil
}

func tencentSimpleCertificateAction(ctx context.Context, account model.SSLAccount, action, certificateID string) error {
	var out struct {
		Response struct {
			Error     *tencentError `json:"Error"`
			RequestID string        `json:"RequestId"`
		} `json:"Response"`
	}
	trace, err := tencentSSLRequest(ctx, account, action, map[string]any{"CertificateId": certificateID}, &out)
	if err != nil {
		return err
	}
	return tencentResponseErr(out.Response.Error, trace)
}

func tencentDescribeCertificate(ctx context.Context, account model.SSLAccount, certificateID string) (*tencentDetail, error) {
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
		return nil, err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return nil, err
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
	}, nil
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

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
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
		return "revoking"
	case 10, 12:
		return certStatusRevoked
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

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}
