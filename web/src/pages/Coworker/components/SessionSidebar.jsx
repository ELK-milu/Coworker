/*
Copyright (C) 2025 QuantumNous
*/

import React, { useState } from 'react';
import { Button, Typography, Tooltip, Popconfirm } from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconChevronLeft,
  IconChevronRight,
  IconDelete,
  IconHistory
} from '@douyinfe/semi-icons';
import './SessionSidebar.css';

const { Text } = Typography;

// 格式化时间
const formatTime = (timestamp) => {
  if (!timestamp) return '';
  const date = new Date(timestamp * 1000);
  const now = new Date();
  const diff = now - date;

  // 今天
  if (diff < 24 * 60 * 60 * 1000 && date.getDate() === now.getDate()) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  }
  // 昨天
  if (diff < 48 * 60 * 60 * 1000) {
    return '昨天';
  }
  // 本周
  if (diff < 7 * 24 * 60 * 60 * 1000) {
    const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
    return days[date.getDay()];
  }
  // 更早
  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
};

const SessionSidebar = ({
  sessions = [],
  currentSessionId,
  onNewChat,
  onSelectSession,
  onDeleteSession,
  loading = false,
}) => {
  const [collapsed, setCollapsed] = useState(false);

  // 按时间分组会话
  const groupSessions = (sessions) => {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const yesterday = new Date(today - 24 * 60 * 60 * 1000);
    const weekAgo = new Date(today - 7 * 24 * 60 * 60 * 1000);

    const groups = {
      today: [],
      yesterday: [],
      thisWeek: [],
      earlier: [],
    };

    sessions.forEach(session => {
      const date = new Date(session.updated_at * 1000);
      if (date >= today) {
        groups.today.push(session);
      } else if (date >= yesterday) {
        groups.yesterday.push(session);
      } else if (date >= weekAgo) {
        groups.thisWeek.push(session);
      } else {
        groups.earlier.push(session);
      }
    });

    return groups;
  };

  const groupedSessions = groupSessions(sessions);

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

  const renderGroup = (title, sessions) => {
    if (sessions.length === 0) return null;
    return (
      <div className="session-group" key={title}>
        {!collapsed && <div className="session-group-title">{title}</div>}
        {sessions.map(renderSessionItem)}
      </div>
    );
  };

  return (
    <div className={`session-sidebar ${collapsed ? 'collapsed' : ''}`}>
      {/* 头部 */}
      <div className="sidebar-header">
        {!collapsed && (
          <div className="sidebar-title">
            <IconHistory style={{ marginRight: 8 }} />
            <span>历史对话</span>
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

      {/* 新建对话按钮 */}
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

      {/* 会话列表 */}
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
    </div>
  );
};

export default SessionSidebar;
