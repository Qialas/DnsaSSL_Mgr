package model

import "time"

type DomainAccount struct {
	BaseModel
	Name       string  `gorm:"size:128;not null;comment:域名账号名称" json:"name"`
	Provider   string  `gorm:"size:64;not null;comment:DNS服务商类型，如aliyun、dnspod、tencentcloud、cloudflare" json:"provider"`
	AccessKey  string  `gorm:"size:255;comment:服务商AccessKey或账号标识" json:"accessKey"`
	SecretKey  string  `gorm:"size:512;comment:服务商SecretKey或授权密钥" json:"secretKey"`
	Remark     string  `gorm:"size:255;comment:备注信息" json:"remark"`
	Status     string  `gorm:"size:32;not null;default:enabled;comment:账号状态：enabled启用 disabled禁用" json:"status"`
	LastTestAt *string `gorm:"size:32;comment:最近连通性检测时间" json:"lastTestAt"`
	UseProxy   bool    `gorm:"not null;default:false;comment:是否通过代理池请求服务商" json:"useProxy"`
	ProxyMode  string  `gorm:"size:32;not null;default:manual;comment:代理模式：manual指定 auto自动轮询" json:"proxyMode"`
	ProxyID    uint    `gorm:"comment:指定代理池节点ID" json:"proxyId"`
}

func (DomainAccount) TableName() string {
	return "qdl_domain_accounts"
}

type Domain struct {
	BaseModel
	Name             string     `gorm:"size:255;uniqueIndex:idx_domain_account_name;not null;comment:域名名称，如example.com" json:"name"`
	DomainAccountID  uint       `gorm:"uniqueIndex:idx_domain_account_name;comment:关联的域名账号ID" json:"domainAccountId"`
	ProviderDomainID string     `gorm:"size:128;comment:DNS服务商侧域名ID" json:"providerDomainId"`
	ProviderGrade    string     `gorm:"size:64;comment:DNS服务商侧域名套餐等级，如腾讯云DP_FREE" json:"providerGrade"`
	DNSProvider      string     `gorm:"size:64;comment:DNS服务商类型" json:"dnsProvider"`
	RecordCount      int        `gorm:"not null;default:0;comment:DNS解析记录数量" json:"recordCount"`
	ExpiresAt        *time.Time `gorm:"comment:域名到期时间，由WHOIS查询写入" json:"expiresAt"`
	Remark           string     `gorm:"size:255;comment:备注信息" json:"remark"`
	Status           string     `gorm:"size:32;not null;default:enabled;comment:域名状态：enabled启用 disabled停用" json:"status"`
}

func (Domain) TableName() string {
	return "qdl_domains"
}

type DomainRecord struct {
	BaseModel
	DomainID         uint   `gorm:"index;comment:关联域名ID" json:"domainId"`
	ProviderRecordID string `gorm:"size:128;index;comment:DNS服务商侧记录ID" json:"providerRecordId"`
	RR               string `gorm:"size:128;not null;comment:主机记录，如@、www" json:"rr"`
	Type             string `gorm:"size:32;not null;comment:记录类型，如A、AAAA、CNAME、MX、TXT" json:"type"`
	Value            string `gorm:"size:512;not null;comment:记录值" json:"value"`
	Line             string `gorm:"size:64;comment:解析线路" json:"line"`
	TTL              int64  `gorm:"not null;default:600;comment:TTL秒数" json:"ttl"`
	Priority         int64  `gorm:"comment:MX优先级" json:"priority"`
	Remark           string `gorm:"size:255;comment:备注信息" json:"remark"`
	Status           string `gorm:"size:32;comment:记录状态：enabled启用 disabled暂停" json:"status"`
}

func (DomainRecord) TableName() string {
	return "qdl_domain_records"
}
