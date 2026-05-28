package model

import "time"

const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
)

type BaseModel struct {
	ID        uint      `gorm:"primaryKey;comment:主键ID" json:"id"`
	CreatedAt time.Time `gorm:"comment:创建时间" json:"createdAt"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updatedAt"`
}
