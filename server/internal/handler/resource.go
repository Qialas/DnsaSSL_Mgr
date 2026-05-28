package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ResourceHandler[T any] struct {
	db *gorm.DB
}

func NewResourceHandler[T any](db *gorm.DB) *ResourceHandler[T] {
	return &ResourceHandler[T]{db: db}
}

func (h *ResourceHandler[T]) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var rows []T
	var total int64
	query := h.db.Model(new(T))
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("name LIKE ? OR remark LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询总数失败")
		return
	}
	if err := query.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询列表失败")
		return
	}
	ok(c, gin.H{"items": rows, "total": total, "page": page, "pageSize": pageSize})
}

func (h *ResourceHandler[T]) Get(c *gin.Context) {
	var row T
	if err := h.db.First(&row, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "数据不存在")
		return
	}
	ok(c, row)
}

func (h *ResourceHandler[T]) Create(c *gin.Context) {
	var row T
	if err := c.ShouldBindJSON(&row); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := h.db.Create(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建失败")
		return
	}
	ok(c, row)
}

func (h *ResourceHandler[T]) Update(c *gin.Context) {
	id := c.Param("id")
	var row T
	if err := h.db.First(&row, id).Error; err != nil {
		fail(c, http.StatusNotFound, "数据不存在")
		return
	}
	if err := c.ShouldBindJSON(&row); err != nil {
		fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := h.db.Save(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, row)
}

func (h *ResourceHandler[T]) Delete(c *gin.Context) {
	if err := h.db.Delete(new(T), c.Param("id")).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	ok(c, gin.H{"deleted": true})
}
