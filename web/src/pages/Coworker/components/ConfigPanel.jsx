/*
Copyright (C) 2025 QuantumNous
*/

import React, { useRef, useEffect, useState } from 'react';
import { Button, Typography, Toast, TextArea, Input, Collapsible, Select } from '@douyinfe/semi-ui';
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
    email: '',
    apiTokenKey: '',
    apiTokenName: '',
  });
  const [userInfoLoading, setUserInfoLoading] = useState(false);
  const [userInfoExpanded, setUserInfoExpanded] = useState(true);
  const [promptExpanded, setPromptExpanded] = useState(true);

  // API 令牌状态
  const [tokens, setTokens] = useState([]);
  const [tokenExpanded, setTokenExpanded] = useState(true);
  const [tokenLoading, setTokenLoading] = useState(false);

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

  // 加载令牌列表
  const loadTokens = async () => {
    setTokenLoading(true);
    try {
      const items = await api.listTokens();
      setTokens(items);
      return items;
    } catch (error) {
      console.log('Failed to load tokens:', error.message);
      return [];
    } finally {
      setTokenLoading(false);
    }
  };

  // 处理令牌选择
  const handleTokenChange = async (value) => {
    if (value === '') {
      // 选择"系统默认"
      const updated = { ...userInfo, apiTokenKey: '', apiTokenName: '' };
      setUserInfo(updated);
      try {
        await api.saveUserInfo(userId, updated);
        Toast.success('已切换为系统默认');
      } catch (error) {
        Toast.error('保存失败: ' + error.message);
      }
    } else {
      const token = tokens.find(t => t.key === value);
      if (token) {
        const updated = { ...userInfo, apiTokenKey: token.key, apiTokenName: token.name };
        setUserInfo(updated);
        try {
          await api.saveUserInfo(userId, updated);
          Toast.success(`已选择令牌: ${token.name}`);
        } catch (error) {
          Toast.error('保存失败: ' + error.message);
        }
      }
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
      // 顺序加载：先用户信息，再令牌列表，最后自动选择
      (async () => {
        let info = null;
        try {
          const data = await api.getUserInfo(userId);
          info = {
            userName: data.user_name || '',
            coworkerName: data.coworker_name || '',
            phone: data.phone || '',
            email: data.email || '',
            apiTokenKey: data.api_token_key || '',
            apiTokenName: data.api_token_name || '',
          };
          setUserInfo(info);
        } catch (error) {
          console.log('No user info found, using defaults');
          info = userInfo;
        }

        const tokenList = await loadTokens();

        // 自动选择：用户未选令牌且有可用令牌时，自动选第一个
        if (!info.apiTokenKey && tokenList.length > 0) {
          const first = tokenList[0];
          const updated = { ...info, apiTokenKey: first.key, apiTokenName: first.name };
          setUserInfo(updated);
          try {
            await api.saveUserInfo(userId, updated);
            Toast.success(`已自动选择令牌: ${first.name}`);
          } catch (error) {
            console.log('Auto-select token save failed:', error.message);
          }
        }
      })();
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

      {/* API 令牌配置 */}
      <div className="config-section">
        <div
          className="config-section-header"
          onClick={() => setTokenExpanded(!tokenExpanded)}
        >
          <Title heading={6}>API 令牌</Title>
          {tokenExpanded ? <IconChevronUp /> : <IconChevronDown />}
        </div>

        {tokenExpanded && (
          <div className="config-section-content">
            <Text type="tertiary" size="small">
              选择 NewAPI 令牌后，API 调用将通过 Relay 代理，按令牌统计用量和计费
            </Text>
            <div style={{ marginTop: 8 }}>
              <Select
                value={userInfo.apiTokenKey || ''}
                onChange={handleTokenChange}
                loading={tokenLoading}
                style={{ width: '100%' }}
                size="small"
                placeholder="选择令牌..."
              >
                <Select.Option value="">不使用令牌（系统默认）</Select.Option>
                {tokens.map(token => {
                  const maskedKey = token.key ? `sk-***${token.key.slice(-4)}` : '';
                  const quota = token.remain_quota != null
                    ? (token.unlimited_quota ? '无限' : `余额 ${(token.remain_quota / 500000).toFixed(2)}`)
                    : '';
                  return (
                    <Select.Option key={token.key} value={token.key}>
                      {token.name} ({maskedKey}){quota ? ` - ${quota}` : ''}
                    </Select.Option>
                  );
                })}
              </Select>
            </div>
            <Button
              icon={<IconRefresh />}
              onClick={loadTokens}
              size="small"
              loading={tokenLoading}
              style={{ marginTop: 8 }}
            >
              刷新令牌列表
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
