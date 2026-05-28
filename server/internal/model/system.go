package model

type OperationLog struct {
	BaseModel
	UserID    uint   `gorm:"comment:操作用户ID" json:"userId"`
	Username  string `gorm:"size:64;comment:操作用户名" json:"username"`
	Action    string `gorm:"size:128;not null;comment:操作动作" json:"action"`
	Resource  string `gorm:"size:128;comment:操作资源" json:"resource"`
	Method    string `gorm:"size:16;comment:HTTP请求方法" json:"method"`
	Path      string `gorm:"size:255;comment:HTTP请求路径" json:"path"`
	IP        string `gorm:"size:64;comment:客户端IP地址" json:"ip"`
	UserAgent string `gorm:"size:512;comment:客户端User-Agent" json:"userAgent"`
	Detail    string `gorm:"type:text;comment:操作详情" json:"detail"`
	Status    string `gorm:"size:32;comment:操作状态：success成功 failed失败" json:"status"`
}

func (OperationLog) TableName() string {
	return "qdl_operation_logs"
}

type LoginLog struct {
	BaseModel
	UserID    uint   `gorm:"comment:登录用户ID，登录失败时为0" json:"userId"`
	Username  string `gorm:"size:64;comment:登录用户名" json:"username"`
	IP        string `gorm:"size:64;comment:客户端IP地址" json:"ip"`
	UserAgent string `gorm:"size:512;comment:客户端User-Agent" json:"userAgent"`
	Status    string `gorm:"size:32;comment:登录状态：success成功 failed失败" json:"status"`
	Message   string `gorm:"size:255;comment:登录结果说明" json:"message"`
}

func (LoginLog) TableName() string {
	return "qdl_login_logs"
}

type CertificateLog struct {
	BaseModel
	CertificateID         uint   `gorm:"index;comment:关联证书ID" json:"certificateId"`
	ProviderCertificateID string `gorm:"size:128;index;comment:证书服务商侧证书ID" json:"providerCertificateId"`
	CommonName            string `gorm:"size:255;comment:证书主域名" json:"commonName"`
	Action                string `gorm:"size:64;not null;comment:证书动作：apply申请 submit提交 revoke吊销 detail详情" json:"action"`
	Provider              string `gorm:"size:64;comment:证书服务商" json:"provider"`
	Status                string `gorm:"size:32;comment:执行状态：success成功 failed失败" json:"status"`
	Message               string `gorm:"size:512;comment:执行结果说明" json:"message"`
	RequestURL            string `gorm:"size:512;comment:证书服务商请求URL" json:"requestUrl"`
	RequestMethod         string `gorm:"size:16;comment:证书服务商请求方法" json:"requestMethod"`
	RequestHeaders        string `gorm:"type:text;comment:证书服务商请求头，JSON字符串，敏感签名已脱敏" json:"requestHeaders"`
	RequestBody           string `gorm:"type:longtext;comment:证书服务商请求体" json:"requestBody"`
	ResponseBody          string `gorm:"type:longtext;comment:证书服务商响应体" json:"responseBody"`
}

func (CertificateLog) TableName() string {
	return "qdl_certificate_logs"
}

type SystemSetting struct {
	BaseModel
	SettingKey   string `gorm:"size:128;uniqueIndex;not null;comment:系统设置键名" json:"settingKey"`
	SettingValue string `gorm:"type:text;comment:系统设置值" json:"settingValue"`
	ValueType    string `gorm:"size:32;not null;default:string;comment:值类型：string、number、boolean、json" json:"valueType"`
	Description  string `gorm:"size:255;comment:设置说明" json:"description"`
}

func (SystemSetting) TableName() string {
	return "qdl_system_settings"
}

type NotificationSetting struct {
	BaseModel
	Channel string `gorm:"size:64;not null;comment:通知渠道：email、webhook、wechat" json:"channel"`
	Name    string `gorm:"size:128;not null;comment:通知配置名称" json:"name"`
	Config  string `gorm:"type:text;comment:通知渠道配置，JSON字符串" json:"config"`
	Events  string `gorm:"type:text;comment:订阅事件列表，JSON数组字符串" json:"events"`
	Status  string `gorm:"size:32;not null;default:enabled;comment:通知状态：enabled启用 disabled停用" json:"status"`
}

func (NotificationSetting) TableName() string {
	return "qdl_notification_settings"
}
