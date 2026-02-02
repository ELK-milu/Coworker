/*
Copyright (C) 2025 QuantumNous
任务列表组件 - Claude Code 风格
*/

import React, { useState, useRef } from 'react';
import { Typography, Spin, Button, Tooltip, Progress } from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconRefresh,
  IconTick,
  IconClose,
  IconPlay,
  IconDelete,
  IconHandle,
} from '@douyinfe/semi-icons';
import './TaskList.css';

const { Text } = Typography;

// 状态图标和颜色
const statusConfig = {
  pending: { icon: null, color: 'var(--semi-color-text-2)', label: '待处理' },
  in_progress: { icon: IconPlay, color: 'var(--semi-color-primary)', label: '进行中' },
  completed: { icon: IconTick, color: 'var(--semi-color-success)', label: '已完成' },
};

const TaskList = ({
  tasks = [],
  loading = false,
  onCreateTask,
  onUpdateTask,
  onRefresh,
  onReorder,
}) => {
  const [isCreating, setIsCreating] = useState(false);
  const [newTaskSubject, setNewTaskSubject] = useState('');
  const [newTaskDescription, setNewTaskDescription] = useState('');
  const [expandedTask, setExpandedTask] = useState(null);
  const [draggedId, setDraggedId] = useState(null);
  const [dragOverId, setDragOverId] = useState(null);
  const dragNodeRef = useRef(null);

  // 计算任务统计
  const stats = {
    total: tasks.length,
    completed: tasks.filter(t => t.status === 'completed').length,
    inProgress: tasks.filter(t => t.status === 'in_progress').length,
    pending: tasks.filter(t => t.status === 'pending').length,
  };

  const progress = stats.total > 0 ? (stats.completed / stats.total) * 100 : 0;

  // 处理创建任务
  const handleCreate = () => {
    if (!newTaskSubject.trim()) return;
    onCreateTask({
      subject: newTaskSubject,
      description: newTaskDescription,
      activeForm: newTaskSubject.replace(/^(\w)/, (m) => m.toLowerCase() + 'ing'),
    });
    setNewTaskSubject('');
    setNewTaskDescription('');
    setIsCreating(false);
  };

  // 处理状态切换
  const handleStatusChange = (task, newStatus) => {
    onUpdateTask(task.id, { status: newStatus });
  };

  // 处理删除任务
  const handleDelete = (task) => {
    onUpdateTask(task.id, { status: 'deleted' });
  };

  // 拖拽处理
  const handleDragStart = (e, taskId) => {
    setDraggedId(taskId);
    dragNodeRef.current = e.target;
    e.dataTransfer.effectAllowed = 'move';
  };

  const handleDragOver = (e, taskId) => {
    e.preventDefault();
    if (taskId !== draggedId) {
      setDragOverId(taskId);
    }
  };

  const handleDragEnd = () => {
    if (draggedId && dragOverId && draggedId !== dragOverId && onReorder) {
      // 计算新顺序
      const taskIds = tasks.map(t => t.id);
      const draggedIndex = taskIds.indexOf(draggedId);
      const targetIndex = taskIds.indexOf(dragOverId);

      // 移动元素
      taskIds.splice(draggedIndex, 1);
      taskIds.splice(targetIndex, 0, draggedId);

      onReorder(taskIds);
    }
    setDraggedId(null);
    setDragOverId(null);
    dragNodeRef.current = null;
  };

  const handleDragLeave = () => {
    setDragOverId(null);
  };

  // 渲染任务项
  const renderTaskItem = (task) => {
    const config = statusConfig[task.status] || statusConfig.pending;
    const isExpanded = expandedTask === task.id;
    const StatusIcon = config.icon;
    const isDragging = draggedId === task.id;
    const isDragOver = dragOverId === task.id;

    return (
      <div
        key={task.id}
        className={`task-item ${task.status} ${isDragging ? 'dragging' : ''} ${isDragOver ? 'drag-over' : ''}`}
        onClick={() => setExpandedTask(isExpanded ? null : task.id)}
        draggable={!!onReorder}
        onDragStart={(e) => handleDragStart(e, task.id)}
        onDragOver={(e) => handleDragOver(e, task.id)}
        onDragEnd={handleDragEnd}
        onDragLeave={handleDragLeave}
      >
        <div className="task-item-header">
          {onReorder && (
            <span className="task-drag-handle">
              <IconHandle size="small" />
            </span>
          )}
          <div className="task-status-indicator" style={{ backgroundColor: config.color }}>
            {StatusIcon && <StatusIcon size="small" />}
            {!StatusIcon && <span className="task-number">{task.id}</span>}
          </div>
          <div className="task-item-content">
            <Text
              className={`task-subject ${task.status === 'completed' ? 'completed' : ''}`}
              ellipsis={{ showTooltip: true }}
            >
              {task.subject}
            </Text>
            {task.status === 'in_progress' && task.activeForm && (
              <Text className="task-active-form" type="tertiary" size="small">
                {task.activeForm}...
              </Text>
            )}
          </div>
          <div className="task-item-actions" onClick={(e) => e.stopPropagation()}>
            {task.status === 'pending' && (
              <Tooltip content="开始">
                <Button
                  icon={<IconPlay />}
                  size="small"
                  type="tertiary"
                  theme="borderless"
                  onClick={() => handleStatusChange(task, 'in_progress')}
                />
              </Tooltip>
            )}
            {task.status === 'in_progress' && (
              <Tooltip content="完成">
                <Button
                  icon={<IconTick />}
                  size="small"
                  type="tertiary"
                  theme="borderless"
                  onClick={() => handleStatusChange(task, 'completed')}
                />
              </Tooltip>
            )}
            {task.status === 'completed' && (
              <Tooltip content="重新打开">
                <Button
                  icon={<IconRefresh />}
                  size="small"
                  type="tertiary"
                  theme="borderless"
                  onClick={() => handleStatusChange(task, 'pending')}
                />
              </Tooltip>
            )}
            <Tooltip content="删除">
              <Button
                icon={<IconDelete />}
                size="small"
                type="tertiary"
                theme="borderless"
                className="task-delete-btn"
                onClick={() => handleDelete(task)}
              />
            </Tooltip>
          </div>
        </div>
        {isExpanded && task.description && (
          <div className="task-item-description">
            <Text type="tertiary" size="small">{task.description}</Text>
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="task-list-container">
      {/* 头部 */}
      <div className="task-list-header">
        <span className="task-list-title">任务列表</span>
        <div className="task-list-actions">
          <Button
            icon={<IconPlus />}
            size="small"
            type="tertiary"
            theme="borderless"
            onClick={() => setIsCreating(true)}
          />
          <Button
            icon={<IconRefresh />}
            size="small"
            type="tertiary"
            theme="borderless"
            onClick={onRefresh}
            disabled={loading}
          />
        </div>
      </div>

      {/* 进度条 */}
      {stats.total > 0 && (
        <div className="task-progress">
          <Progress percent={progress} showInfo={false} size="small" />
          <Text type="tertiary" size="small">
            {stats.completed}/{stats.total} 已完成
          </Text>
        </div>
      )}

      {/* 新建任务输入 */}
      {isCreating && (
        <div className="task-create-form">
          <input
            type="text"
            className="task-input"
            placeholder="任务标题"
            value={newTaskSubject}
            onChange={(e) => setNewTaskSubject(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            autoFocus
          />
          <textarea
            className="task-textarea"
            placeholder="任务描述（可选）"
            value={newTaskDescription}
            onChange={(e) => setNewTaskDescription(e.target.value)}
            rows={2}
          />
          <div className="task-create-actions">
            <Button size="small" onClick={() => setIsCreating(false)}>取消</Button>
            <Button size="small" theme="solid" onClick={handleCreate}>创建</Button>
          </div>
        </div>
      )}

      {/* 任务列表 */}
      <div className="task-list">
        {loading ? (
          <div className="task-loading">
            <Spin size="small" />
            <Text type="tertiary">加载中...</Text>
          </div>
        ) : tasks.length === 0 ? (
          <div className="task-empty">
            <Text type="tertiary">暂无任务</Text>
            <Button
              size="small"
              icon={<IconPlus />}
              onClick={() => setIsCreating(true)}
            >
              创建任务
            </Button>
          </div>
        ) : (
          <>
            {/* 进行中的任务 - 按 order 排序 */}
            {tasks.filter(t => t.status === 'in_progress').sort((a, b) => a.order - b.order).map(renderTaskItem)}
            {/* 待处理的任务 - 按 order 排序 */}
            {tasks.filter(t => t.status === 'pending').sort((a, b) => a.order - b.order).map(renderTaskItem)}
            {/* 已完成的任务 - 按 order 排序 */}
            {tasks.filter(t => t.status === 'completed').sort((a, b) => a.order - b.order).map(renderTaskItem)}
          </>
        )}
      </div>
    </div>
  );
};

export default TaskList;
