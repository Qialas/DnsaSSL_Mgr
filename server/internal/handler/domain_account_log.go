package handler

import (
	"fmt"
	"strings"

	"qdl/server/internal/model"

	"gorm.io/gorm"
)

type domainAccountTrace struct {
	RequestURL     string
	RequestMethod  string
	RequestHeaders string
	RequestBody    string
	ResponseBody   string
}

func logDomainAccount(db *gorm.DB, account model.DomainAccount, domainID uint, targetName, action, status, message string, trace *domainAccountTrace) {
	log := model.DomainAccountLog{
		DomainAccountID: account.ID,
		DomainID:        domainID,
		Provider:        account.Provider,
		TargetName:      targetName,
		Action:          action,
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
	_ = db.Create(&log).Error
}

func dnsTrace(account model.DomainAccount, action string, request any, response any, err error) *domainAccountTrace {
	trace := &domainAccountTrace{
		RequestURL:    dnsActionURL(account.Provider, action),
		RequestMethod: "SDK",
		RequestHeaders: mustJSON(map[string]string{
			"Provider":  account.Provider,
			"AccessKey": maskSecret(account.AccessKey),
		}),
		RequestBody: mustJSON(request),
	}
	if err != nil {
		trace.ResponseBody = mustJSON(map[string]string{"error": err.Error()})
	} else {
		trace.ResponseBody = mustJSON(response)
	}
	return trace
}

func dnsActionURL(provider, action string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "aliyun":
		return fmt.Sprintf("sdk://alidns/%s", action)
	case "dnspod", "tencentcloud":
		return fmt.Sprintf("sdk://dnspod.tencentcloudapi.com/%s", action)
	case "cloudflare":
		return fmt.Sprintf("sdk://api.cloudflare.com/client/v4/%s", action)
	default:
		return fmt.Sprintf("sdk://%s/%s", provider, action)
	}
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 6 {
		return "***"
	}
	return value[:3] + "***" + value[len(value)-3:]
}
