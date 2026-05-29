package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"qdl/server/internal/model"
	proxyservice "qdl/server/internal/service/proxypool"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DeployAccountHandler struct {
	*ResourceHandler[model.DeployAccount]
	db *gorm.DB
}

type btPanelSiteItem struct {
	ID       any    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	PS       string `json:"ps"`
	SSL      any    `json:"ssl"`
	Expire   string `json:"expire"`
	AddTime  string `json:"addTime"`
	Provider string `json:"provider"`
}

func NewDeployAccountHandler(db *gorm.DB) *DeployAccountHandler {
	return &DeployAccountHandler{ResourceHandler: NewResourceHandler[model.DeployAccount](db), db: db}
}

func (h *DeployAccountHandler) Sites(c *gin.Context) {
	account, found := h.deployAccount(c)
	if !found {
		return
	}
	if !isBTPanelProvider(account.Provider) {
		fail(c, http.StatusBadRequest, "当前只支持宝塔面板站点列表")
		return
	}
	client, err := deployHTTPClient(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
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
	items, trace, err := btPanelListSites(c.Request.Context(), account, client, page, pageSize, c.Query("keyword"))
	status := "success"
	message := "已获取宝塔站点列表"
	if err != nil {
		status = "failed"
		message = err.Error()
	}
	h.logDeployAccount("sites", account, "", status, message, trace)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"items": items, "total": len(items), "page": page, "pageSize": pageSize})
}

func (h *DeployAccountHandler) Test(c *gin.Context) {
	account, found := h.deployAccount(c)
	if !found {
		return
	}
	if !isBTPanelProvider(account.Provider) {
		fail(c, http.StatusBadRequest, "当前只支持宝塔面板检测")
		return
	}
	client, err := deployHTTPClient(h.db, account)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	info, trace, err := btPanelGetSystemTotal(c.Request.Context(), account, client)
	status := "success"
	message := "部署账号检测通过"
	if err != nil {
		status = "failed"
		message = err.Error()
		h.logDeployAccount("test", account, "", status, message, trace)
		fail(c, http.StatusBadRequest, "检测失败："+err.Error())
		return
	}
	if !info.Valid() {
		status = "failed"
		message = "宝塔面板返回数据不完整"
		h.logDeployAccount("test", account, "", status, message, trace)
		fail(c, http.StatusBadRequest, "检测失败：宝塔面板返回数据不完整")
		return
	}
	h.logDeployAccount("test", account, "", status, message, trace)
	ok(c, gin.H{
		"message":  fmt.Sprintf("检测通过：%s / %s", info.Version, info.System),
		"system":   info.System,
		"version":  info.Version,
		"cpuNum":   info.CPUNum,
		"memTotal": info.MemTotal,
	})
}

func (h *DeployAccountHandler) deployAccount(c *gin.Context) (model.DeployAccount, bool) {
	var account model.DeployAccount
	if err := h.db.First(&account, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "部署账号不存在")
		return account, false
	}
	return account, true
}

func (h *DeployAccountHandler) logDeployAccount(action string, account model.DeployAccount, targetName, status, message string, trace *deployRequestTrace) {
	log := model.CertificateDeployLog{
		DeployAccountID: account.ID,
		Provider:        account.Provider,
		TargetName:      targetName,
		Status:          status,
		Message:         fmt.Sprintf("%s：%s", action, message),
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

type btPanelSystemTotal struct {
	MemTotal    int     `json:"memTotal"`
	MemFree     int     `json:"memFree"`
	MemRealUsed int     `json:"memRealUsed"`
	CPUNum      int     `json:"cpuNum"`
	CPURealUsed float64 `json:"cpuRealUsed"`
	Time        string  `json:"time"`
	System      string  `json:"system"`
	IsUser      int     `json:"isuser"`
	IsPort      bool    `json:"isport"`
	Version     string  `json:"version"`
}

func (info btPanelSystemTotal) Valid() bool {
	return info.MemTotal > 0 || info.CPUNum > 0 || strings.TrimSpace(info.Version) != "" || strings.TrimSpace(info.System) != ""
}

func deployHTTPClient(db *gorm.DB, account model.DeployAccount) (*http.Client, error) {
	proxySetting, err := deployAccountProxy(db, account)
	if err != nil {
		return nil, err
	}
	return proxyservice.Client(proxySetting, 30*time.Second)
}

func btPanelGetSystemTotal(ctx context.Context, account model.DeployAccount, client *http.Client) (*btPanelSystemTotal, *deployRequestTrace, error) {
	form := url.Values{}
	form.Set("action", "GetSystemTotal")
	var out btPanelSystemTotal
	trace, err := btPanelRequest(ctx, account, client, "/system", form, &out, "request_token")
	if err != nil {
		return nil, trace, err
	}
	return &out, trace, nil
}

func btPanelListSites(ctx context.Context, account model.DeployAccount, client *http.Client, page, pageSize int, keyword string) ([]btPanelSiteItem, *deployRequestTrace, error) {
	form := url.Values{}
	form.Set("action", "getData")
	form.Set("table", "sites")
	form.Set("type", "-1")
	form.Set("list", "true")
	form.Set("p", strconv.Itoa(page))
	form.Set("limit", strconv.Itoa(pageSize))
	if strings.TrimSpace(keyword) != "" {
		form.Set("search", strings.TrimSpace(keyword))
	}
	var out struct {
		Data []btPanelSiteItem `json:"data"`
	}
	trace, err := btPanelRequest(ctx, account, client, "/data", form, &out, "request_token")
	if err != nil {
		return nil, trace, err
	}
	for i := range out.Data {
		out.Data[i].Provider = account.Provider
	}
	return out.Data, trace, nil
}

func btPanelRequest(ctx context.Context, account model.DeployAccount, client *http.Client, path string, form url.Values, out any, sensitiveKeys ...string) (*deployRequestTrace, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(account.Endpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("宝塔面板地址不能为空")
	}
	if strings.TrimSpace(account.AccessKey) == "" {
		return nil, fmt.Errorf("宝塔面板API密钥不能为空")
	}
	requestURL := endpoint + path
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	token := md5Hex(timestamp + md5Hex(strings.TrimSpace(account.AccessKey)))
	form.Set("request_time", timestamp)
	form.Set("request_token", token)
	trace := &deployRequestTrace{
		RequestURL:    requestURL,
		RequestMethod: http.MethodPost,
		RequestHeaders: mustJSON(map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		}),
		RequestBody: maskFormBody(form, sensitiveKeys...),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return trace, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return trace, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	trace.ResponseBody = string(body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return trace, fmt.Errorf("宝塔面板请求失败，HTTP状态码：%d，响应：%s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return trace, fmt.Errorf("解析宝塔面板响应失败：%w", err)
	}
	if message := btPanelErrorMessage(body); message != "" {
		return trace, errors.New(message)
	}
	return trace, nil
}

func btPanelErrorMessage(body []byte) string {
	var out struct {
		Status *bool  `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return ""
	}
	if out.Status != nil && !*out.Status {
		if strings.TrimSpace(out.Msg) != "" {
			return out.Msg
		}
		return "宝塔面板返回失败"
	}
	return ""
}

func isBTPanelProvider(provider string) bool {
	return strings.EqualFold(provider, "btpanel") || strings.EqualFold(provider, "baota")
}

func maskFormBody(form url.Values, sensitiveKeys ...string) string {
	sensitive := map[string]bool{}
	for _, key := range sensitiveKeys {
		sensitive[key] = true
	}
	masked := url.Values{}
	for key, values := range form {
		for _, value := range values {
			if sensitive[key] && strings.TrimSpace(value) != "" {
				masked.Add(key, "***")
				continue
			}
			masked.Add(key, value)
		}
	}
	return masked.Encode()
}
