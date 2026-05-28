package model

import "time"

type AutoTask struct {
	BaseModel
	Name          string     `gorm:"size:128;not null;comment:自动任务名称" json:"name"`
	TaskType      string     `gorm:"size:64;not null;comment:任务类型：dns_sync域名同步 ssl_issue证书申请 ssl_deploy证书部署" json:"taskType"`
	CronExpr      string     `gorm:"size:64;not null;comment:Cron执行表达式" json:"cronExpr"`
	TargetID      uint       `gorm:"comment:任务目标资源ID" json:"targetId"`
	TargetType    string     `gorm:"size:64;comment:任务目标类型：domain、certificate、account" json:"targetType"`
	LastRunAt     *time.Time `gorm:"comment:最近执行时间" json:"lastRunAt"`
	NextRunAt     *time.Time `gorm:"comment:下次执行时间" json:"nextRunAt"`
	LastRunStatus string     `gorm:"size:32;comment:最近执行状态：success成功 failed失败 running运行中" json:"lastRunStatus"`
	Status        string     `gorm:"size:32;not null;default:enabled;comment:任务状态：enabled启用 disabled停用" json:"status"`
	Remark        string     `gorm:"size:255;comment:备注信息" json:"remark"`
}

func (AutoTask) TableName() string {
	return "qdl_auto_tasks"
}
