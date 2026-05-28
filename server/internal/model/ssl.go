package model

import "time"

type SSLAccount struct {
	BaseModel
	Name         string `gorm:"size:128;not null;comment:SSL账号名称" json:"name"`
	Provider     string `gorm:"size:64;not null;comment:证书服务商或ACME服务，如letsencrypt、zerossl、custom_acme、tencent_free、aliyun_free" json:"provider"`
	Email        string `gorm:"size:128;comment:ACME注册邮箱" json:"email"`
	AccessKey    string `gorm:"size:255;comment:云厂商AccessKey或SecretId" json:"accessKey"`
	SecretKey    string `gorm:"size:512;comment:云厂商SecretKey或SecretAccessKey" json:"secretKey"`
	DirectoryURL string `gorm:"size:512;comment:自定义ACME目录地址" json:"directoryUrl"`
	EABKid       string `gorm:"size:255;comment:ACME外部账号绑定KeyID" json:"eabKid"`
	EABHmacKey   string `gorm:"size:512;comment:ACME外部账号绑定HMAC密钥" json:"eabHmacKey"`
	Remark       string `gorm:"size:255;comment:备注信息" json:"remark"`
	Status       string `gorm:"size:32;not null;default:enabled;comment:账号状态：enabled启用 disabled禁用" json:"status"`
}

func (SSLAccount) TableName() string {
	return "qdl_ssl_accounts"
}

type Certificate struct {
	BaseModel
	DomainID              uint       `gorm:"comment:关联域名ID" json:"domainId"`
	SSLAccountID          uint       `gorm:"comment:关联SSL账号ID" json:"sslAccountId"`
	CommonName            string     `gorm:"size:255;not null;comment:证书主域名" json:"commonName"`
	SANs                  string     `gorm:"type:text;comment:证书备用名称列表，JSON数组字符串" json:"sans"`
	Issuer                string     `gorm:"size:128;comment:签发机构" json:"issuer"`
	SerialNumber          string     `gorm:"size:255;comment:证书序列号" json:"serialNumber"`
	CertPEM               string     `gorm:"type:longtext;comment:证书PEM内容" json:"certPem"`
	KeyPEM                string     `gorm:"type:longtext;comment:私钥PEM内容" json:"keyPem"`
	ChainPEM              string     `gorm:"type:longtext;comment:证书链PEM内容" json:"chainPem"`
	ProviderCertificateID string     `gorm:"size:128;index;comment:证书服务商侧证书ID" json:"providerCertificateId"`
	ProviderOrderID       string     `gorm:"size:128;comment:证书服务商侧订单ID" json:"providerOrderId"`
	ProviderStatus        string     `gorm:"size:64;comment:证书服务商侧状态码" json:"providerStatus"`
	ProviderStatusMsg     string     `gorm:"size:255;comment:证书服务商侧状态说明" json:"providerStatusMsg"`
	VerifyType            string     `gorm:"size:32;comment:证书域名验证方式" json:"verifyType"`
	AuthRecordID          string     `gorm:"size:128;comment:域名验证DNS记录ID" json:"authRecordId"`
	AuthRecordName        string     `gorm:"size:255;comment:域名验证DNS记录主机记录" json:"authRecordName"`
	AuthRecordValue       string     `gorm:"size:512;comment:域名验证DNS记录值" json:"authRecordValue"`
	ExpiresAt             *time.Time `gorm:"comment:证书过期时间" json:"expiresAt"`
	RenewBeforeDay        int        `gorm:"not null;default:30;comment:提前续期天数" json:"renewBeforeDay"`
	Status                string     `gorm:"size:32;not null;default:pending;comment:证书状态：pending待申请 applying申请中 dns_added已添加DNS验证 submitted已提交 issued已签发 failed失败 expired已过期 canceled已取消 revoked已吊销" json:"status"`
}

func (Certificate) TableName() string {
	return "qdl_certificates"
}
