/*
Copyright (C) 2025 QuantumNous
*/

import React, { useState } from 'react';
import { Button, Typography, Tooltip, Popconfirm, Nav } from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconChevronLeft,
  IconChevronRight,
  IconDelete,
  IconHistory,
  IconFolder,
  IconList,
  IconSetting,
  IconClock,
} from '@douyinfe/semi-icons';
import FileExplorer from './FileExplorer';
import TaskList from './TaskList';
import JobList from './JobList';
import ConfigPanel from './ConfigPanel';
import './SessionSidebar.css';

const { Text } = Typography;

// 格式化时间
const formatTime = (timestamp) => {
  if (!timestamp) return '';
  const date = new Date(timestamp * 1000);
  const now = new Date();
  const diff = now - date;

  if (diff < 24 * 60 * 60 * 1000 && date.getDate() === now.getDate()) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  }
  if (diff < 48 * 60 * 60 * 1000) {
    return '昨天';
  }
  if (diff < 7 * 24 * 60 * 60 * 1000) {
    const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
    return days[date.getDay()];
  }
  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
};

const SessionSidebar = ({
  sessions = [],
  currentSessionId,
  onNewChat,
  onSelectSession,
  onDeleteSession,
  loading = false,
  // 文件相关
  files = [],
  currentPath = '',
  filesLoading = false,
  onNavigateFile,
  onRefreshFiles,
  // 任务相关
  tasks = [],
  tasksLoading = false,
  onCreateTask,
  onUpdateTask,
  onRefreshTasks,
  onReorderTasks,
  // 配置相关
  configContent = '',
  configLoading = false,
  onConfigChange,
  onConfigLoadingChange,
  // 事项相关
  jobs = [],
  jobsLoading = false,
  onCreateJob,
  onUpdateJob,
  onDeleteJob,
  onRunJob,
  onRefreshJobs,
  onReorderJobs,
  // 用户ID
  userId,
}) => {
  const [collapsed, setCollapsed] = useState(false);
  const [activeTab, setActiveTab] = useState('history');

  // 按时间分组会话
  const groupSessions = (sessions) => {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const yesterday = new Date(today - 24 * 60 * 60 * 1000);
    const weekAgo = new Date(today - 7 * 24 * 60 * 60 * 1000);

    const groups = { today: [], yesterday: [], thisWeek: [], earlier: [] };

    sessions.forEach(session => {
      const date = new Date(session.updated_at * 1000);
      if (date >= today) groups.today.push(session);
      else if (date >= yesterday) groups.yesterday.push(session);
      else if (date >= weekAgo) groups.thisWeek.push(session);
      else groups.earlier.push(session);
    });

    return groups;
  };

  const groupedSessions = groupSessions(sessions);

  // 渲染会话项
  const renderSessionItem = (session) => (
    <div
      key={session.id}
      className={`session-item ${currentSessionId === session.id ? 'active' : ''}`}
      onClick={() => onSelectSession(session.id)}
    >
      <div className="session-item-content">
        <Text className="session-title" ellipsis={{ showTooltip: true }}>
          {session.title || '新对话'}
        </Text>
        {!collapsed && (
          <Text className="session-meta" type="tertiary" size="small">
            {formatTime(session.updated_at)}
          </Text>
        )}
      </div>
      {!collapsed && (
        <Popconfirm
          title="确定删除此对话？"
          content="删除后无法恢复"
          onConfirm={(e) => {
            e.stopPropagation();
            onDeleteSession(session.id);
          }}
          onCancel={(e) => e.stopPropagation()}
        >
          <Button
            className="session-delete-btn"
            icon={<IconDelete />}
            size="small"
            type="tertiary"
            theme="borderless"
            onClick={(e) => e.stopPropagation()}
          />
        </Popconfirm>
      )}
    </div>
  );

  // 渲染会话分组
  const renderGroup = (title, sessions) => {
    if (sessions.length === 0) return null;
    return (
      <div className="session-group" key={title}>
        {!collapsed && <div className="session-group-title">{title}</div>}
        {sessions.map(renderSessionItem)}
      </div>
    );
  };

  // 渲染历史会话内容
  const renderHistoryContent = () => (
    <>
      <div className="new-chat-wrapper">
        <Tooltip content="新建对话" position="right" disabled={!collapsed}>
          <Button
            icon={<IconPlus />}
            theme="solid"
            className="new-chat-btn"
            onClick={onNewChat}
          >
            {!collapsed && '新建对话'}
          </Button>
        </Tooltip>
      </div>
      <div className="session-list">
        {loading ? (
          <div className="session-loading">
            <Text type="tertiary">加载中...</Text>
          </div>
        ) : sessions.length === 0 ? (
          <div className="session-empty">
            {!collapsed && <Text type="tertiary">暂无历史对话</Text>}
          </div>
        ) : (
          <>
            {renderGroup('今天', groupedSessions.today)}
            {renderGroup('昨天', groupedSessions.yesterday)}
            {renderGroup('本周', groupedSessions.thisWeek)}
            {renderGroup('更早', groupedSessions.earlier)}
          </>
        )}
      </div>
    </>
  );

  return (
    <div className={`session-sidebar ${collapsed ? 'collapsed' : ''}`}>
      {/* 头部导航 */}
      <div className="sidebar-header">
        {!collapsed && (
          <div className="sidebar-tabs">
            <button
              type="button"
              className={`tab-btn ${activeTab === 'history' ? 'active' : ''}`}
              onClick={() => setActiveTab('history')}
            >
              <IconHistory size="small" />
              <span>历史</span>
            </button>
            <button
              type="button"
              className={`tab-btn ${activeTab === 'tasks' ? 'active' : ''}`}
              onClick={() => setActiveTab('tasks')}
            >
              <IconList size="small" />
              <span>任务</span>
            </button>
            <button
              type="button"
              className={`tab-btn ${activeTab === 'files' ? 'active' : ''}`}
              onClick={() => setActiveTab('files')}
            >
              <IconFolder size="small" />
              <span>文件</span>
            </button>
            <button
              type="button"
              className={`tab-btn ${activeTab === 'config' ? 'active' : ''}`}
              onClick={() => setActiveTab('config')}
            >
              <IconSetting size="small" />
              <span>配置</span>
            </button>
            <button
              type="button"
              className={`tab-btn ${activeTab === 'jobs' ? 'active' : ''}`}
              onClick={() => setActiveTab('jobs')}
            >
              <IconClock size="small" />
              <span>事项</span>
            </button>
          </div>
        )}
        <Button
          icon={collapsed ? <IconChevronRight /> : <IconChevronLeft />}
          type="tertiary"
          theme="borderless"
          onClick={() => setCollapsed(!collapsed)}
          className="collapse-btn"
        />
      </div>

      {/* 内容区域 */}
      <div className="sidebar-content">
        {activeTab === 'history' ? (
          renderHistoryContent()
        ) : activeTab === 'tasks' ? (
          <TaskList
            tasks={tasks}
            loading={tasksLoading}
            onCreateTask={onCreateTask}
            onUpdateTask={onUpdateTask}
            onRefresh={onRefreshTasks}
            onReorder={onReorderTasks}
          />
        ) : activeTab === 'config' ? (
          <ConfigPanel
            userId={userId}
            content={configContent}
            loading={configLoading}
            onContentChange={onConfigChange}
            onLoadingChange={onConfigLoadingChange}
          />
        ) : activeTab === 'jobs' ? (
          <JobList
            jobs={jobs}
            loading={jobsLoading}
            onCreateJob={onCreateJob}
            onUpdateJob={onUpdateJob}
            onDeleteJob={onDeleteJob}
            onRunJob={onRunJob}
            onRefresh={onRefreshJobs}
            onReorder={onReorderJobs}
          />
        ) : (
          <FileExplorer
            files={files}
            currentPath={currentPath}
            loading={filesLoading}
            userId={userId}
            onNavigate={onNavigateFile}
            onRefresh={onRefreshFiles}
          />
        )}
      </div>
    </div>
  );
};

export default SessionSidebar;