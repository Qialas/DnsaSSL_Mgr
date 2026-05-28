package middleware

import (
	"strings"

	"qdl/server/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func OperationLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if c.Request.Method == "GET" || c.Request.Method == "OPTIONS" {
			return
		}

		username, _ := c.Get("username")
		userID, _ := c.Get("userId")
		status := "success"
		if c.Writer.Status() >= 400 || len(c.Errors) > 0 {
			status = "failed"
		}

		_ = db.Create(&model.OperationLog{
			UserID:    asUint(userID),
			Username:  asString(username),
			Action:    actionName(c.Request.Method),
			Resource:  resourceName(c.FullPath()),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			Status:    status,
		}).Error
	}
}

func actionName(method string) string {
	switch method {
	case "POST":
		return "创建/执行"
	case "PUT":
		return "更新"
	case "DELETE":
		return "删除"
	default:
		return method
	}
}

func resourceName(path string) string {
	path = strings.TrimPrefix(path, "/api/")
	if path == "" {
		return "-"
	}
	return strings.Split(path, "/")[0]
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func asUint(value any) uint {
	if n, ok := value.(uint); ok {
		return n
	}
	return 0
}
