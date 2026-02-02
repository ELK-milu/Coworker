/*
Copyright (C) 2025 QuantumNous
内联任务卡片组件 - 在对话流中显示任务状态
*/

import React, { useState } from 'react';
import { Typography, Progress, Button, Tooltip } from '@douyinfe/semi-ui';
import { IconTick, IconPlay, IconChevronDown, IconChevronUp } from '@douyinfe/semi-icons';
import './InlineTaskCard.css';

const { Text } = Typography;

const statusConfig = {
  pending: { color: 'var(--semi-color-text-2)', label: '待处理' },
  in_progress: { color: 'var(--semi-color-primary)', label: '进行中' },
  completed: { color: 'var(--semi-color-success)', label: '已完成' },
};

const InlineTaskCard = ({ tasks = [], editable = false, onUpdateTask }) => {
  const [expanded, setExpanded] = useState(false);
  const [expandedTaskId, setExpandedTaskId] = useState(null);

  if (!tasks || tasks.length === 0) return null;

  const stats = {
    total: tasks.length,
    completed: tasks.filter(t => t.status === 'completed').length,
    inProgress: tasks.filter(t => t.status === 'in_progress').length,
  };

  const progress = stats.total > 0 ? (stats.completed / stats.total) * 100 : 0;

  // 按状态排序：进行中 > 待处理 > 已完成
  const sortedTasks = [...tasks].sort((a, b) => {
    const order = { in_progress: 0, pending: 1, completed: 2 };
    return (order[a.status] || 1) - (order[b.status] || 1);
  });

  const displayTasks = expanded ? sortedTasks : sortedTasks.slice(0, 3);

  const handleStatusToggle = (task, e) => {
    e.stopPropagation();
    if (!editable || !onUpdateTask) return;

    const nextStatus = {
      pending: 'in_progress',
      in_progress: 'completed',
      completed: 'pending',
    };
    onUpdateTask(task.id, { status: nextStatus[task.status] || 'pending' });
  };

  return (
    <div className="inline-task-card">
      <div className="inline-task-header">
        <span className="inline-task-title">任务进度</span>
        <Text type="tertiary" size="small">
          {stats.completed}/{stats.total}
        </Text>
      </div>
      <Progress percent={progress} showInfo={false} size="small" />
      <div className="inline-task-list">
        {displayTasks.map(task => (
          <div
            key={task.id}
            className={`inline-task-item ${task.status} ${editable ? 'editable' : ''}`}
            onClick={() => setExpandedTaskId(expandedTaskId === task.id ? null : task.id)}
          >
            <Tooltip content={editable ? '点击切换状态' : statusConfig[task.status]?.label}>
              <span
                className="inline-task-status"
                onClick={(e) => handleStatusToggle(task, e)}
              >
                {task.status === 'completed' && <IconTick size="extra-small" />}
                {task.status === 'in_progress' && <IconPlay size="extra-small" />}
                {task.status === 'pending' && <span className="task-dot" />}
              </span>
            </Tooltip>
            <div className="inline-task-content">
              <Text
                className="inline-task-subject"
                ellipsis={{ showTooltip: true }}
                style={{
                  textDecoration: task.status === 'completed' ? 'line-through' : 'none',
                  opacity: task.status === 'completed' ? 0.6 : 1
                }}
              >
                {task.subject}
              </Text>
              {expandedTaskId === task.id && task.description && (
                <Text className="inline-task-description" type="tertiary" size="small">
                  {task.description}
                </Text>
              )}
            </div>
          </div>
        ))}
      </div>
      {tasks.length > 3 && (
        <Button
          size="small"
          type="tertiary"
          theme="borderless"
          className="inline-task-expand-btn"
          icon={expanded ? <IconChevronUp /> : <IconChevronDown />}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? '收起' : `展开 ${tasks.length - 3} 个任务`}
        </Button>
      )}
    </div>
  );
};

export default InlineTaskCard;
