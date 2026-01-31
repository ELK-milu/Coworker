package api

import (
	"github.com/QuantumNous/new-api/claudecli/internal/session"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RESTHandler REST API 处理器
type RESTHandler struct {
	sessions *session.Manager
}

// NewRESTHandler 创建 REST 处理器
func NewRESTHandler(sm *session.Manager) *RESTHandler {
	return &RESTHandler{sessions: sm}
}

// CreateSession 创建会话
func (h *RESTHandler) CreateSession(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sess := h.sessions.Create(req.UserID)
	c.JSON(http.StatusOK, gin.H{"session_id": sess.ID})
}

// GetSession 获取会话
func (h *RESTHandler) GetSession(c *gin.Context) {
	id := c.Param("id")
	sess := h.sessions.Get(id)
	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, sess)
}

// DeleteSession 删除会话
func (h *RESTHandler) DeleteSession(c *gin.Context) {
	id := c.Param("id")
	h.sessions.Delete(id)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// Health 健康检查
func (h *RESTHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
