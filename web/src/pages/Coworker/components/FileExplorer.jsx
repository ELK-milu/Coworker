/*
Copyright (C) 2025 QuantumNous
*/

import React, { useState, useEffect, useCallback } from 'react';
import { Typography, Spin, Button, Breadcrumb } from '@douyinfe/semi-ui';
import {
  IconFolder,
  IconFile,
  IconArrowUp,
  IconRefresh,
} from '@douyinfe/semi-icons';
import './FileExplorer.css';

const { Text } = Typography;

// 格式化文件大小
const formatSize = (bytes) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
};

// 格式化时间
const formatTime = (timestamp) => {
  if (!timestamp) return '';
  const date = new Date(timestamp * 1000);
  return date.toLocaleDateString('zh-CN', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const FileExplorer = ({
  files = [],
  currentPath = '',
  loading = false,
  onNavigate,
  onRefresh,
}) => {
  // 解析路径为面包屑
  const pathParts = currentPath ? currentPath.split('/').filter(Boolean) : [];

  // 处理文件/文件夹点击
  const handleItemClick = (file) => {
    if (file.is_dir) {
      onNavigate(file.path);
    }
  };

  // 返回上级目录
  const handleGoUp = () => {
    if (pathParts.length > 0) {
      const parentPath = pathParts.slice(0, -1).join('/');
      onNavigate(parentPath);
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

  return (
    <div className="file-explorer">
      {/* 工具栏 */}
      <div className="file-toolbar">
        <Button
          icon={<IconArrowUp />}
          size="small"
          type="tertiary"
          theme="borderless"
          disabled={pathParts.length === 0}
          onClick={handleGoUp}
        />
        <Button
          icon={<IconRefresh />}
          size="small"
          type="tertiary"
          theme="borderless"
          onClick={onRefresh}
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

      {/* 文件列表 */}
      <div className="file-list">
        {loading ? (
          <div className="file-loading">
            <Spin size="small" />
            <Text type="tertiary">加载中...</Text>
          </div>
        ) : files.length === 0 ? (
          <div className="file-empty">
            <Text type="tertiary">空文件夹</Text>
          </div>
        ) : (
          files.map((file) => (
            <div
              key={file.path}
              className="file-item"
              onClick={() => handleItemClick(file)}
            >
              <div className="file-icon">
                {file.is_dir ? (
                  <IconFolder style={{ color: 'var(--semi-color-warning)' }} />
                ) : (
                  <IconFile style={{ color: 'var(--semi-color-text-2)' }} />
                )}
              </div>
              <div className="file-info">
                <Text className="file-name" ellipsis={{ showTooltip: true }}>
                  {file.name}
                </Text>
                <Text className="file-meta" type="tertiary" size="small">
                  {file.is_dir ? '文件夹' : formatSize(file.size)}
                  {' · '}
                  {formatTime(file.mod_time)}
                </Text>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
};

export default FileExplorer;
