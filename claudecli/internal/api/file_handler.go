package api

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/claudecli/internal/workspace"
	"github.com/gin-gonic/gin"
)

// FileHandler 文件操作处理器
type FileHandler struct {
	workspace *workspace.Manager
}

// NewFileHandler 创建文件处理器
func NewFileHandler(wm *workspace.Manager) *FileHandler {
	return &FileHandler{workspace: wm}
}

// Upload 处理文件上传
func (h *FileHandler) Upload(c *gin.Context) {
	userID := c.PostForm("user_id")
	targetPath := c.PostForm("path")

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	// 确保用户工作空间存在
	if err := h.workspace.EnsureUserWorkspace(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create workspace"})
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded"})
		return
	}
	defer file.Close()

	// 构建目标路径
	filename := header.Filename
	if targetPath != "" {
		filename = filepath.Join(targetPath, filename)
	}

	// 保存文件
	if err := h.workspace.SaveUploadedFile(userID, filename, file); err != nil {
		log.Printf("[FileHandler] Failed to save file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	log.Printf("[FileHandler] File uploaded for user %s: %s", userID, filename)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"filename": filename,
		"size":     header.Size,
	})
}

// Download 处理文件下载
func (h *FileHandler) Download(c *gin.Context) {
	userID := c.Query("user_id")
	filePath := c.Query("path")

	if userID == "" || filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and path are required"})
		return
	}

	// 获取文件绝对路径
	absPath, err := h.workspace.GetAbsolutePath(userID, filePath)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// 获取文件信息
	fileInfo, err := h.workspace.GetFileInfo(userID, filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	// 如果是文件夹，打包为zip下载
	if fileInfo.IsDir {
		h.downloadAsZip(c, absPath, fileInfo.Name)
		return
	}

	// 普通文件下载
	filename := filepath.Base(filePath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "application/octet-stream")

	log.Printf("[FileHandler] File download for user %s: %s", userID, filePath)

	c.File(absPath)
}

// downloadAsZip 将文件夹打包为zip下载
func (h *FileHandler) downloadAsZip(c *gin.Context, dirPath string, dirName string) {
	// 设置响应头
	zipFilename := dirName + ".zip"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))
	c.Header("Content-Type", "application/zip")

	// 创建zip writer
	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	// 遍历目录并添加文件
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// 跳过根目录
		if relPath == "." {
			return nil
		}

		// 使用正斜杠作为zip内路径分隔符
		zipPath := dirName + "/" + strings.ReplaceAll(relPath, string(filepath.Separator), "/")

		if info.IsDir() {
			// 添加目录
			_, err := zipWriter.Create(zipPath + "/")
			return err
		}

		// 添加文件
		writer, err := zipWriter.Create(zipPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		log.Printf("[FileHandler] Failed to create zip: %v", err)
	}

	log.Printf("[FileHandler] Folder downloaded as zip: %s", dirPath)
}
