package handler

import (
	"fmt"
	"net/http"
	"qdl/server/internal/model"
	proxyservice "qdl/server/internal/service/proxypool"
	"time"

	"gorm.io/gorm"
)

func domainAccountProxy(db *gorm.DB, account model.DomainAccount) (*model.ProxySetting, error) {
	return proxyservice.Resolve(db, fmt.Sprintf("domain-account-%d", account.ID), account.UseProxy, account.ProxyMode, account.ProxyID)
}

func sslAccountProxy(db *gorm.DB, account model.SSLAccount) (*model.ProxySetting, error) {
	return proxyservice.Resolve(db, fmt.Sprintf("ssl-account-%d", account.ID), account.UseProxy, account.ProxyMode, account.ProxyID)
}

func deployAccountProxy(db *gorm.DB, account model.DeployAccount) (*model.ProxySetting, error) {
	return proxyservice.Resolve(db, fmt.Sprintf("deploy-account-%d", account.ID), account.UseProxy, account.ProxyMode, account.ProxyID)
}

func sslHTTPClient(db *gorm.DB, account model.SSLAccount, timeout time.Duration) (*http.Client, error) {
	proxySetting, err := sslAccountProxy(db, account)
	if err != nil {
		return nil, err
	}
	return proxyservice.Client(proxySetting, timeout)
}
