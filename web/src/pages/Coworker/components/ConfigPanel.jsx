/*
Copyright (C) 2025 QuantumNous
*/

import React, { useRef, useEffect, useState } from 'react';
import { Button, Typography, Toast, TextArea, Input, Collapsible } from '@douyinfe/semi-ui';
import { IconUpload, IconDownload, IconSave, IconRefresh, IconChevronDown, IconChevronUp } from '@douyinfe/semi-icons';
import * as api from '../services/api';
import './ConfigPanel.css';

const { Text, Title } = Typography;

const ConfigPanel = ({ userId, content, loading, onContentChange, onLoadingChange }) => {
  const fileInputRef = useRef(null);

  // 用户信息状态
  const [userInfo, setUserInfo] = useState({
    userName: '',
    coworkerName: '',
    phone: '',
    email: ''
  });
  const [userInfoLoading, setUserInfoLoading] = useState(false);
  const [userInfoExpanded, setUserInfoExpanded] = useState(true);
  const [promptExpanded, setPromptExpanded] = useState(true);

  // 加载配置文件 (REST API)
  const loadConfig = async () => {
    onLoadingChange(true);
    try {
      const data = await api.getConfig(userId);
      onContentChange(data.content || '');
    } catch (error) {
      Toast.error('加载配置失败: ' + error.message);
    } finally {
      onLoadingChange(false);
    }
  };

  // 加载用户信息
  const loadUserInfo = async () => {
    setUserInfoLoading(true);
    try {
      const data = await api.getUserInfo(userId);
      setUserInfo({
        userName: data.user_name || '',
        coworkerName: data.coworker_name || '',
        phone: data.phone || '',
        email: data.email || ''
      });
    } catch (error) {
      // 如果没有用户信息，使用默认值
      console.log('No user info found, using defaults');
    } finally {
      setUserInfoLoading(false);
    }
  };

  // 保存配置文件 (REST API)
  const saveConfig = async () => {
    onLoadingChange(true);
    try {
      await api.saveConfig(userId, content);
      Toast.success('提示词配置已保存');
    } catch (error) {
      Toast.error('保存失败: ' + error.message);
    } finally {
      onLoadingChange(false);
    }
  };

  // 保存用户信息
  const saveUserInfo = async () => {
    setUserInfoLoading(true);
    try {
      await api.saveUserInfo(userId, userInfo);
      Toast.success('用户信息已保存');
    } catch (error) {
      Toast.error('保存失败: ' + error.message);
    } finally {
      setUserInfoLoading(false);
    }
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
      loadUserInfo();
    }
  }, [userId]);

  return (
    <div className="config-panel">
      {/* 用户信息配置 */}
      <div className="config-section">
        <div
          className="config-section-header"
          onClick={() => setUserInfoExpanded(!userInfoExpanded)}
        >
          <Title heading={6}>用户信息</Title>
          {userInfoExpanded ? <IconChevronUp /> : <IconChevronDown />}
        </div>

        {userInfoExpanded && (
          <div className="config-section-content">
            <div className="config-form">
              <div className="config-form-item">
                <Text size="small" type="secondary">用户称呼</Text>
                <Input
                  value={userInfo.userName}
                  onChange={(v) => setUserInfo({ ...userInfo, userName: v })}
                  placeholder="您希望 AI 如何称呼您"
                  size="small"
                />
              </div>
              <div className="config-form-item">
                <Text size="small" type="secondary">Coworker 称呼</Text>
                <Input
                  value={userInfo.coworkerName}
                  onChange={(v) => setUserInfo({ ...userInfo, coworkerName: v })}
                  placeholder="您希望如何称呼 AI 助手"
                  size="small"
                />
              </div>
              <div className="config-form-item">
                <Text size="small" type="secondary">手机号</Text>
                <Input
                  value={userInfo.phone}
                  onChange={(v) => setUserInfo({ ...userInfo, phone: v })}
                  placeholder="用于接收通知（可选）"
                  size="small"
                />
              </div>
              <div className="config-form-item">
                <Text size="small" type="secondary">邮箱</Text>
                <Input
                  value={userInfo.email}
                  onChange={(v) => setUserInfo({ ...userInfo, email: v })}
                  placeholder="用于接收通知（可选）"
                  size="small"
                />
              </div>
            </div>
            <Button
              icon={<IconSave />}
              theme="solid"
              onClick={saveUserInfo}
              loading={userInfoLoading}
              size="small"
              style={{ marginTop: 8 }}
            >
              保存用户信息
            </Button>
          </div>
        )}
      </div>

      {/* 系统提示词配置 */}
      <div className="config-section">
        <div
          className="config-section-header"
          onClick={() => setPromptExpanded(!promptExpanded)}
        >
          <Title heading={6}>系统提示词</Title>
          {promptExpanded ? <IconChevronUp /> : <IconChevronDown />}
        </div>

        {promptExpanded && (
          <div className="config-section-content">
            <Text type="tertiary" size="small">
              上传 COWORKER.md 文件自定义系统提示词
            </Text>

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
                autosize={{ minRows: 8, maxRows: 15 }}
              />
            </div>

            <Button
              icon={<IconSave />}
              theme="solid"
              onClick={saveConfig}
              loading={loading}
              size="small"
            >
              保存提示词
            </Button>
          </div>
        )}
      </div>
    </div>
  );
};

export default ConfigPanel;
