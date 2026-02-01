/*
Copyright (C) 2025 QuantumNous
内联任务卡片组件 - 在对话流中显示任务状态
*/

import React from 'react';
import { Typography, Progress } from '@douyinfe/semi-ui';
import { IconTick, IconPlay } from '@douyinfe/semi-icons';
import './InlineTaskCard.css';

const { Text } = Typography;

const statusConfig = {
  pending: { color: 'var(--semi-color-text-2)', label: '待处理' },
  in_progress: { color: 'var(--semi-color-primary)', label: '进行中' },
  completed: { color: 'var(--semi-color-success)', label: '已完成' },
};

const InlineTaskCard = ({ tasks = [] }) => {
  if (!tasks || tasks.length === 0) return null;

  const stats = {
    total: tasks.length,
    completed: tasks.filter(t => t.status === 'completed').length,
    inProgress: tasks.filter(t => t.status === 'in_progress').length,
  };

  const progress = stats.total > 0 ? (stats.completed / stats.total) * 100 : 0;

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
        {tasks.slice(0, 5).map(task => (
          <div key={task.id} className={`inline-task-item ${task.status}`}>
            <span className="inline-task-status">
              {task.status === 'completed' && <IconTick size="extra-small" />}
              {task.status === 'in_progress' && <IconPlay size="extra-small" />}
              {task.status === 'pending' && <span className="task-dot" />}
            </span>
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
          </div>
        ))}
        {tasks.length > 5 && (
          <Text type="tertiary" size="small">
            +{tasks.length - 5} 更多任务
          </Text>
        )}
      </div>
    </div>
  );
};

export default InlineTaskCard;
