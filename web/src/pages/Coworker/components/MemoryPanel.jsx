import React, { useState, useEffect, useCallback } from 'react';
import { Input, Button, Tag, Empty, Modal, Form, TextArea, Slider, Popconfirm, Toast } from '@douyinfe/semi-ui';
import { IconPlus, IconSearch, IconDelete, IconEdit, IconRefresh } from '@douyinfe/semi-icons';
import { listMemories, createMemory, updateMemory, deleteMemory, searchMemories } from '../services/api';
import './MemoryPanel.css';

const MemoryPanel = ({ userId }) => {
  const [memories, setMemories] = useState([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingMemory, setEditingMemory] = useState(null);
  const [formData, setFormData] = useState({
    tags: '',
    content: '',
    summary: '',
    weight: 0.5
  });

  // 加载记忆列表（REST API）
  const loadMemories = useCallback(async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const data = await listMemories(userId);
      // 后端已按 weight 降序排序
      setMemories(data.memories || []);
    } catch (e) {
      Toast.error('加载记忆失败: ' + e.message);
    } finally {
      setLoading(false);
    }
  }, [userId]);

  // 搜索记忆（REST API）
  const handleSearch = useCallback(async () => {
    if (!userId) return;
    if (!searchQuery.trim()) {
      loadMemories();
      return;
    }
    setLoading(true);
    try {
      const data = await searchMemories(userId, searchQuery);
      setMemories(data.memories || []);
    } catch (e) {
      Toast.error('搜索失败: ' + e.message);
    } finally {
      setLoading(false);
    }
  }, [userId, searchQuery, loadMemories]);

  // 创建/更新记忆（REST API）
  const saveMemory = async () => {
    const tags = formData.tags.split(',').map(t => t.trim()).filter(t => t);
    if (tags.length === 0) {
      Toast.error('请至少添加一个标签');
      return;
    }
    if (!formData.content.trim()) {
      Toast.error('请输入记忆内容');
      return;
    }

    try {
      if (editingMemory) {
        await updateMemory(userId, editingMemory.id, {
          tags,
          content: formData.content,
          summary: formData.summary || formData.content.substring(0, 50),
          weight: formData.weight,
        });
        Toast.success('记忆已更新');
      } else {
        await createMemory(userId, {
          tags,
          content: formData.content,
          summary: formData.summary || formData.content.substring(0, 50),
          weight: formData.weight,
        });
        Toast.success('记忆已创建');
      }
      setEditModalVisible(false);
      resetForm();
      loadMemories();
    } catch (e) {
      Toast.error('保存失败: ' + e.message);
    }
  };

  // 删除记忆（REST API）
  const handleDelete = async (memoryId) => {
    try {
      await deleteMemory(userId, memoryId);
      Toast.success('记忆已删除');
      loadMemories();
    } catch (e) {
      Toast.error('删除失败: ' + e.message);
    }
  };

  // 重置表单
  const resetForm = () => {
    setEditingMemory(null);
    setFormData({ tags: '', content: '', summary: '', weight: 0.5 });
  };

  // 打开编辑弹窗
  const openEditModal = (memory = null) => {
    if (memory) {
      setEditingMemory(memory);
      setFormData({
        tags: memory.tags.join(', '),
        content: memory.content,
        summary: memory.summary,
        weight: memory.weight
      });
    } else {
      resetForm();
    }
    setEditModalVisible(true);
  };

  // 初始加载
  useEffect(() => {
    loadMemories();
  }, [loadMemories]);

  // 按标签分组（组内已按 weight 降序）
  const groupedMemories = memories.reduce((acc, mem) => {
    const primaryTag = mem.tags[0] || 'other';
    if (!acc[primaryTag]) acc[primaryTag] = [];
    acc[primaryTag].push(mem);
    return acc;
  }, {});

  // 组按最高 weight 排序
  const sortedGroups = Object.entries(groupedMemories).sort((a, b) => {
    const maxA = Math.max(...a[1].map(m => m.weight || 0));
    const maxB = Math.max(...b[1].map(m => m.weight || 0));
    return maxB - maxA;
  });

  return (
    <div className="memory-panel">
      <div className="memory-header">
        <div className="memory-search">
          <Input
            prefix={<IconSearch />}
            placeholder="搜索记忆..."
            value={searchQuery}
            onChange={setSearchQuery}
            onEnterPress={handleSearch}
          />
        </div>
        <div className="memory-actions">
          <Button icon={<IconRefresh />} onClick={loadMemories} loading={loading} />
          <Button icon={<IconPlus />} theme="solid" onClick={() => openEditModal()}>
            添加
          </Button>
        </div>
      </div>

      <div className="memory-content">
        {memories.length === 0 ? (
          <Empty description="暂无记忆" />
        ) : (
          sortedGroups.map(([tag, mems]) => (
            <div key={tag} className="memory-group">
              <div className="memory-group-header">
                <Tag color="blue">{tag}</Tag>
                <span className="memory-count">{mems.length}</span>
              </div>
              {mems.map(mem => (
                <MemoryCard
                  key={mem.id}
                  memory={mem}
                  onEdit={() => openEditModal(mem)}
                  onDelete={() => handleDelete(mem.id)}
                />
              ))}
            </div>
          ))
        )}
      </div>

      <Modal
        title={editingMemory ? '编辑记忆' : '添加记忆'}
        visible={editModalVisible}
        onOk={saveMemory}
        onCancel={() => setEditModalVisible(false)}
        okText="保存"
        cancelText="取消"
      >
        <Form layout="vertical">
          <Form.Slot label="标签 (逗号分隔)">
            <Input
              value={formData.tags}
              onChange={(v) => setFormData({ ...formData, tags: v })}
              placeholder="tech, python, debugging"
            />
          </Form.Slot>
          <Form.Slot label="内容">
            <TextArea
              value={formData.content}
              onChange={(v) => setFormData({ ...formData, content: v })}
              placeholder="记忆内容..."
              rows={4}
            />
          </Form.Slot>
          <Form.Slot label="摘要 (可选)">
            <Input
              value={formData.summary}
              onChange={(v) => setFormData({ ...formData, summary: v })}
              placeholder="简短摘要"
            />
          </Form.Slot>
          <Form.Slot label={`重要性: ${formData.weight.toFixed(1)}`}>
            <Slider
              value={formData.weight}
              onChange={(v) => setFormData({ ...formData, weight: v })}
              min={0}
              max={1}
              step={0.1}
            />
          </Form.Slot>
        </Form>
      </Modal>
    </div>
  );
};

// 记忆卡片组件
const MemoryCard = ({ memory, onEdit, onDelete }) => {
  const [expanded, setExpanded] = useState(false);

  const formatDate = (timestamp) => {
    if (!timestamp) return '';
    return new Date(timestamp * 1000).toLocaleDateString();
  };

  return (
    <div className="memory-card">
      <div className="memory-card-header" onClick={() => setExpanded(!expanded)}>
        <div className="memory-summary">{memory.summary}</div>
        <div className="memory-weight">
          <div
            className="weight-bar"
            style={{ width: `${memory.weight * 100}%` }}
          />
        </div>
      </div>

      {expanded && (
        <div className="memory-card-body">
          <div className="memory-tags">
            {memory.tags.map(tag => (
              <Tag key={tag} size="small">{tag}</Tag>
            ))}
          </div>
          <div className="memory-text">{memory.content}</div>
          <div className="memory-meta">
            <span>访问: {memory.access_cnt || 0}次</span>
            <span>创建: {formatDate(memory.created_at)}</span>
          </div>
          <div className="memory-card-actions">
            <Button size="small" icon={<IconEdit />} onClick={onEdit}>编辑</Button>
            <Popconfirm title="确定删除?" onConfirm={onDelete}>
              <Button size="small" type="danger" icon={<IconDelete />}>删除</Button>
            </Popconfirm>
          </div>
        </div>
      )}
    </div>
  );
};

export default MemoryPanel;
