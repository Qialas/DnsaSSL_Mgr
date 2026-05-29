package dns

import (
	"bytes"
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

	alidns "github.com/alibabacloud-go/alidns-20150109/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

type Account struct {
	Provider   string
	AccessKey  string
	SecretKey  string
	ProxyURL   string
	HTTPClient *http.Client
}

type DomainItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Grade       string `json:"grade"`
	RecordCount int    `json:"recordCount"`
	Status      string `json:"status"`
}

type RecordItem struct {
	ID       string `json:"id"`
	RR       string `json:"rr"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Line     string `json:"line"`
	TTL      int64  `json:"ttl"`
	Priority int64  `json:"priority"`
	Remark   string `json:"remark"`
	Status   string `json:"status"`
}

type RecordLineItem struct {
	Name   string `json:"name"`
	LineID string `json:"lineId"`
}

type RecordInput struct {
	DomainName string
	RecordID   string
	RR         string `json:"rr"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	Line       string `json:"line"`
	TTL        int64  `json:"ttl"`
	Priority   int64  `json:"priority"`
	Remark     string `json:"remark"`
	Status     string `json:"status"`
}

type TestResult struct {
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
}

type Provider interface {
	Test(ctx context.Context, account Account) (*TestResult, error)
	ListDomains(ctx context.Context, account Account) ([]DomainItem, error)
	ListRecords(ctx context.Context, account Account, domainName string) ([]RecordItem, error)
	ListRecordLines(ctx context.Context, account Account, domainName, domainGrade string) ([]RecordLineItem, error)
	CreateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error)
	UpdateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error)
	DeleteRecord(ctx context.Context, account Account, domainName, recordID string) error
}

func NewProvider(name string) (Provider, error) {
	switch strings.ToLower(name) {
	case "aliyun":
		return aliyunProvider{}, nil
	case "dnspod", "tencentcloud":
		return tencentProvider{}, nil
	case "cloudflare":
		return cloudflareProvider{}, nil
	default:
		return nil, fmt.Errorf("暂不支持该DNS渠道商：%s", name)
	}
}

func TestAccount(ctx context.Context, account Account) (*TestResult, error) {
	provider, err := checkedProvider(account)
	if err != nil {
		return nil, err
	}
	return provider.Test(ctx, account)
}

func checkedProvider(account Account) (Provider, error) {
	provider, err := NewProvider(account.Provider)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(account.Provider) == "cloudflare" {
		if strings.TrimSpace(account.SecretKey) == "" {
			return nil, errors.New("请先填写Cloudflare API Token")
		}
	} else if strings.TrimSpace(account.AccessKey) == "" || strings.TrimSpace(account.SecretKey) == "" {
		return nil, errors.New("请先填写AccessKey和SecretKey")
	}
	return provider, nil
}

func ListDomains(ctx context.Context, account Account) ([]DomainItem, error) {
	provider, err := checkedProvider(account)
	if err != nil {
		return nil, err
	}
	return provider.ListDomains(ctx, account)
}

func ListRecords(ctx context.Context, account Account, domainName string) ([]RecordItem, error) {
	provider, err := checkedProvider(account)
	if err != nil {
		return nil, err
	}
	return provider.ListRecords(ctx, account, domainName)
}

func ListRecordLines(ctx context.Context, account Account, domainName, domainGrade string) ([]RecordLineItem, error) {
	provider, err := checkedProvider(account)
	if err != nil {
		return nil, err
	}
	return provider.ListRecordLines(ctx, account, domainName, domainGrade)
}

func CreateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	provider, err := checkedProvider(account)
	if err != nil {
		return nil, err
	}
	normalizeRecordInput(&input)
	normalizeProviderRecordInput(account, &input)
	return provider.CreateRecord(ctx, account, input)
}

func UpdateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	provider, err := checkedProvider(account)
	if err != nil {
		return nil, err
	}
	normalizeRecordInput(&input)
	normalizeProviderRecordInput(account, &input)
	return provider.UpdateRecord(ctx, account, input)
}

func DeleteRecord(ctx context.Context, account Account, domainName, recordID string) error {
	provider, err := checkedProvider(account)
	if err != nil {
		return err
	}
	return provider.DeleteRecord(ctx, account, domainName, recordID)
}

func normalizeRecordInput(input *RecordInput) {
	input.RR = strings.TrimSpace(input.RR)
	if input.RR == "" {
		input.RR = "@"
	}
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	input.Value = strings.TrimSpace(input.Value)
	if input.Line == "" {
		input.Line = "default"
	}
	if input.TTL <= 0 {
		input.TTL = 600
	}
	if input.Status == "" {
		input.Status = "enabled"
	}
}

func normalizeProviderRecordInput(account Account, input *RecordInput) {
	if strings.EqualFold(account.Provider, "cloudflare") {
		input.Line = ""
	}
}

type aliyunProvider struct{}

func newAliyunClient(account Account) (*alidns.Client, error) {
	cfg := &openapi.Config{
		AccessKeyId:     tea.String(account.AccessKey),
		AccessKeySecret: tea.String(account.SecretKey),
		Endpoint:        tea.String("alidns.cn-hangzhou.aliyuncs.com"),
	}
	applyAliyunProxy(cfg, account.ProxyURL)
	return alidns.NewClient(cfg)
}

func applyAliyunProxy(cfg *openapi.Config, proxyURL string) {
	if strings.TrimSpace(proxyURL) == "" {
		return
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return
	}
	switch parsed.Scheme {
	case "http":
		cfg.HttpProxy = tea.String(proxyURL)
		cfg.HttpsProxy = tea.String(proxyURL)
	case "https":
		cfg.HttpsProxy = tea.String(proxyURL)
	case "socks5", "socks5h":
		cfg.Socks5Proxy = tea.String(proxyURL)
		cfg.Socks5NetWork = tea.String("tcp")
	}
}

func (aliyunProvider) Test(ctx context.Context, account Account) (*TestResult, error) {
	client, err := newAliyunClient(account)
	if err != nil {
		return nil, err
	}
	req := &alidns.DescribeDomainsRequest{PageSize: tea.Int64(1)}
	if _, err := client.DescribeDomainsWithContext(ctx, req, &dara.RuntimeOptions{}); err != nil {
		return nil, err
	}
	return &TestResult{Provider: "aliyun", OK: true, Message: "阿里云DNS账号连通性正常"}, nil
}

func (aliyunProvider) ListDomains(ctx context.Context, account Account) ([]DomainItem, error) {
	client, err := newAliyunClient(account)
	if err != nil {
		return nil, err
	}
	resp, err := client.DescribeDomainsWithContext(ctx, &alidns.DescribeDomainsRequest{PageSize: tea.Int64(100)}, &dara.RuntimeOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]DomainItem, 0)
	if resp.Body == nil || resp.Body.Domains == nil {
		return items, nil
	}
	for _, item := range resp.Body.Domains.Domain {
		if item == nil {
			continue
		}
		items = append(items, DomainItem{
			ID:          tea.StringValue(item.DomainId),
			Name:        tea.StringValue(item.DomainName),
			RecordCount: int(tea.Int64Value(item.RecordCount)),
			Status:      aliStatus(tea.StringValue(item.DomainLoggingSwitchStatus)),
		})
	}
	return items, nil
}

func (aliyunProvider) ListRecords(ctx context.Context, account Account, domainName string) ([]RecordItem, error) {
	client, err := newAliyunClient(account)
	if err != nil {
		return nil, err
	}
	resp, err := client.DescribeDomainRecordsWithContext(ctx, &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(domainName),
		PageSize:   tea.Int64(500),
	}, &dara.RuntimeOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]RecordItem, 0)
	if resp.Body == nil || resp.Body.DomainRecords == nil {
		return items, nil
	}
	for _, item := range resp.Body.DomainRecords.Record {
		if item == nil {
			continue
		}
		items = append(items, RecordItem{
			ID:       tea.StringValue(item.RecordId),
			RR:       tea.StringValue(item.RR),
			Type:     tea.StringValue(item.Type),
			Value:    tea.StringValue(item.Value),
			Line:     tea.StringValue(item.Line),
			TTL:      tea.Int64Value(item.TTL),
			Priority: tea.Int64Value(item.Priority),
			Remark:   tea.StringValue(item.Remark),
			Status:   aliRecordStatus(tea.StringValue(item.Status)),
		})
	}
	return items, nil
}

func (aliyunProvider) ListRecordLines(_ context.Context, _ Account, _, _ string) ([]RecordLineItem, error) {
	return defaultRecordLines(), nil
}

func (aliyunProvider) CreateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	client, err := newAliyunClient(account)
	if err != nil {
		return nil, err
	}
	resp, err := client.AddDomainRecordWithContext(ctx, &alidns.AddDomainRecordRequest{
		DomainName: tea.String(input.DomainName),
		RR:         tea.String(input.RR),
		Type:       tea.String(input.Type),
		Value:      tea.String(input.Value),
		Line:       tea.String(input.Line),
		TTL:        tea.Int64(input.TTL),
		Priority:   tea.Int64(input.Priority),
	}, &dara.RuntimeOptions{})
	if err != nil {
		return nil, err
	}
	input.RecordID = tea.StringValue(resp.Body.RecordId)
	return recordFromInput(input), nil
}

func (aliyunProvider) UpdateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	client, err := newAliyunClient(account)
	if err != nil {
		return nil, err
	}
	if _, err := client.UpdateDomainRecordWithContext(ctx, &alidns.UpdateDomainRecordRequest{
		RecordId: tea.String(input.RecordID),
		RR:       tea.String(input.RR),
		Type:     tea.String(input.Type),
		Value:    tea.String(input.Value),
		Line:     tea.String(input.Line),
		TTL:      tea.Int64(input.TTL),
		Priority: tea.Int64(input.Priority),
	}, &dara.RuntimeOptions{}); err != nil {
		return nil, err
	}
	return recordFromInput(input), nil
}

func (aliyunProvider) DeleteRecord(ctx context.Context, account Account, _ string, recordID string) error {
	client, err := newAliyunClient(account)
	if err != nil {
		return err
	}
	_, err = client.DeleteDomainRecordWithContext(ctx, &alidns.DeleteDomainRecordRequest{RecordId: tea.String(recordID)}, &dara.RuntimeOptions{})
	return err
}

type tencentProvider struct{}

func newTencentClient(account Account) (*dnspod.Client, error) {
	cred := common.NewCredential(account.AccessKey, account.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"
	if strings.HasPrefix(account.ProxyURL, "http://") || strings.HasPrefix(account.ProxyURL, "https://") {
		cpf.HttpProfile.Proxy = account.ProxyURL
	}
	return dnspod.NewClient(cred, "", cpf)
}

func (tencentProvider) Test(ctx context.Context, account Account) (*TestResult, error) {
	client, err := newTencentClient(account)
	if err != nil {
		return nil, err
	}
	req := dnspod.NewDescribeDomainListRequest()
	if _, err := client.DescribeDomainListWithContext(ctx, req); err != nil {
		return nil, tencentErr(err)
	}
	return &TestResult{Provider: "tencentcloud", OK: true, Message: "腾讯云DNSPod账号连通性正常"}, nil
}

func (tencentProvider) ListDomains(ctx context.Context, account Account) ([]DomainItem, error) {
	client, err := newTencentClient(account)
	if err != nil {
		return nil, err
	}
	limit := int64(100)
	req := dnspod.NewDescribeDomainListRequest()
	req.Limit = &limit
	resp, err := client.DescribeDomainListWithContext(ctx, req)
	if err != nil {
		return nil, tencentErr(err)
	}
	items := make([]DomainItem, 0)
	for _, item := range resp.Response.DomainList {
		if item == nil {
			continue
		}
		items = append(items, DomainItem{
			ID:          strconv.FormatUint(uint64Value(item.DomainId), 10),
			Name:        stringValue(item.Name),
			Grade:       stringValue(item.Grade),
			RecordCount: int(uint64Value(item.RecordCount)),
			Status:      tencentStatus(stringValue(item.Status)),
		})
	}
	return items, nil
}

func (tencentProvider) ListRecords(ctx context.Context, account Account, domainName string) ([]RecordItem, error) {
	client, err := newTencentClient(account)
	if err != nil {
		return nil, err
	}
	limit := uint64(3000)
	no := "no"
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = &domainName
	req.Limit = &limit
	req.ErrorOnEmpty = &no
	resp, err := client.DescribeRecordListWithContext(ctx, req)
	if err != nil {
		return nil, tencentErr(err)
	}
	items := make([]RecordItem, 0)
	for _, item := range resp.Response.RecordList {
		if item == nil {
			continue
		}
		if strings.EqualFold(stringValue(item.Type), "NS") {
			continue
		}
		items = append(items, RecordItem{
			ID:       strconv.FormatUint(uint64Value(item.RecordId), 10),
			RR:       stringValue(item.Name),
			Type:     stringValue(item.Type),
			Value:    stringValue(item.Value),
			Line:     stringValue(item.Line),
			TTL:      int64(uint64Value(item.TTL)),
			Priority: int64(uint64Value(item.MX)),
			Remark:   stringValue(item.Remark),
			Status:   tencentRecordStatus(stringValue(item.Status)),
		})
	}
	return items, nil
}

func (tencentProvider) ListRecordLines(ctx context.Context, account Account, domainName, domainGrade string) ([]RecordLineItem, error) {
	client, err := newTencentClient(account)
	if err != nil {
		return nil, err
	}
	grade := strings.TrimSpace(domainGrade)
	if grade == "" {
		grade = lookupTencentDomainGrade(ctx, client, domainName)
	}
	if grade == "" {
		grade = "DP_FREE"
	}
	req := dnspod.NewDescribeRecordLineListRequest()
	req.Domain = &domainName
	req.DomainGrade = &grade
	resp, err := client.DescribeRecordLineListWithContext(ctx, req)
	if err != nil {
		return nil, tencentErr(err)
	}
	items := make([]RecordLineItem, 0)
	seen := make(map[string]bool)
	if resp.Response != nil {
		for _, item := range resp.Response.LineList {
			if item == nil {
				continue
			}
			name := stringValue(item.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			items = append(items, RecordLineItem{Name: name, LineID: stringValue(item.LineId)})
		}
	}
	if len(items) == 0 {
		return defaultRecordLines(), nil
	}
	return items, nil
}

func (tencentProvider) CreateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	client, err := newTencentClient(account)
	if err != nil {
		return nil, err
	}
	ttl := uint64(input.TTL)
	mx := uint64(input.Priority)
	status := toTencentRecordStatus(input.Status)
	line := tencentLine(input.Line)
	req := dnspod.NewCreateRecordRequest()
	req.Domain = &input.DomainName
	req.SubDomain = &input.RR
	req.RecordType = &input.Type
	req.Value = &input.Value
	req.RecordLine = &line
	req.TTL = &ttl
	req.MX = &mx
	req.Status = &status
	req.Remark = &input.Remark
	resp, err := client.CreateRecordWithContext(ctx, req)
	if err != nil {
		return nil, tencentErr(err)
	}
	input.RecordID = strconv.FormatUint(uint64Value(resp.Response.RecordId), 10)
	return recordFromInput(input), nil
}

func (tencentProvider) UpdateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	client, err := newTencentClient(account)
	if err != nil {
		return nil, err
	}
	recordID, _ := strconv.ParseUint(input.RecordID, 10, 64)
	ttl := uint64(input.TTL)
	mx := uint64(input.Priority)
	status := toTencentRecordStatus(input.Status)
	line := tencentLine(input.Line)
	req := dnspod.NewModifyRecordRequest()
	req.Domain = &input.DomainName
	req.RecordId = &recordID
	req.SubDomain = &input.RR
	req.RecordType = &input.Type
	req.Value = &input.Value
	req.RecordLine = &line
	req.TTL = &ttl
	req.MX = &mx
	req.Status = &status
	req.Remark = &input.Remark
	if _, err := client.ModifyRecordWithContext(ctx, req); err != nil {
		return nil, tencentErr(err)
	}
	return recordFromInput(input), nil
}

func (tencentProvider) DeleteRecord(ctx context.Context, account Account, domainName, recordID string) error {
	client, err := newTencentClient(account)
	if err != nil {
		return err
	}
	id, _ := strconv.ParseUint(recordID, 10, 64)
	req := dnspod.NewDeleteRecordRequest()
	req.Domain = &domainName
	req.RecordId = &id
	if _, err := client.DeleteRecordWithContext(ctx, req); err != nil {
		return tencentErr(err)
	}
	return nil
}

type cloudflareProvider struct{}

func (cloudflareProvider) Test(ctx context.Context, account Account) (*TestResult, error) {
	if err := cfRequest(ctx, account, http.MethodGet, "/user/tokens/verify", nil, nil); err != nil {
		return nil, err
	}
	return &TestResult{Provider: "cloudflare", OK: true, Message: "Cloudflare账号连通性正常"}, nil
}

func (cloudflareProvider) ListDomains(ctx context.Context, account Account) ([]DomainItem, error) {
	var body struct {
		Result []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := cfRequest(ctx, account, http.MethodGet, "/zones?per_page=100", nil, &body); err != nil {
		return nil, err
	}
	items := make([]DomainItem, 0, len(body.Result))
	for _, item := range body.Result {
		items = append(items, DomainItem{ID: item.ID, Name: item.Name, Status: item.Status})
	}
	return items, nil
}

func (cloudflareProvider) ListRecords(ctx context.Context, account Account, domainName string) ([]RecordItem, error) {
	zoneID, err := cloudflareZoneID(ctx, account, domainName)
	if err != nil {
		return nil, err
	}
	var body struct {
		Result []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Name    string `json:"name"`
			Content string `json:"content"`
			TTL     int64  `json:"ttl"`
			Comment string `json:"comment"`
			Proxied bool   `json:"proxied"`
		} `json:"result"`
	}
	if err := cfRequest(ctx, account, http.MethodGet, "/zones/"+zoneID+"/dns_records?per_page=500", nil, &body); err != nil {
		return nil, err
	}
	items := make([]RecordItem, 0, len(body.Result))
	for _, item := range body.Result {
		items = append(items, RecordItem{
			ID:     item.ID,
			RR:     cfRR(item.Name, domainName),
			Type:   item.Type,
			Value:  item.Content,
			TTL:    item.TTL,
			Remark: item.Comment,
			Status: "enabled",
		})
	}
	return items, nil
}

func (cloudflareProvider) ListRecordLines(_ context.Context, _ Account, _, _ string) ([]RecordLineItem, error) {
	return []RecordLineItem{}, nil
}

func (cloudflareProvider) CreateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	zoneID, err := cloudflareZoneID(ctx, account, input.DomainName)
	if err != nil {
		return nil, err
	}
	payload := cfRecordPayload(input)
	var body struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := cfRequest(ctx, account, http.MethodPost, "/zones/"+zoneID+"/dns_records", payload, &body); err != nil {
		return nil, err
	}
	input.RecordID = body.Result.ID
	return recordFromInput(input), nil
}

func (cloudflareProvider) UpdateRecord(ctx context.Context, account Account, input RecordInput) (*RecordItem, error) {
	zoneID, err := cloudflareZoneID(ctx, account, input.DomainName)
	if err != nil {
		return nil, err
	}
	if err := cfRequest(ctx, account, http.MethodPut, "/zones/"+zoneID+"/dns_records/"+input.RecordID, cfRecordPayload(input), nil); err != nil {
		return nil, err
	}
	return recordFromInput(input), nil
}

func (cloudflareProvider) DeleteRecord(ctx context.Context, account Account, domainName, recordID string) error {
	zoneID, err := cloudflareZoneID(ctx, account, domainName)
	if err != nil {
		return err
	}
	return cfRequest(ctx, account, http.MethodDelete, "/zones/"+zoneID+"/dns_records/"+recordID, nil, nil)
}

func cfRequest(ctx context.Context, account Account, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.cloudflare.com/client/v4"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+account.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	client := account.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare请求失败，HTTP状态码：%d", resp.StatusCode)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func cloudflareZoneID(ctx context.Context, account Account, domainName string) (string, error) {
	var body struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := cfRequest(ctx, account, http.MethodGet, "/zones?name="+domainName, nil, &body); err != nil {
		return "", err
	}
	if len(body.Result) == 0 {
		return "", errors.New("Cloudflare未找到对应域名")
	}
	return body.Result[0].ID, nil
}

func cfRecordPayload(input RecordInput) map[string]any {
	name := input.RR
	if input.RR != "@" && !strings.HasSuffix(input.RR, "."+input.DomainName) {
		name = input.RR + "." + input.DomainName
	}
	if input.RR == "@" {
		name = input.DomainName
	}
	return map[string]any{"type": input.Type, "name": name, "content": input.Value, "ttl": input.TTL, "comment": input.Remark}
}

func recordFromInput(input RecordInput) *RecordItem {
	return &RecordItem{
		ID:       input.RecordID,
		RR:       input.RR,
		Type:     input.Type,
		Value:    input.Value,
		Line:     input.Line,
		TTL:      input.TTL,
		Priority: input.Priority,
		Remark:   input.Remark,
		Status:   input.Status,
	}
}

func aliStatus(status string) string {
	if status == "" {
		return "enabled"
	}
	return strings.ToLower(status)
}

func aliRecordStatus(status string) string {
	if strings.EqualFold(status, "ENABLE") {
		return "enabled"
	}
	if strings.EqualFold(status, "DISABLE") {
		return "disabled"
	}
	return strings.ToLower(status)
}

func tencentStatus(status string) string {
	if strings.EqualFold(status, "ENABLE") {
		return "enabled"
	}
	return strings.ToLower(status)
}

func tencentRecordStatus(status string) string {
	if strings.EqualFold(status, "ENABLE") {
		return "enabled"
	}
	if strings.EqualFold(status, "DISABLE") {
		return "disabled"
	}
	return strings.ToLower(status)
}

func toTencentRecordStatus(status string) string {
	if status == "disabled" {
		return "DISABLE"
	}
	return "ENABLE"
}

func tencentLine(line string) string {
	if line == "" || line == "default" {
		return "默认"
	}
	return line
}

func defaultRecordLines() []RecordLineItem {
	return []RecordLineItem{{Name: "默认"}}
}

func lookupTencentDomainGrade(ctx context.Context, client *dnspod.Client, domainName string) string {
	limit := int64(100)
	req := dnspod.NewDescribeDomainListRequest()
	req.Limit = &limit
	resp, err := client.DescribeDomainListWithContext(ctx, req)
	if err != nil || resp.Response == nil {
		return ""
	}
	for _, item := range resp.Response.DomainList {
		if item == nil {
			continue
		}
		if strings.EqualFold(stringValue(item.Name), domainName) {
			return stringValue(item.Grade)
		}
	}
	return ""
}

func cfRR(name, domainName string) string {
	if name == domainName {
		return "@"
	}
	return strings.TrimSuffix(name, "."+domainName)
}

func tencentErr(err error) error {
	if sdkErr, ok := err.(*tcerrors.TencentCloudSDKError); ok {
		return fmt.Errorf("%s：%s", sdkErr.Code, sdkErr.Message)
	}
	return err
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func uint64Value(v *uint64) uint64 {
	if v == nil {
		return 0
	}
	return *v
}
