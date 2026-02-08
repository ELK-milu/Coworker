import React, { useState, useEffect } from 'react';
import { Form, Select, Input, Button, Tag, Toast, Spin } from '@douyinfe/semi-ui';
import { IconSave, IconRefresh } from '@douyinfe/semi-icons';
import './ProfileSettings.css';

const ProfileSettings = ({ ws, userId }) => {
  const [profile, setProfile] = useState(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  // 表单状态
  const [languages, setLanguages] = useState([]);
  const [frameworks, setFrameworks] = useState([]);
  const [responseStyle, setResponseStyle] = useState('balanced');
  const [language, setLanguage] = useState('zh-CN');

  // 预定义选项
  const languageOptions = [
    'Python', 'Go', 'JavaScript', 'TypeScript', 'Rust', 'Java',
    'C++', 'C#', 'Ruby', 'PHP', 'Swift', 'Kotlin', 'Scala'
  ];

  const frameworkOptions = [
    'React', 'Vue.js', 'Angular', 'Next.js', 'Svelte',
    'Express', 'FastAPI', 'Django', 'Flask', 'Gin',
    'Spring', 'Rails', 'Laravel', 'Tailwind CSS', 'Bootstrap'
  ];

  const responseStyleOptions = [
    { value: 'concise', label: '简洁' },
    { value: 'balanced', label: '平衡' },
    { value: 'detailed', label: '详细' }
  ];

  const uiLanguageOptions = [
    { value: 'zh-CN', label: '中文' },
    { value: 'en-US', label: 'English' }
  ];

  // 加载用户画像
  const loadProfile = () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    setLoading(true);
    ws.send(JSON.stringify({
      type: 'profile_get',
      payload: { user_id: userId }
    }));
  };

  // 保存用户画像
  const saveProfile = () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    setSaving(true);
    ws.send(JSON.stringify({
      type: 'profile_update',
      payload: {
        user_id: userId,
        languages,
        frameworks,
        response_style: responseStyle,
        language
      }
    }));
  };

  // 处理 WebSocket 消息
  useEffect(() => {
    if (!ws) return;

    const handleMessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'profile_data') {
          const p = data.payload.profile;
          setProfile(p);
          setLanguages(p.languages || []);
          setFrameworks(p.frameworks || []);
          setResponseStyle(p.response_style || 'balanced');
          setLanguage(p.language || 'zh-CN');
          setLoading(false);
        } else if (data.type === 'profile_updated') {
          setSaving(false);
          if (data.payload.success) {
            Toast.success('设置已保存');
          }
        }
      } catch (e) {
        console.error('Parse message error:', e);
      }
    };

    ws.addEventListener('message', handleMessage);
    return () => ws.removeEventListener('message', handleMessage);
  }, [ws]);

  // 初始加载
  useEffect(() => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      loadProfile();
    }
  }, [ws, userId]);

  if (loading) {
    return (
      <div className="profile-loading">
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div className="profile-settings">
      <div className="profile-header">
        <h3>用户偏好设置</h3>
        <div className="profile-actions">
          <Button icon={<IconRefresh />} onClick={loadProfile} />
          <Button
            icon={<IconSave />}
            theme="solid"
            onClick={saveProfile}
            loading={saving}
          >
            保存
          </Button>
        </div>
      </div>

      <Form layout="vertical" className="profile-form">
        <Form.Slot label="常用编程语言">
          <Select
            multiple
            filter
            value={languages}
            onChange={setLanguages}
            placeholder="选择或输入语言"
            style={{ width: '100%' }}
          >
            {languageOptions.map(lang => (
              <Select.Option key={lang} value={lang}>{lang}</Select.Option>
            ))}
          </Select>
        </Form.Slot>

        <Form.Slot label="常用框架">
          <Select
            multiple
            filter
            value={frameworks}
            onChange={setFrameworks}
            placeholder="选择或输入框架"
            style={{ width: '100%' }}
          >
            {frameworkOptions.map(fw => (
              <Select.Option key={fw} value={fw}>{fw}</Select.Option>
            ))}
          </Select>
        </Form.Slot>

        <Form.Slot label="回复风格">
          <Select
            value={responseStyle}
            onChange={setResponseStyle}
            style={{ width: '100%' }}
          >
            {responseStyleOptions.map(opt => (
              <Select.Option key={opt.value} value={opt.value}>
                {opt.label}
              </Select.Option>
            ))}
          </Select>
        </Form.Slot>

        <Form.Slot label="交互语言">
          <Select
            value={language}
            onChange={setLanguage}
            style={{ width: '100%' }}
          >
            {uiLanguageOptions.map(opt => (
              <Select.Option key={opt.value} value={opt.value}>
                {opt.label}
              </Select.Option>
            ))}
          </Select>
        </Form.Slot>
      </Form>

      {profile && (
        <div className="profile-stats">
          <h4>使用统计</h4>
          <div className="stats-grid">
            <div className="stat-item">
              <span className="stat-value">{profile.total_sessions || 0}</span>
              <span className="stat-label">会话数</span>
            </div>
            <div className="stat-item">
              <span className="stat-value">{profile.total_messages || 0}</span>
              <span className="stat-label">消息数</span>
            </div>
          </div>

          {profile.top_tools && Object.keys(profile.top_tools).length > 0 && (
            <div className="tool-usage">
              <h5>工具使用</h5>
              <div className="tool-tags">
                {Object.entries(profile.top_tools)
                  .sort((a, b) => b[1] - a[1])
                  .slice(0, 10)
                  .map(([tool, count]) => (
                    <Tag key={tool} color="blue">
                      {tool}: {count}
                    </Tag>
                  ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ProfileSettings;
