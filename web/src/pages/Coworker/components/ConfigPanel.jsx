/*
Copyright (C) 2025 QuantumNous
*/

import React, { useRef, useEffect, useState, useCallback } from 'react';
import { Button, Typography, Toast, TextArea, Input, Select, Slider, Tag } from '@douyinfe/semi-ui';
import { IconUpload, IconDownload, IconSave, IconRefresh, IconChevronDown, IconChevronUp, IconDelete, IconGridStroked } from '@douyinfe/semi-icons';
import * as api from '../services/api';
import { API } from '../../../helpers/api';
import './ConfigPanel.css';

const { Text, Title } = Typography;

const ConfigPanel = ({ userId, content, loading, onContentChange, onLoadingChange }) => {
  const fileInputRef = useRef(null);

  // 用户信息状态
  const avatarInputRef = useRef(null);
  const [userInfo, setUserInfo] = useState({
    userName: '',
    coworkerName: '',
    assistantAvatar: '',
    phone: '',
    email: '',
    wechatId: '',
    apiTokenKey: '',
    apiTokenName: '',
    selectedModel: '',
    group: '',
    temperature: null,
    topP: null,
    frequencyPenalty: null,
    presencePenalty: null,
  });
  const [userInfoLoading, setUserInfoLoading] = useState(false);
  const [userInfoExpanded, setUserInfoExpanded] = useState(true);
  const [promptExpanded, setPromptExpanded] = useState(true);

  // 模型配置状态
  const [models, setModels] = useState([]);
  const [groups, setGroups] = useState([]);
  const [modelExpanded, setModelExpanded] = useState(true);
  const [modelLoading, setModelLoading] = useState(false);
  // 参数启用开关
  const [paramEnabled, setParamEnabled] = useState({
    temperature: false,
    topP: false,
    frequencyPenalty: false,
    presencePenalty: false,
  });

  // 扩展技能状态
  const [skillsExpanded, setSkillsExpanded] = useState(true);
  const [storeItems, setStoreItems] = useState([]);
  const [installedItems, setInstalledItems] = useState([]);

  // MCP 配置状态
  const [mcpJsons, setMcpJsons] = useState({}); // itemId → MCP JSON string
  const [mcpConfigExpanded, setMcpConfigExpanded] = useState({}); // itemId → bool
  const [mcpTestStatus, setMcpTestStatus] = useState({}); // itemId → 'testing'|'success'|'error'|null
  const [mcpTestMessage, setMcpTestMessage] = useState({}); // itemId → string

  const TYPE_LABELS = { skill: '技能', agent: 'Agent', mcp: 'MCP', plugin: '插件' };
  const TYPE_COLORS = { skill: 'blue', agent: 'purple', mcp: 'green', plugin: 'orange' };
  const DEFAULT_ICONS = { skill: '✨', agent: '🤖', mcp: '🔔', plugin: '🔌' };

  const loadStoreData = useCallback(async () => {
    try {
      const user = JSON.parse(localStorage.getItem('user') || '{}');
      const headers = user.token ? { Authorization: 'Bearer ' + user.token } : {};
      const [itemsRes, userRes] = await Promise.all([
        fetch('/coworker/store/items', { headers }).then(r => r.json()),
        userId ? fetch(`/coworker/store/user?user_id=${userId}`, { headers }).then(r => r.json()) : { installed: [] },
      ]);
      setStoreItems(itemsRes.items || []);
      setInstalledItems(userRes.installed || []);
    } catch (e) {
      console.log('Failed to load store data:', e.message);
    }
  }, [userId]);

  const handleUninstall = async (itemId) => {
    try {
      const user = JSON.parse(localStorage.getItem('user') || '{}');
      const res = await fetch(`/coworker/store/user/uninstall/${itemId}?user_id=${userId}`, {
        method: 'DELETE',
        headers: { ...(user.token ? { Authorization: 'Bearer ' + user.token } : {}) },
      });
      const data = await res.json();
      if (!data.success) throw new Error(data.error || '卸载失败');
      setInstalledItems(prev => prev.filter(id => id !== itemId));
      Toast.success('已卸载');
    } catch (e) {
      Toast.error('卸载失败: ' + e.message);
    }
  };

  const handleInstall = async (itemId) => {
    try {
      const user = JSON.parse(localStorage.getItem('user') || '{}');
      const res = await fetch(`/coworker/store/user/install/${itemId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...(user.token ? { Authorization: 'Bearer ' + user.token } : {}) },
        body: JSON.stringify({ user_id: userId }),
      });
      const data = await res.json();
      if (!data.success) throw new Error(data.error || '安装失败');
      setInstalledItems(prev => [...prev, itemId]);
      Toast.success('已安装');
    } catch (e) {
      Toast.error('安装失败: ' + e.message);
    }
  };

  // MCP 配置加载
  const loadMCPConfig = async (itemId) => {
    try {
      const data = await api.getUserMCPConfig(userId, itemId);
      setMcpJsons(prev => ({ ...prev, [itemId]: data.mcp_json || '' }));
    } catch (e) {
      console.log('Failed to load MCP config:', e.message);
    }
  };

  // MCP 配置保存
  const saveMCPConfig = async (itemId) => {
    try {
      await api.saveUserMCPConfig(userId, itemId, mcpJsons[itemId] || '');
      Toast.success('MCP 配置已保存');
    } catch (e) {
      Toast.error('保存失败: ' + e.message);
    }
  };

  // MCP 连接测试
  const testMCPConnection = async (itemId) => {
    const item = storeItems.find(s => s.id === itemId);
    if (!item) return;
    const mcpJson = mcpJsons[itemId] || '';
    if (!mcpJson) {
      Toast.warning('请先粘贴 MCP 配置 JSON');
      return;
    }

    setMcpTestStatus(prev => ({ ...prev, [itemId]: 'testing' }));
    setMcpTestMessage(prev => ({ ...prev, [itemId]: '' }));

    try {
      const data = await api.testMCPConnection(mcpJson, item.name || '', 15);
      if (data.success) {
        setMcpTestStatus(prev => ({ ...prev, [itemId]: 'success' }));
        setMcpTestMessage(prev => ({ ...prev, [itemId]: `${data.tool_count || 0} 个工具` }));
      } else {
        setMcpTestStatus(prev => ({ ...prev, [itemId]: 'error' }));
        setMcpTestMessage(prev => ({ ...prev, [itemId]: data.error || '连接失败' }));
      }
    } catch (e) {
      setMcpTestStatus(prev => ({ ...prev, [itemId]: 'error' }));
      setMcpTestMessage(prev => ({ ...prev, [itemId]: e.message }));
    }
  };

  // 切换 MCP 配置面板展开
  const toggleMCPConfig = (itemId) => {
    const newExpanded = !mcpConfigExpanded[itemId];
    setMcpConfigExpanded(prev => ({ ...prev, [itemId]: newExpanded }));
    if (newExpanded && mcpJsons[itemId] === undefined) {
      loadMCPConfig(itemId);
    }
  };

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

  // 加载模型列表
  const loadModels = useCallback(async () => {
    setModelLoading(true);
    try {
      const res = await API.get('/api/user/models');
      if (res.data?.success) {
        const opts = (res.data.data || []).map(m => ({ label: m, value: m }));
        setModels(opts);
      }
    } catch (e) {
      console.log('Failed to load models:', e.message);
    } finally {
      setModelLoading(false);
    }
  }, []);

  // 加载分组列表
  const loadGroups = useCallback(async () => {
    try {
      const res = await API.get('/api/user/self/groups');
      if (res.data?.success) {
        const opts = Object.entries(res.data.data || {}).map(([group, info]) => ({
          label: info.desc || group,
          value: group,
        }));
        setGroups(opts);
      }
    } catch (e) {
      console.log('Failed to load groups:', e.message);
    }
  }, []);

  // 防抖保存（Slider 拖动时避免频繁请求）
  const saveTimerRef = useRef(null);
  const autoSave = useCallback((updated) => {
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
    saveTimerRef.current = setTimeout(async () => {
      try {
        await api.saveUserInfo(userId, updated);
      } catch (e) {
        Toast.error('保存失败: ' + e.message);
      }
    }, 500);
  }, [userId]);

  // 处理模型选择
  const handleModelChange = (value) => {
    const updated = { ...userInfo, selectedModel: value };
    setUserInfo(updated);
    autoSave(updated);
  };

  // 处理分组选择
  const handleGroupChange = (value) => {
    const updated = { ...userInfo, group: value };
    setUserInfo(updated);
    autoSave(updated);
  };

  // 处理参数变更
  const handleParamChange = (key, value) => {
    const updated = { ...userInfo, [key]: value };
    setUserInfo(updated);
    autoSave(updated);
  };

  // 切换参数启用
  const toggleParam = (key) => {
    const enabled = !paramEnabled[key];
    setParamEnabled(prev => ({ ...prev, [key]: enabled }));
    if (!enabled) {
      handleParamChange(key, null);
    } else {
      const defaults = { temperature: 0.7, topP: 1, frequencyPenalty: 0, presencePenalty: 0 };
      handleParamChange(key, defaults[key]);
    }
  };

  // 处理头像上传
  const handleAvatarUpload = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      Toast.error('请上传图片文件');
      return;
    }
    const reader = new FileReader();
    reader.onload = (event) => {
      setUserInfo(prev => ({ ...prev, assistantAvatar: event.target.result }));
    };
    reader.readAsDataURL(file);
    e.target.value = '';
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
      loadModels();
      loadGroups();
      loadStoreData();
      (async () => {
        try {
          const data = await api.getUserInfo(userId);
          const info = {
            userName: data.user_name || '',
            coworkerName: data.coworker_name || '',
            assistantAvatar: data.assistant_avatar || '',
            phone: data.phone || '',
            email: data.email || '',
            wechatId: data.wechat_id || '',
            apiTokenKey: data.api_token_key || '',
            apiTokenName: data.api_token_name || '',
            selectedModel: data.selected_model || '',
            group: data.group || '',
            temperature: data.temperature ?? null,
            topP: data.top_p ?? null,
            frequencyPenalty: data.frequency_penalty ?? null,
            presencePenalty: data.presence_penalty ?? null,
          };
          setUserInfo(info);
          setParamEnabled({
            temperature: info.temperature != null,
            topP: info.topP != null,
            frequencyPenalty: info.frequencyPenalty != null,
            presencePenalty: info.presencePenalty != null,
          });
        } catch (error) {
          console.log('No user info found, using defaults');
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
                <Text size="small" type="secondary">助理头像</Text>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  {userInfo.assistantAvatar ? (
                    <img
                      src={userInfo.assistantAvatar}
                      alt="avatar"
                      style={{ width: 32, height: 32, borderRadius: '50%', objectFit: 'cover' }}
                    />
                  ) : (
                    <div style={{ width: 32, height: 32, borderRadius: '50%', background: '#6B4EE6', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: 14 }}>
                      {(userInfo.coworkerName || 'C')[0].toUpperCase()}
                    </div>
                  )}
                  <Button size="small" onClick={() => avatarInputRef.current?.click()}>上传头像</Button>
                  {userInfo.assistantAvatar && (
                    <Button size="small" type="danger" onClick={() => setUserInfo(prev => ({ ...prev, assistantAvatar: '' }))}>移除</Button>
                  )}
                  <input ref={avatarInputRef} type="file" accept="image/*" style={{ display: 'none' }} onChange={handleAvatarUpload} />
                </div>
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
              <div className="config-form-item">
                <Text size="small" type="secondary">微信号（需关注公众号）</Text>
                <Input
                  value={userInfo.wechatId}
                  onChange={(v) => setUserInfo({ ...userInfo, wechatId: v })}
                  placeholder="用于接收微信通知（可选）"
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

      {/* 模型配置 */}
      <div className="config-section">
        <div
          className="config-section-header"
          onClick={() => setModelExpanded(!modelExpanded)}
        >
          <Title heading={6}>模型配置</Title>
          {modelExpanded ? <IconChevronUp /> : <IconChevronDown />}
        </div>

        {modelExpanded && (
          <div className="config-section-content">
            {/* 分组选择 */}
            <div className="config-form-item">
              <Text size="small" type="secondary">分组</Text>
              <Select
                value={userInfo.group || undefined}
                onChange={handleGroupChange}
                style={{ width: '100%' }}
                size="small"
                placeholder="请选择分组..."
                filter
              >
                {groups.map(g => (
                  <Select.Option key={g.value} value={g.value}>{g.label}</Select.Option>
                ))}
              </Select>
            </div>

            {/* 模型选择 */}
            <div className="config-form-item">
              <Text size="small" type="secondary">模型</Text>
              <Select
                value={userInfo.selectedModel || undefined}
                onChange={handleModelChange}
                loading={modelLoading}
                style={{ width: '100%' }}
                size="small"
                placeholder="请选择模型..."
                filter
              >
                {models.map(m => (
                  <Select.Option key={m.value} value={m.value}>{m.label}</Select.Option>
                ))}
              </Select>
            </div>

            {/* Temperature */}
            <div className="config-param-item" style={{ opacity: paramEnabled.temperature ? 1 : 0.5 }}>
              <div className="config-param-header">
                <Text size="small" type="secondary">Temperature</Text>
                {paramEnabled.temperature && <Tag size="small">{userInfo.temperature ?? 0.7}</Tag>}
                <Button
                  size="small"
                  theme={paramEnabled.temperature ? 'solid' : 'borderless'}
                  type={paramEnabled.temperature ? 'primary' : 'tertiary'}
                  onClick={() => toggleParam('temperature')}
                  style={{ marginLeft: 'auto', width: 20, height: 20, padding: 0, minWidth: 0, borderRadius: '50%' }}
                >
                  {paramEnabled.temperature ? '✓' : '✕'}
                </Button>
              </div>
              <Slider
                step={0.1} min={0} max={1}
                value={userInfo.temperature ?? 0.7}
                onChange={(v) => handleParamChange('temperature', v)}
                disabled={!paramEnabled.temperature}
              />
            </div>

            {/* Top P */}
            <div className="config-param-item" style={{ opacity: paramEnabled.topP ? 1 : 0.5 }}>
              <div className="config-param-header">
                <Text size="small" type="secondary">Top P</Text>
                {paramEnabled.topP && <Tag size="small">{userInfo.topP ?? 1}</Tag>}
                <Button
                  size="small"
                  theme={paramEnabled.topP ? 'solid' : 'borderless'}
                  type={paramEnabled.topP ? 'primary' : 'tertiary'}
                  onClick={() => toggleParam('topP')}
                  style={{ marginLeft: 'auto', width: 20, height: 20, padding: 0, minWidth: 0, borderRadius: '50%' }}
                >
                  {paramEnabled.topP ? '✓' : '✕'}
                </Button>
              </div>
              <Slider
                step={0.1} min={0} max={1}
                value={userInfo.topP ?? 1}
                onChange={(v) => handleParamChange('topP', v)}
                disabled={!paramEnabled.topP}
              />
            </div>

            {/* Frequency Penalty */}
            <div className="config-param-item" style={{ opacity: paramEnabled.frequencyPenalty ? 1 : 0.5 }}>
              <div className="config-param-header">
                <Text size="small" type="secondary">Frequency Penalty</Text>
                {paramEnabled.frequencyPenalty && <Tag size="small">{userInfo.frequencyPenalty ?? 0}</Tag>}
                <Button
                  size="small"
                  theme={paramEnabled.frequencyPenalty ? 'solid' : 'borderless'}
                  type={paramEnabled.frequencyPenalty ? 'primary' : 'tertiary'}
                  onClick={() => toggleParam('frequencyPenalty')}
                  style={{ marginLeft: 'auto', width: 20, height: 20, padding: 0, minWidth: 0, borderRadius: '50%' }}
                >
                  {paramEnabled.frequencyPenalty ? '✓' : '✕'}
                </Button>
              </div>
              <Slider
                step={0.1} min={-2} max={2}
                value={userInfo.frequencyPenalty ?? 0}
                onChange={(v) => handleParamChange('frequencyPenalty', v)}
                disabled={!paramEnabled.frequencyPenalty}
              />
            </div>

            {/* Presence Penalty */}
            <div className="config-param-item" style={{ opacity: paramEnabled.presencePenalty ? 1 : 0.5 }}>
              <div className="config-param-header">
                <Text size="small" type="secondary">Presence Penalty</Text>
                {paramEnabled.presencePenalty && <Tag size="small">{userInfo.presencePenalty ?? 0}</Tag>}
                <Button
                  size="small"
                  theme={paramEnabled.presencePenalty ? 'solid' : 'borderless'}
                  type={paramEnabled.presencePenalty ? 'primary' : 'tertiary'}
                  onClick={() => toggleParam('presencePenalty')}
                  style={{ marginLeft: 'auto', width: 20, height: 20, padding: 0, minWidth: 0, borderRadius: '50%' }}
                >
                  {paramEnabled.presencePenalty ? '✓' : '✕'}
                </Button>
              </div>
              <Slider
                step={0.1} min={-2} max={2}
                value={userInfo.presencePenalty ?? 0}
                onChange={(v) => handleParamChange('presencePenalty', v)}
                disabled={!paramEnabled.presencePenalty}
              />
            </div>

            <Button
              icon={<IconRefresh />}
              onClick={() => { loadModels(); loadGroups(); }}
              size="small"
              loading={modelLoading}
              style={{ marginTop: 8 }}
            >
              刷新模型列表
            </Button>
          </div>
        )}
      </div>

      {/* 扩展技能 */}
      <div className="config-section">
        <div
          className="config-section-header"
          onClick={() => setSkillsExpanded(!skillsExpanded)}
        >
          <Title heading={6}>扩展技能</Title>
          {skillsExpanded ? <IconChevronUp /> : <IconChevronDown />}
        </div>

        {skillsExpanded && (
          <div className="config-section-content">
            {installedItems.length === 0 ? (
              <Text type="tertiary" size="small">暂无已安装的技能</Text>
            ) : (
              installedItems.map(itemId => {
                const item = storeItems.find(s => s.id === itemId);
                if (!item) return null;
                const isMCP = item.type === 'mcp';
                const isExpanded = mcpConfigExpanded[itemId];
                const testStatus = mcpTestStatus[itemId];
                const testMsg = mcpTestMessage[itemId];

                return (
                  <div key={itemId} style={{ borderBottom: '1px solid var(--semi-color-border)', paddingBottom: isMCP ? 8 : 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 0' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flex: 1, minWidth: 0 }}>
                        {item.icon && (item.icon.startsWith('data:image/') || item.icon.startsWith('http://') || item.icon.startsWith('https://'))
                          ? <img src={item.icon} alt="icon" style={{ width: 20, height: 20, borderRadius: 3, objectFit: 'cover' }} />
                          : <span style={{ fontSize: 14 }}>{item.icon || DEFAULT_ICONS[item.type] || '✨'}</span>
                        }
                        <Text size="small" ellipsis={{ showTooltip: true }} style={{ flex: 1 }}>{item.display_name || item.name}</Text>
                        <Tag color={TYPE_COLORS[item.type]} size="small">{TYPE_LABELS[item.type]}</Tag>
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                        {isMCP && (
                          <Button
                            size="small"
                            theme="borderless"
                            icon={isExpanded ? <IconChevronUp size="small" /> : <IconChevronDown size="small" />}
                            onClick={() => toggleMCPConfig(itemId)}
                            style={{ width: 24, height: 24, padding: 0, minWidth: 0 }}
                          />
                        )}
                        <Button
                          size="small"
                          type="danger"
                          theme="borderless"
                          icon={<IconDelete />}
                          onClick={() => handleUninstall(itemId)}
                        />
                      </div>
                    </div>

                    {/* MCP JSON 配置 */}
                    {isMCP && isExpanded && (
                      <div style={{ padding: '4px 0 8px 26px' }}>
                        {item.server_url && (
                          <div style={{ marginBottom: 6 }}>
                            <a href={item.server_url} target="_blank" rel="noopener noreferrer" style={{ fontSize: 12, color: 'var(--semi-color-link)' }}>
                              查看详情页
                            </a>
                          </div>
                        )}
                        <Text size="small" type="secondary" style={{ display: 'block', marginBottom: 4 }}>
                          MCP 配置 JSON
                        </Text>
                        <TextArea
                          size="small"
                          value={mcpJsons[itemId] || ''}
                          onChange={(v) => setMcpJsons(prev => ({ ...prev, [itemId]: v }))}
                          placeholder='{"mcpServers":{"name":{"type":"streamable_http","url":"...","headers":{...}}}}'
                          autosize={{ minRows: 3, maxRows: 8 }}
                          style={{ fontFamily: 'monospace', fontSize: 12 }}
                        />
                        <div style={{ display: 'flex', gap: 6, marginTop: 6, alignItems: 'center' }}>
                          <Button size="small" theme="solid" onClick={() => saveMCPConfig(itemId)}>
                            保存
                          </Button>
                          <Button size="small" onClick={() => testMCPConnection(itemId)} loading={testStatus === 'testing'}>
                            测试连接
                          </Button>
                          {testStatus === 'success' && (
                            <Tag color="green" size="small">已连接 ({testMsg})</Tag>
                          )}
                          {testStatus === 'error' && (
                            <Tag color="red" size="small">{testMsg}</Tag>
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                );
              })
            )}
            <Button
              icon={<IconGridStroked />}
              size="small"
              style={{ marginTop: 8 }}
              onClick={() => window.open('/skills', '_blank')}
            >
              技能商店
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
