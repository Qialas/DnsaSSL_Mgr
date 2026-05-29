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
	ProviderCertificateID string `gorm:"size:512;index;comment:证书服务商侧证书ID或证书URL" json:"providerCertificateId"`
	CommonName            string `gorm:"size:255;comment:证书主域名" json:"commonName"`
	Action                string `gorm:"size:64;not null;comment:证书动作：apply申请 submit提交 verify验证 revoke吊销 detail详情 cleanup清理" json:"action"`
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

type CertificateDeployLog struct {
	BaseModel
	CertificateID   uint   `gorm:"index;comment:关联证书ID" json:"certificateId"`
	DeployAccountID uint   `gorm:"index;comment:关联部署账号ID" json:"deployAccountId"`
	CommonName      string `gorm:"size:255;comment:证书主域名" json:"commonName"`
	Provider        string `gorm:"size:64;comment:部署服务商，如btpanel" json:"provider"`
	TargetName      string `gorm:"size:255;comment:部署目标名称，如宝塔站点名称" json:"targetName"`
	Status          string `gorm:"size:32;comment:部署状态：success成功 failed失败" json:"status"`
	Message         string `gorm:"size:512;comment:部署结果说明" json:"message"`
	RequestURL      string `gorm:"size:512;comment:部署服务请求URL" json:"requestUrl"`
	RequestMethod   string `gorm:"size:16;comment:部署服务请求方法" json:"requestMethod"`
	RequestHeaders  string `gorm:"type:text;comment:部署服务请求头，敏感信息已脱敏" json:"requestHeaders"`
	RequestBody     string `gorm:"type:longtext;comment:部署服务请求体，敏感信息已脱敏" json:"requestBody"`
	ResponseBody    string `gorm:"type:longtext;comment:部署服务响应体" json:"responseBody"`
}

func (CertificateDeployLog) TableName() string {
	return "qdl_certificate_deploy_logs"
}

type DomainAccountLog struct {
	BaseModel
	DomainAccountID uint   `gorm:"index;comment:关联域名账号ID" json:"domainAccountId"`
	DomainID        uint   `gorm:"index;comment:关联域名ID" json:"domainId"`
	Provider        string `gorm:"size:64;comment:DNS服务商" json:"provider"`
	TargetName      string `gorm:"size:255;comment:操作目标，如域名或解析记录" json:"targetName"`
	Action          string `gorm:"size:64;not null;comment:域名账号动作：test检测 domains域名列表 records记录列表 create_record新增记录 update_record更新记录 delete_record删除记录 record_lines线路列表" json:"action"`
	Status          string `gorm:"size:32;comment:执行状态：success成功 failed失败" json:"status"`
	Message         string `gorm:"size:512;comment:执行结果说明" json:"message"`
	RequestURL      string `gorm:"size:512;comment:DNS服务商请求URL或SDK动作" json:"requestUrl"`
	RequestMethod   string `gorm:"size:16;comment:DNS服务商请求方式" json:"requestMethod"`
	RequestHeaders  string `gorm:"type:text;comment:DNS服务商请求头或调用上下文，敏感信息已脱敏" json:"requestHeaders"`
	RequestBody     string `gorm:"type:longtext;comment:DNS服务商请求体" json:"requestBody"`
	ResponseBody    string `gorm:"type:longtext;comment:DNS服务商响应体" json:"responseBody"`
}

func (DomainAccountLog) TableName() string {
	return "qdl_domain_account_logs"
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

type ProxySetting struct {
	BaseModel
	Name       string `gorm:"size:128;not null;comment:代理名称" json:"name"`
	Protocol   string `gorm:"size:16;not null;default:http;comment:代理协议：http、https、sock4、sock5、sock5h" json:"protocol"`
	Host       string `gorm:"size:255;not null;comment:代理主机" json:"host"`
	Port       int    `gorm:"not null;comment:代理端口" json:"port"`
	Username   string `gorm:"size:128;comment:代理用户名" json:"username"`
	Password   string `gorm:"size:255;comment:代理密码" json:"password"`
	Weight     int    `gorm:"not null;default:1;comment:代理池权重" json:"weight"`
	Status     string `gorm:"size:32;not null;default:enabled;comment:状态：enabled启用 disabled停用" json:"status"`
	LastTestAt string `gorm:"size:64;comment:最近检测时间" json:"lastTestAt"`
	Remark     string `gorm:"size:255;comment:备注" json:"remark"`
}

func (ProxySetting) TableName() string {
	return "qdl_proxy_settings"
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
