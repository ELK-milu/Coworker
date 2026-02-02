/*
Copyright (C) 2025 QuantumNous
*/

import React, { useRef, useEffect } from 'react';
import { Button, Typography, Toast, TextArea } from '@douyinfe/semi-ui';
import { IconUpload, IconDownload, IconSave, IconRefresh } from '@douyinfe/semi-icons';
import './ConfigPanel.css';

const { Text, Title } = Typography;

const ConfigPanel = ({ wsRef, userId, content, loading, onContentChange, onLoadingChange }) => {
  const fileInputRef = useRef(null);

  // 加载配置文件
  const loadConfig = () => {
    if (!wsRef?.current || wsRef.current.readyState !== WebSocket.OPEN) {
      Toast.error('WebSocket 未连接');
      return;
    }
    onLoadingChange(true);
    wsRef.current.send(JSON.stringify({
      type: 'load_config',
      payload: { user_id: userId }
    }));
  };

  // 保存配置文件
  const saveConfig = () => {
    if (!wsRef?.current || wsRef.current.readyState !== WebSocket.OPEN) {
      Toast.error('WebSocket 未连接');
      return;
    }
    onLoadingChange(true);
    wsRef.current.send(JSON.stringify({
      type: 'save_config',
      payload: { user_id: userId, content }
    }));
  };

  // 处理文件上传
  const handleUpload = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith('.md')) {
      Toast.error('请上传 .md 文件');
      return;
    }

    const reader = new FileReader();
    reader.onload = (event) => {
      onContentChange(event.target.result);
      Toast.success('文件已加载，点击保存生效');
    };
    reader.readAsText(file);
    e.target.value = '';
  };

  // 下载配置文件
  const handleDownload = () => {
    if (!content) {
      Toast.warning('没有内容可下载');
      return;
    }
    const blob = new Blob([content], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'COWORKER.md';
    a.click();
    URL.revokeObjectURL(url);
  };

  // 初始加载
  useEffect(() => {
    if (userId) {
      loadConfig();
    }
  }, [userId]);

  return (
    <div className="config-panel">
      <div className="config-header">
        <Title heading={6}>系统提示词配置</Title>
        <Text type="tertiary" size="small">
          上传 COWORKER.md 文件自定义系统提示词
        </Text>
      </div>

      <div className="config-actions">
        <input
          type="file"
          ref={fileInputRef}
          accept=".md"
          style={{ display: 'none' }}
          onChange={handleUpload}
        />
        <Button
          icon={<IconUpload />}
          onClick={() => fileInputRef.current?.click()}
          size="small"
        >
          上传
        </Button>
        <Button
          icon={<IconDownload />}
          onClick={handleDownload}
          size="small"
          disabled={!content}
        >
          下载
        </Button>
        <Button
          icon={<IconRefresh />}
          onClick={loadConfig}
          size="small"
          loading={loading}
        >
          刷新
        </Button>
      </div>

      <div className="config-editor">
        <TextArea
          value={content}
          onChange={onContentChange}
          placeholder="在此编辑系统提示词，或上传 COWORKER.md 文件..."
          autosize={{ minRows: 10, maxRows: 20 }}
        />
      </div>

      <div className="config-footer">
        <Button
          icon={<IconSave />}
          theme="solid"
          onClick={saveConfig}
          loading={loading}
          block
        >
          保存配置
        </Button>
      </div>
    </div>
  );
};

export default ConfigPanel;
