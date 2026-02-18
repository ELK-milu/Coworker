/*
Copyright (C) 2025 QuantumNous
Google Colab 风格文件浏览器
*/

import React, { useState, useEffect, useRef } from 'react';
import { Typography, Spin, Button, Breadcrumb, Toast, Modal } from '@douyinfe/semi-ui';
import {
  IconFolder,
  IconFile,
  IconRefresh,
  IconUpload,
  IconFolderOpen,
  IconDownload,
  IconDelete,
  IconEdit,
  IconPlus,
  IconImage,
  IconVideo,
  IconMusic,
  IconCode,
  IconArticle,
  IconSetting,
  IconLink,
  IconCopy,
  IconMore,
} from '@douyinfe/semi-icons';
import * as api from '../services/api';
import './FileExplorer.css';

const { Text } = Typography;

// 获取文件扩展名
const getFileExtension = (filename) => {
  const parts = filename.split('.');
  return parts.length > 1 ? parts.pop().toLowerCase() : '';
};

// 根据文件类型获取图标和颜色
const getFileIcon = (filename) => {
  const ext = getFileExtension(filename);

  // 图片文件
  if (['jpg', 'jpeg', 'png', 'gif', 'bmp', 'svg', 'webp', 'ico'].includes(ext)) {
    return { icon: IconImage, color: '#10b981' };
  }

  // 视频文件
  if (['mp4', 'avi', 'mov', 'wmv', 'flv', 'mkv', 'webm'].includes(ext)) {
    return { icon: IconVideo, color: '#f43f5e' };
  }

  // 音频文件
  if (['mp3', 'wav', 'flac', 'aac', 'ogg', 'm4a'].includes(ext)) {
    return { icon: IconMusic, color: '#8b5cf6' };
  }

  // 代码文件
  if (['js', 'jsx', 'ts', 'tsx', 'py', 'go', 'java', 'c', 'cpp', 'h', 'rs', 'rb', 'php', 'swift', 'kt'].includes(ext)) {
    return { icon: IconCode, color: '#3b82f6' };
  }

  // 配置文件
  if (['json', 'yaml', 'yml', 'toml', 'xml', 'ini', 'env', 'conf', 'config'].includes(ext)) {
    return { icon: IconSetting, color: '#6b7280' };
  }

  // 文档文件
  if (['md', 'txt', 'doc', 'docx', 'pdf', 'rtf', 'odt'].includes(ext)) {
    return { icon: IconArticle, color: '#f59e0b' };
  }

  // 压缩文件
  if (['zip', 'rar', '7z', 'tar', 'gz', 'bz2'].includes(ext)) {
    return { icon: IconLink, color: '#ec4899' };
  }

  // 样式文件
  if (['css', 'scss', 'sass', 'less', 'styl'].includes(ext)) {
    return { icon: IconCode, color: '#06b6d4' };
  }

  // HTML 文件
  if (['html', 'htm', 'vue', 'svelte'].includes(ext)) {
    return { icon: IconCode, color: '#ef4444' };
  }

  // 默认文件图标
  return { icon: IconFile, color: 'var(--semi-color-text-2)' };
};

// 格式化文件大小
const formatSize = (bytes) => {
  if (bytes === 0) return '-';
  if (bytes === undefined || bytes === null) return '-';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

// 格式化时间
const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  const date = new Date(timestamp * 1000);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();

  if (isToday) {
    return date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  return date.toLocaleDateString('zh-CN', {
    month: 'short',
    day: 'numeric',
  });
};

// 右键菜单组件
const ContextMenu = ({ x, y, file, onClose, onDownload, onDelete, onRename, onCopyPath }) => {
  const menuRef = useRef(null);

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (menuRef.current && !menuRef.current.contains(e.target)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [onClose]);

  return (
    <div
      ref={menuRef}
      className="file-context-menu"
      style={{ left: x, top: y }}
    >
      <div className="file-context-menu-item" onClick={() => { onDownload(file); onClose(); }}>
        <IconDownload size="small" />
        <span>{file.is_dir ? '下载为 ZIP' : '下载'}</span>
      </div>
      <div className="file-context-menu-item" onClick={() => { onRename(file); onClose(); }}>
        <IconEdit size="small" />
        <span>重命名</span>
      </div>
      <div className="file-context-menu-item" onClick={() => { onCopyPath(file); onClose(); }}>
        <IconCopy size="small" />
        <span>复制路径</span>
      </div>
      <div className="file-context-menu-divider" />
      <div className="file-context-menu-item danger" onClick={() => { onDelete(file); onClose(); }}>
        <IconDelete size="small" />
        <span>删除</span>
      </div>
    </div>
  );
};

const FileExplorer = ({
  files = [],
  currentPath = '',
  loading = false,
  userId,
  onNavigate,
  onRefresh,
  onPreviewFile,
}) => {
  const [isDragging, setIsDragging] = useState(false);
  const [selectedFile, setSelectedFile] = useState(null);
  const [contextMenu, setContextMenu] = useState(null);
  const [actionMenu, setActionMenu] = useState(null);
  const [isCreatingFolder, setIsCreatingFolder] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [uploading, setUploading] = useState(false);
  const [renamingFile, setRenamingFile] = useState(null);
  const [renameValue, setRenameValue] = useState('');
  const fileInputRef = useRef(null);
  const folderInputRef = useRef(null);
  const newFolderInputRef = useRef(null);
  const renameInputRef = useRef(null);

  // 解析路径为面包屑
  const pathParts = currentPath ? currentPath.split('/').filter(Boolean) : [];

  // 排序文件：文件夹在前，文件在后
  const sortedFiles = [...files].sort((a, b) => {
    if (a.is_dir && !b.is_dir) return -1;
    if (!a.is_dir && b.is_dir) return 1;
    return a.name.localeCompare(b.name);
  });

  // 处理文件/文件夹点击
  const handleItemClick = (file) => {
    if (file.is_dir) {
      onNavigate(file.path);
    } else {
      setSelectedFile(file);
      if (onPreviewFile) onPreviewFile(file);
    }
  };

  // 处理双击
  const handleItemDoubleClick = (file) => {
    if (file.is_dir) {
      onNavigate(file.path);
    }
  };

  // 面包屑导航
  const handleBreadcrumbClick = (index) => {
    if (index === -1) {
      onNavigate('');
    } else {
      const newPath = pathParts.slice(0, index + 1).join('/');
      onNavigate(newPath);
    }
  };

  // 右键菜单
  const handleContextMenu = (e, file) => {
    e.preventDefault();
    setContextMenu({ x: e.clientX, y: e.clientY, file });
  };

  // 关闭右键菜单
  const closeContextMenu = () => {
    setContextMenu(null);
    setActionMenu(null);
  };

  // 复制路径
  const handleCopyPath = (file) => {
    const fullPath = `workspace/${file.path}`;
    navigator.clipboard.writeText(fullPath).then(() => {
      Toast.success('路径已复制');
    }).catch(() => {
      Toast.error('复制失败');
    });
  };

  // 打开操作菜单
  const handleActionClick = (e, file) => {
    e.stopPropagation();
    const rect = e.currentTarget.getBoundingClientRect();
    setActionMenu({ x: rect.right - 160, y: rect.bottom + 4, file });
  };

  // 下载文件（文件夹会自动打包为zip）
  const handleDownload = (file) => {
    const url = api.getDownloadUrl(userId, file.path);
    // 创建临时链接触发下载，不弹出新窗口
    const link = document.createElement('a');
    link.href = url;
    link.download = file.is_dir ? `${file.name}.zip` : file.name;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  // 删除文件 (REST API)
  const handleDelete = (file) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除 "${file.name}" 吗？${file.is_dir ? '文件夹内的所有内容也将被删除。' : ''}`,
      okType: 'danger',
      onOk: async () => {
        try {
          await api.deleteFile(userId, file.path);
          Toast.success('删除成功');
          onRefresh();
        } catch (error) {
          Toast.error('删除失败: ' + error.message);
        }
      }
    });
  };

  // 重命名文件 - 开始内联编辑
  const handleRename = (file) => {
    setRenamingFile(file);
    setRenameValue(file.name);
    setTimeout(() => renameInputRef.current?.focus(), 100);
  };

  // 确认重命名 (REST API)
  const confirmRename = async () => {
    if (!renamingFile || !renameValue.trim()) {
      setRenamingFile(null);
      return;
    }
    if (renameValue !== renamingFile.name) {
      try {
        await api.renameFile(userId, renamingFile.path, renameValue);
        Toast.success('重命名成功');
        onRefresh();
      } catch (error) {
        Toast.error('重命名失败: ' + error.message);
      }
    }
    setRenamingFile(null);
  };

  // 重命名输入框按键处理
  const handleRenameKeyDown = (e) => {
    if (e.key === 'Enter') {
      confirmRename();
    } else if (e.key === 'Escape') {
      setRenamingFile(null);
    }
  };

  // 创建文件夹
  const handleCreateFolder = () => {
    setIsCreatingFolder(true);
    setNewFolderName('');
    setTimeout(() => newFolderInputRef.current?.focus(), 100);
  };

  // 确认创建文件夹 (REST API)
  const confirmCreateFolder = async () => {
    if (!newFolderName.trim()) {
      setIsCreatingFolder(false);
      return;
    }

    const folderPath = currentPath ? `${currentPath}/${newFolderName}` : newFolderName;
    try {
      await api.createFolder(userId, folderPath);
      Toast.success('文件夹创建成功');
      onRefresh();
    } catch (error) {
      Toast.error('创建失败: ' + error.message);
    }
    setIsCreatingFolder(false);
    setNewFolderName('');
  };

  // 处理新建文件夹输入框按键
  const handleNewFolderKeyDown = (e) => {
    if (e.key === 'Enter') {
      confirmCreateFolder();
    } else if (e.key === 'Escape') {
      setIsCreatingFolder(false);
    }
  };

  // 上传文件 (REST API)
  const uploadFiles = async (fileList) => {
    if (!fileList || fileList.length === 0) return;

    setUploading(true);

    for (const file of fileList) {
      try {
        await api.uploadFile(userId, currentPath, file);
        Toast.success(`上传成功: ${file.name}`);
      } catch (error) {
        Toast.error(`上传失败: ${file.name}`);
      }
    }

    setUploading(false);
    onRefresh();
  };

  // 处理文件选择
  const handleFileSelect = (e) => {
    uploadFiles(e.target.files);
    e.target.value = '';
  };

  // 拖拽事件处理
  const handleDragEnter = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.currentTarget.contains(e.relatedTarget)) return;
    setIsDragging(false);
  };

  const handleDragOver = (e) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleDrop = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);

    const droppedFiles = e.dataTransfer.files;
    if (droppedFiles.length > 0) {
      uploadFiles(droppedFiles);
    }
  };

  return (
    <div className="file-explorer">
      {/* 工具栏 */}
      <div className="file-toolbar">
        <span className="file-toolbar-title">文件</span>
        <Button
          icon={<IconUpload />}
          size="small"
          type="tertiary"
          theme="borderless"
          onClick={() => fileInputRef.current?.click()}
          disabled={uploading}
        />
        <Button
          icon={<IconPlus />}
          size="small"
          type="tertiary"
          theme="borderless"
          onClick={handleCreateFolder}
        />
        <Button
          icon={<IconRefresh />}
          size="small"
          type="tertiary"
          theme="borderless"
          onClick={onRefresh}
          disabled={loading}
        />
      </div>

      {/* 面包屑导航 */}
      <div className="file-breadcrumb">
        <Breadcrumb>
          <Breadcrumb.Item onClick={() => handleBreadcrumbClick(-1)}>
            workspace
          </Breadcrumb.Item>
          {pathParts.map((part, index) => (
            <Breadcrumb.Item
              key={index}
              onClick={() => handleBreadcrumbClick(index)}
            >
              {part}
            </Breadcrumb.Item>
          ))}
        </Breadcrumb>
      </div>

      {/* 上传进度 */}
      {uploading && (
        <div className="upload-progress">
          <div className="upload-progress-item">
            <Spin size="small" />
            <span>正在上传...</span>
          </div>
        </div>
      )}

      {/* 拖拽区域 */}
      <div
        className={`file-drop-zone ${isDragging ? 'dragging' : ''}`}
        onDragEnter={handleDragEnter}
        onDragLeave={handleDragLeave}
        onDragOver={handleDragOver}
        onDrop={handleDrop}
      >
        {/* 文件列表容器 */}
        <div className="file-list-container">
          {/* 列表头部 */}
          <div className="file-list-header">
            <span>名称</span>
            <span style={{ textAlign: 'right' }}>大小</span>
            <span style={{ textAlign: 'right' }}>修改时间</span>
            <span></span>
          </div>

          {/* 文件列表 */}
          <div className="file-list">
            {/* 新建文件夹输入 */}
            {isCreatingFolder && (
              <div className="new-folder-input">
                <div className="file-item-name">
                  <IconFolder className="file-item-icon folder" />
                  <input
                    ref={newFolderInputRef}
                    type="text"
                    value={newFolderName}
                    onChange={(e) => setNewFolderName(e.target.value)}
                    onKeyDown={handleNewFolderKeyDown}
                    onBlur={confirmCreateFolder}
                    placeholder="新建文件夹"
                  />
                </div>
                <span></span>
                <span></span>
              </div>
            )}

            {loading ? (
              <div className="file-loading">
                <Spin size="small" />
                <Text type="tertiary">加载中...</Text>
              </div>
            ) : sortedFiles.length === 0 && !isCreatingFolder ? (
              <div className="file-empty">
                <div className="file-empty-actions">
                  <button className="file-empty-btn" onClick={() => fileInputRef.current?.click()}>
                    <IconUpload />
                    <span>上传文件</span>
                  </button>
                  <button className="file-empty-btn" onClick={handleCreateFolder}>
                    <IconPlus />
                    <span>新建文件夹</span>
                  </button>
                </div>
              </div>
            ) : (
              sortedFiles.map((file) => {
                const fileIconInfo = !file.is_dir ? getFileIcon(file.name) : null;
                const FileIcon = fileIconInfo?.icon || IconFile;
                const iconColor = fileIconInfo?.color || 'var(--semi-color-text-2)';
                const isRenaming = renamingFile?.path === file.path;

                return (
                  <div
                    key={file.path}
                    className={`file-item ${selectedFile?.path === file.path ? 'selected' : ''}`}
                    onClick={() => !isRenaming && handleItemClick(file)}
                    onDoubleClick={() => !isRenaming && handleItemDoubleClick(file)}
                    onContextMenu={(e) => handleContextMenu(e, file)}
                  >
                    <div className="file-item-name">
                      {file.is_dir ? (
                        <IconFolder className="file-item-icon folder" />
                      ) : (
                        <FileIcon className="file-item-icon" style={{ color: iconColor }} />
                      )}
                      {isRenaming ? (
                        <input
                          ref={renameInputRef}
                          type="text"
                          className="file-rename-input"
                          value={renameValue}
                          onChange={(e) => setRenameValue(e.target.value)}
                          onKeyDown={handleRenameKeyDown}
                          onBlur={confirmRename}
                          onClick={(e) => e.stopPropagation()}
                        />
                      ) : (
                        <span className="file-item-text" title={file.name}>
                          {file.name}
                        </span>
                      )}
                    </div>
                    <span className="file-item-size">
                      {file.is_dir ? '-' : formatSize(file.size)}
                    </span>
                    <span className="file-item-time">
                      {formatTime(file.mod_time)}
                    </span>
                    <div className="file-item-actions">
                      <button
                        className="file-action-btn"
                        onClick={(e) => handleActionClick(e, file)}
                      >
                        <IconMore />
                      </button>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </div>

      {/* 隐藏的文件输入 */}
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="file-input-hidden"
        onChange={handleFileSelect}
      />
      <input
        ref={folderInputRef}
        type="file"
        webkitdirectory=""
        directory=""
        multiple
        className="file-input-hidden"
        onChange={handleFileSelect}
      />

      {/* 右键菜单 */}
      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          file={contextMenu.file}
          onClose={closeContextMenu}
          onDownload={handleDownload}
          onDelete={handleDelete}
          onRename={handleRename}
          onCopyPath={handleCopyPath}
        />
      )}

      {/* 操作菜单 */}
      {actionMenu && (
        <ContextMenu
          x={actionMenu.x}
          y={actionMenu.y}
          file={actionMenu.file}
          onClose={closeContextMenu}
          onDownload={handleDownload}
          onDelete={handleDelete}
          onRename={handleRename}
          onCopyPath={handleCopyPath}
        />
      )}
    </div>
  );
};

export default FileExplorer;
