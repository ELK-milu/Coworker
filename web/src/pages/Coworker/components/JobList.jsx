/*
Copyright (C) 2025 QuantumNous
事项列表组件 - 简化定时任务
*/

import React, { useState, useRef } from 'react';
import { Typography, Spin, Button, Tooltip, Switch, Popconfirm, Select } from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconRefresh,
  IconDelete,
  IconPlay,
  IconEdit,
  IconHandle,
  IconTick,
  IconClose,
} from '@douyinfe/semi-icons';
import './JobList.css';

const { Text } = Typography;

// 调度类型选项
const scheduleTypeOptions = [
  { value: 'daily', label: '每天' },
  { value: 'weekly', label: '每周' },
  { value: 'interval', label: '间隔' },
  { value: 'once', label: '单次' },
  { value: 'cron', label: '高级 (Cron)' },
];

// 星期选项
const weekdayOptions = [
  { value: 0, label: '周日' },
  { value: 1, label: '周一' },
  { value: 2, label: '周二' },
  { value: 3, label: '周三' },
  { value: 4, label: '周四' },
  { value: 5, label: '周五' },
  { value: 6, label: '周六' },
];

// 调度类型转换为可读文本
const scheduleToReadable = (job) => {
  if (!job) return '';

  switch (job.schedule_type) {
    case 'once':
      if (job.run_at) {
        const date = new Date(job.run_at);
        return `单次 ${date.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}`;
      }
      return '单次';

    case 'daily':
      return `每天 ${job.time || '00:00'}`;

    case 'weekly':
      const days = (job.weekdays || []).map(d => weekdayOptions.find(w => w.value === d)?.label || d).join('、');
      return `每周${days} ${job.time || '00:00'}`;

    case 'interval':
      const mins = job.interval_minutes || 0;
      if (mins >= 60) {
        return `每 ${Math.floor(mins / 60)} 小时`;
      }
      return `每 ${mins} 分钟`;

    case 'cron':
    default:
      // 兼容旧的 cron 表达式
      return cronToReadable(job.cron_expr);
  }
};

// Cron 表达式转换为可读文本（简易版本）
const cronToReadable = (cronExpr) => {
  if (!cronExpr) return '';

  const parts = cronExpr.trim().split(/\s+/);
  if (parts.length < 5) return cronExpr;

  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts;

  // 每分钟
  if (minute === '*' && hour === '*' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    return '每分钟';
  }

  // 每 N 分钟
  if (minute.startsWith('*/') && hour === '*') {
    const interval = minute.slice(2);
    return `每 ${interval} 分钟`;
  }

  // 每小时
  if (minute !== '*' && hour === '*' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    return `每小时第 ${minute} 分钟`;
  }

  // 每 N 小时
  if (hour.startsWith('*/')) {
    const interval = hour.slice(2);
    return `每 ${interval} 小时`;
  }

  // 每天特定时间
  if (minute !== '*' && hour !== '*' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    return `每天 ${hour.padStart(2, '0')}:${minute.padStart(2, '0')}`;
  }

  // 每周特定天
  if (dayOfWeek !== '*' && dayOfMonth === '*') {
    const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
    const dayName = days[parseInt(dayOfWeek)] || dayOfWeek;
    if (hour !== '*' && minute !== '*') {
      return `每${dayName} ${hour.padStart(2, '0')}:${minute.padStart(2, '0')}`;
    }
    return `每${dayName}`;
  }

  // 每月特定日
  if (dayOfMonth !== '*' && dayOfWeek === '*') {
    if (hour !== '*' && minute !== '*') {
      return `每月 ${dayOfMonth} 日 ${hour.padStart(2, '0')}:${minute.padStart(2, '0')}`;
    }
    return `每月 ${dayOfMonth} 日`;
  }

  return cronExpr;
};

// 格式化时间戳
const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  const date = new Date(timestamp * 1000);
  return date.toLocaleString('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

// 状态配置
const statusConfig = {
  idle: { color: 'var(--semi-color-text-2)', label: '空闲' },
  running: { color: 'var(--semi-color-primary)', label: '运行中' },
  failed: { color: 'var(--semi-color-danger)', label: '失败' },
};

const JobList = ({
  jobs = [],
  loading = false,
  onCreateJob,
  onUpdateJob,
  onDeleteJob,
  onRunJob,
  onRefresh,
  onReorder,
}) => {
  const [isCreating, setIsCreating] = useState(false);
  const [editingJob, setEditingJob] = useState(null);
  const [formData, setFormData] = useState({
    name: '',
    command: '',
    schedule_type: 'daily',
    time: '09:00',
    weekdays: [1], // 默认周一
    interval_minutes: 60,
    run_at: '',
    cron_expr: '',
  });
  const [expandedJob, setExpandedJob] = useState(null);
  const [draggedId, setDraggedId] = useState(null);
  const [dragOverId, setDragOverId] = useState(null);
  const dragNodeRef = useRef(null);

  // 重置表单
  const resetForm = () => {
    setFormData({
      name: '',
      command: '',
      schedule_type: 'daily',
      time: '09:00',
      weekdays: [1],
      interval_minutes: 60,
      run_at: '',
      cron_expr: '',
    });
    setIsCreating(false);
    setEditingJob(null);
  };

  // 处理创建/更新 Job
  const handleSubmit = () => {
    if (!formData.name.trim() || !formData.command.trim()) {
      return;
    }

    // 验证调度配置
    if (formData.schedule_type === 'cron' && !formData.cron_expr.trim()) {
      return;
    }

    if (editingJob) {
      onUpdateJob(editingJob.id, formData);
    } else {
      onCreateJob(formData);
    }
    resetForm();
  };

  // 处理编辑
  const handleEdit = (job, e) => {
    e.stopPropagation();
    setEditingJob(job);
    setFormData({
      name: job.name,
      command: job.command,
      schedule_type: job.schedule_type || 'cron',
      time: job.time || '09:00',
      weekdays: job.weekdays || [1],
      interval_minutes: job.interval_minutes || 60,
      run_at: job.run_at ? new Date(job.run_at).toISOString().slice(0, 16) : '',
      cron_expr: job.cron_expr || '',
    });
    setIsCreating(true);
  };

  // 处理启用/禁用
  const handleToggleEnabled = (job, checked, e) => {
    e.stopPropagation();
    onUpdateJob(job.id, { enabled: checked });
  };

  // 拖拽处理
  const handleDragStart = (e, jobId) => {
    setDraggedId(jobId);
    dragNodeRef.current = e.target;
    e.dataTransfer.effectAllowed = 'move';
  };

  const handleDragOver = (e, jobId) => {
    e.preventDefault();
    if (jobId !== draggedId) {
      setDragOverId(jobId);
    }
  };

  const handleDragEnd = () => {
    if (draggedId && dragOverId && draggedId !== dragOverId && onReorder) {
      const jobIds = jobs.map(j => j.id);
      const draggedIndex = jobIds.indexOf(draggedId);
      const targetIndex = jobIds.indexOf(dragOverId);

      jobIds.splice(draggedIndex, 1);
      jobIds.splice(targetIndex, 0, draggedId);

      onReorder(jobIds);
    }
    setDraggedId(null);
    setDragOverId(null);
    dragNodeRef.current = null;
  };

  const handleDragLeave = () => {
    setDragOverId(null);
  };

  // 渲染 Job 项
  const renderJobItem = (job) => {
    const config = statusConfig[job.status] || statusConfig.idle;
    const isExpanded = expandedJob === job.id;
    const isDragging = draggedId === job.id;
    const isDragOver = dragOverId === job.id;

    return (
      <div
        key={job.id}
        className={`job-item ${job.status} ${isDragging ? 'dragging' : ''} ${isDragOver ? 'drag-over' : ''} ${!job.enabled ? 'disabled' : ''}`}
        onClick={() => setExpandedJob(isExpanded ? null : job.id)}
        draggable={!!onReorder}
        onDragStart={(e) => handleDragStart(e, job.id)}
        onDragOver={(e) => handleDragOver(e, job.id)}
        onDragEnd={handleDragEnd}
        onDragLeave={handleDragLeave}
      >
        <div className="job-item-header">
          {onReorder && (
            <span className="job-drag-handle">
              <IconHandle size="small" />
            </span>
          )}
          <div className="job-status-indicator" style={{ backgroundColor: config.color }} />
          <div className="job-item-content">
            <Text
              className={`job-name ${!job.enabled ? 'disabled' : ''}`}
              ellipsis={{ showTooltip: true }}
            >
              {job.name}
            </Text>
            <Text className="job-cron" type="tertiary" size="small">
              {scheduleToReadable(job)}
            </Text>
          </div>
          <div className="job-item-actions" onClick={(e) => e.stopPropagation()}>
            <Switch
              size="small"
              checked={job.enabled}
              onChange={(checked, e) => handleToggleEnabled(job, checked, e)}
            />
            <Tooltip content="立即执行">
              <Button
                icon={<IconPlay />}
                size="small"
                type="tertiary"
                theme="borderless"
                disabled={!job.enabled || job.status === 'running'}
                onClick={(e) => {
                  e.stopPropagation();
                  onRunJob(job.id);
                }}
              />
            </Tooltip>
            <Tooltip content="编辑">
              <Button
                icon={<IconEdit />}
                size="small"
                type="tertiary"
                theme="borderless"
                onClick={(e) => handleEdit(job, e)}
              />
            </Tooltip>
            <Popconfirm
              title="确定删除此事项？"
              content="删除后无法恢复"
              onConfirm={(e) => {
                e.stopPropagation();
                onDeleteJob(job.id);
              }}
              onCancel={(e) => e.stopPropagation()}
            >
              <Tooltip content="删除">
                <Button
                  icon={<IconDelete />}
                  size="small"
                  type="tertiary"
                  theme="borderless"
                  className="job-delete-btn"
                  onClick={(e) => e.stopPropagation()}
                />
              </Tooltip>
            </Popconfirm>
          </div>
        </div>
        {isExpanded && (
          <div className="job-item-details">
            <div className="job-detail-row">
              <span className="job-detail-label">命令：</span>
              <Text className="job-detail-value" ellipsis={{ showTooltip: true }}>
                {job.command}
              </Text>
            </div>
            <div className="job-detail-row">
              <span className="job-detail-label">调度：</span>
              <Text className="job-detail-value" size="small">
                {scheduleToReadable(job)}
              </Text>
            </div>
            <div className="job-detail-row">
              <span className="job-detail-label">上次执行：</span>
              <Text className="job-detail-value" type="tertiary" size="small">
                {formatTime(job.last_run)}
              </Text>
            </div>
            <div className="job-detail-row">
              <span className="job-detail-label">下次执行：</span>
              <Text className="job-detail-value" type="tertiary" size="small">
                {job.enabled ? formatTime(job.next_run) : '-'}
              </Text>
            </div>
            {job.last_error && (
              <div className="job-detail-row error">
                <span className="job-detail-label">错误：</span>
                <Text className="job-detail-value" type="danger" size="small">
                  {job.last_error}
                </Text>
              </div>
            )}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="job-list-container">
      {/* 头部 */}
      <div className="job-list-header">
        <span className="job-list-title">定时事项</span>
        <div className="job-list-actions">
          <Button
            icon={<IconPlus />}
            size="small"
            type="tertiary"
            theme="borderless"
            onClick={() => {
              resetForm();
              setIsCreating(true);
            }}
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

      {/* 创建/编辑表单 */}
      {isCreating && (
        <div className="job-create-form">
          <input
            type="text"
            className="job-input"
            placeholder="事项名称"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            autoFocus
          />

          {/* 调度类型选择 */}
          <Select
            className="job-select"
            value={formData.schedule_type}
            onChange={(value) => setFormData({ ...formData, schedule_type: value })}
            optionList={scheduleTypeOptions}
            placeholder="选择调度类型"
          />

          {/* 根据调度类型显示不同的输入 */}
          {(formData.schedule_type === 'daily' || formData.schedule_type === 'weekly') && (
            <input
              type="time"
              className="job-input"
              value={formData.time}
              onChange={(e) => setFormData({ ...formData, time: e.target.value })}
            />
          )}

          {formData.schedule_type === 'weekly' && (
            <Select
              className="job-select"
              multiple
              value={formData.weekdays}
              onChange={(value) => setFormData({ ...formData, weekdays: value })}
              optionList={weekdayOptions}
              placeholder="选择星期"
            />
          )}

          {formData.schedule_type === 'interval' && (
            <div className="job-interval-input">
              <input
                type="number"
                className="job-input"
                min="1"
                value={formData.interval_minutes}
                onChange={(e) => setFormData({ ...formData, interval_minutes: parseInt(e.target.value) || 1 })}
              />
              <span className="job-interval-label">分钟</span>
            </div>
          )}

          {formData.schedule_type === 'once' && (
            <input
              type="datetime-local"
              className="job-input"
              value={formData.run_at}
              onChange={(e) => setFormData({ ...formData, run_at: e.target.value })}
            />
          )}

          {formData.schedule_type === 'cron' && (
            <>
              <input
                type="text"
                className="job-input"
                placeholder="Cron 表达式 (如: */5 * * * *)"
                value={formData.cron_expr}
                onChange={(e) => setFormData({ ...formData, cron_expr: e.target.value })}
              />
              {formData.cron_expr && (
                <Text className="job-cron-preview" type="tertiary" size="small">
                  {cronToReadable(formData.cron_expr)}
                </Text>
              )}
            </>
          )}

          <textarea
            className="job-textarea"
            placeholder="执行命令 (发送给 AI 的指令)"
            value={formData.command}
            onChange={(e) => setFormData({ ...formData, command: e.target.value })}
            rows={3}
          />
          <div className="job-create-actions">
            <Button size="small" onClick={resetForm}>取消</Button>
            <Button
              size="small"
              theme="solid"
              onClick={handleSubmit}
              disabled={!formData.name.trim() || !formData.cron_expr.trim() || !formData.command.trim()}
            >
              {editingJob ? '保存' : '创建'}
            </Button>
          </div>
        </div>
      )}

      {/* Job 列表 */}
      <div className="job-list">
        {loading ? (
          <div className="job-loading">
            <Spin size="small" />
            <Text type="tertiary">加载中...</Text>
          </div>
        ) : jobs.length === 0 ? (
          <div className="job-empty">
            <Text type="tertiary">暂无定时事项</Text>
            <Button
              size="small"
              icon={<IconPlus />}
              onClick={() => {
                resetForm();
                setIsCreating(true);
              }}
            >
              创建事项
            </Button>
          </div>
        ) : (
          jobs.sort((a, b) => a.order - b.order).map(renderJobItem)
        )}
      </div>
    </div>
  );
};

export default JobList;
