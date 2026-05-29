package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qdl/server/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SSLAccountHandler struct {
	*ResourceHandler[model.SSLAccount]
	db *gorm.DB
}

func NewSSLAccountHandler(db *gorm.DB) *SSLAccountHandler {
	return &SSLAccountHandler{ResourceHandler: NewResourceHandler[model.SSLAccount](db), db: db}
}

type tencentCertificateItem struct {
	CertificateID   string   `json:"certificateId"`
	Domain          string   `json:"domain"`
	Alias           string   `json:"alias"`
	ProductZhName   string   `json:"productZhName"`
	Status          *uint64  `json:"status"`
	StatusName      string   `json:"statusName"`
	StatusMsg       string   `json:"statusMsg"`
	VerifyType      string   `json:"verifyType"`
	CertBeginTime   string   `json:"certBeginTime"`
	CertEndTime     string   `json:"certEndTime"`
	InsertTime      string   `json:"insertTime"`
	PackageTypeName string   `json:"packageTypeName"`
	SubjectAltName  []string `json:"subjectAltName"`
}

func (h *SSLAccountHandler) Certificates(c *gin.Context) {
	account, found := h.sslAccount(c)
	if !found {
		return
	}
	if !strings.EqualFold(account.Provider, "tencent_free") {
		fail(c, http.StatusBadRequest, "当前只支持腾讯云免费证书获取证书资源")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	client, err := sslHTTPClient(h.db, account, 30*time.Second)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := context.WithValue(c.Request.Context(), sslHTTPClientContextKey{}, client)
	items, total, err := tencentListCertificates(ctx, account, uint64((page-1)*pageSize), uint64(pageSize), c.Query("keyword"))
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *SSLAccountHandler) ImportCertificates(c *gin.Context) {
	account, found := h.sslAccount(c)
	if !found {
		return
	}
	if !strings.EqualFold(account.Provider, "tencent_free") {
		fail(c, http.StatusBadRequest, "当前只支持腾讯云免费证书保存到本地")
		return
	}
	client, err := sslHTTPClient(h.db, account, 30*time.Second)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := context.WithValue(c.Request.Context(), sslHTTPClientContextKey{}, client)
	items, _, err := tencentListCertificates(ctx, account, 0, 100, "")
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	saved := 0
	for _, item := range items {
		row, ok, err := h.saveTencentCertificate(account, item)
		if err != nil {
			fail(c, http.StatusInternalServerError, "保存证书资源失败")
			return
		}
		if !ok {
			continue
		}
		saved++
		h.logTencentCertificateImport(row, account)
	}
	ok(c, gin.H{"saved": saved})
}

func (h *SSLAccountHandler) ImportCertificate(c *gin.Context) {
	account, found := h.sslAccount(c)
	if !found {
		return
	}
	if !strings.EqualFold(account.Provider, "tencent_free") {
		fail(c, http.StatusBadRequest, "当前只支持腾讯云免费证书保存到本地")
		return
	}
	certificateID := strings.TrimSpace(c.Param("certificateId"))
	if certificateID == "" {
		fail(c, http.StatusBadRequest, "证书ID不能为空")
		return
	}
	client, err := sslHTTPClient(h.db, account, 30*time.Second)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := context.WithValue(c.Request.Context(), sslHTTPClientContextKey{}, client)
	items, _, err := tencentListCertificates(ctx, account, 0, 100, certificateID)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	for _, item := range items {
		if item.CertificateID != certificateID {
			continue
		}
		row, saved, err := h.saveTencentCertificate(account, item)
		if err != nil {
			fail(c, http.StatusInternalServerError, "保存证书资源失败")
			return
		}
		if !saved {
			fail(c, http.StatusBadRequest, "证书资源信息不完整，无法导入")
			return
		}
		h.logTencentCertificateImport(row, account)
		ok(c, gin.H{"item": row})
		return
	}
	fail(c, http.StatusNotFound, "腾讯云证书资源不存在")
}

func (h *SSLAccountHandler) saveTencentCertificate(account model.SSLAccount, item tencentCertificateItem) (model.Certificate, bool, error) {
	if strings.TrimSpace(item.CertificateID) == "" || strings.TrimSpace(item.Domain) == "" {
		return model.Certificate{}, false, nil
	}
	row := certificateFromTencentItem(account.ID, item)
	if domainID := h.findDomainID(item.Domain); domainID > 0 {
		row.DomainID = domainID
	}
	err := h.db.Where("provider_certificate_id = ?", item.CertificateID).Assign(row).FirstOrCreate(&row).Error
	return row, true, err
}

func (h *SSLAccountHandler) logTencentCertificateImport(row model.Certificate, account model.SSLAccount) {
	_ = h.db.Create(&model.CertificateLog{
		CertificateID:         row.ID,
		ProviderCertificateID: row.ProviderCertificateID,
		CommonName:            row.CommonName,
		Action:                "import",
		Provider:              account.Provider,
		Status:                "success",
		Message:               "腾讯云证书资源已保存到本地",
	}).Error
}

func (h *SSLAccountHandler) sslAccount(c *gin.Context) (model.SSLAccount, bool) {
	var account model.SSLAccount
	if err := h.db.First(&account, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "SSL账号不存在")
		return account, false
	}
	return account, true
}

func (h *SSLAccountHandler) findDomainID(domainName string) uint {
	name := strings.TrimPrefix(strings.TrimSpace(domainName), "*.")
	var domain model.Domain
	if err := h.db.Where("name = ?", name).First(&domain).Error; err == nil {
		return domain.ID
	}
	return 0
}

func tencentListCertificates(ctx context.Context, account model.SSLAccount, offset, limit uint64, searchKey string) ([]tencentCertificateItem, uint64, error) {
	var out struct {
		Response struct {
			Error        *tencentError `json:"Error"`
			TotalCount   uint64        `json:"TotalCount"`
			Certificates []struct {
				CertificateID   string   `json:"CertificateId"`
				Domain          string   `json:"Domain"`
				Alias           string   `json:"Alias"`
				ProductZhName   string   `json:"ProductZhName"`
				Status          *uint64  `json:"Status"`
				StatusName      string   `json:"StatusName"`
				StatusMsg       string   `json:"StatusMsg"`
				VerifyType      string   `json:"VerifyType"`
				CertBeginTime   string   `json:"CertBeginTime"`
				CertEndTime     string   `json:"CertEndTime"`
				InsertTime      string   `json:"InsertTime"`
				PackageTypeName string   `json:"PackageTypeName"`
				SubjectAltName  []string `json:"SubjectAltName"`
			} `json:"Certificates"`
			RequestID string `json:"RequestId"`
		} `json:"Response"`
	}
	payload := map[string]any{
		"Offset":          offset,
		"Limit":           limit,
		"CertificateType": "SVR",
	}
	if strings.TrimSpace(searchKey) != "" {
		payload["SearchKey"] = strings.TrimSpace(searchKey)
	}
	trace, err := tencentSSLRequest(ctx, account, "DescribeCertificates", payload, &out)
	if err != nil {
		return nil, 0, err
	}
	if err := tencentResponseErr(out.Response.Error, trace); err != nil {
		return nil, 0, err
	}
	items := make([]tencentCertificateItem, 0, len(out.Response.Certificates))
	for _, item := range out.Response.Certificates {
		items = append(items, tencentCertificateItem{
			CertificateID:   item.CertificateID,
			Domain:          item.Domain,
			Alias:           item.Alias,
			ProductZhName:   item.ProductZhName,
			Status:          item.Status,
			StatusName:      item.StatusName,
			StatusMsg:       item.StatusMsg,
			VerifyType:      item.VerifyType,
			CertBeginTime:   item.CertBeginTime,
			CertEndTime:     item.CertEndTime,
			InsertTime:      item.InsertTime,
			PackageTypeName: item.PackageTypeName,
			SubjectAltName:  item.SubjectAltName,
		})
	}
	return items, out.Response.TotalCount, nil
}

func certificateFromTencentItem(accountID uint, item tencentCertificateItem) model.Certificate {
	sans, _ := json.Marshal(item.SubjectAltName)
	row := model.Certificate{
		SSLAccountID:          accountID,
		CommonName:            item.Domain,
		SANs:                  string(sans),
		Issuer:                item.ProductZhName,
		ProviderCertificateID: item.CertificateID,
		ProviderStatusMsg:     firstNonEmpty(item.StatusName, item.StatusMsg),
		VerifyType:            item.VerifyType,
		RenewBeforeDay:        30,
		Status:                certStatusPending,
	}
	if item.Status != nil {
		row.ProviderStatus = strconv.FormatUint(*item.Status, 10)
		row.Status = tencentCertStatus(*item.Status, certStatusPending)
	}
	if expiresAt := parseTencentTime(item.CertEndTime); expiresAt != nil {
		row.ExpiresAt = expiresAt
	}
	if row.CommonName == "" {
		row.CommonName = fmt.Sprintf("certificate-%s", item.CertificateID)
	}
	return row
}
