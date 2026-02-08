import React, { useState, useEffect } from 'react';
import { Input, Button, Tag, Empty, Modal, Form, TextArea, Slider, Popconfirm, Toast } from '@douyinfe/semi-ui';
import { IconPlus, IconSearch, IconDelete, IconEdit, IconRefresh } from '@douyinfe/semi-icons';
import './MemoryPanel.css';

const MemoryPanel = ({ ws, userId }) => {
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

  // 加载记忆列表
  const loadMemories = () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    setLoading(true);
    ws.send(JSON.stringify({
      type: 'memory_list',
      payload: { user_id: userId }
    }));
  };

  // 搜索记忆
  const searchMemories = () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    if (!searchQuery.trim()) {
      loadMemories();
      return;
    }
    setLoading(true);
    ws.send(JSON.stringify({
      type: 'memory_search',
      payload: { user_id: userId, query: searchQuery }
    }));
  };

  // 创建/更新记忆
  const saveMemory = () => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;

    const tags = formData.tags.split(',').map(t => t.trim()).filter(t => t);
    if (tags.length === 0) {
      Toast.error('请至少添加一个标签');
      return;
    }
    if (!formData.content.trim()) {
      Toast.error('请输入记忆内容');
      return;
    }

    const payload = {
      user_id: userId,
      tags,
      content: formData.content,
      summary: formData.summary || formData.content.substring(0, 50),
      weight: formData.weight
    };

    if (editingMemory) {
      payload.memory_id = editingMemory.id;
      ws.send(JSON.stringify({ type: 'memory_update', payload }));
    } else {
      ws.send(JSON.stringify({ type: 'memory_create', payload }));
    }

    setEditModalVisible(false);
    resetForm();
  };

  // 删除记忆
  const deleteMemory = (memoryId) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({
      type: 'memory_delete',
      payload: { user_id: userId, memory_id: memoryId }
    }));
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

  // 处理 WebSocket 消息
  useEffect(() => {
    if (!ws) return;

    const handleMessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        switch (data.type) {
          case 'memories_list':
            setMemories(data.payload.memories || []);
            setLoading(false);
            break;
          case 'memory_created':
            if (data.payload.success) {
              Toast.success('记忆已创建');
              loadMemories();
            }
            break;
          case 'memory_updated':
            if (data.payload.success) {
              Toast.success('记忆已更新');
              loadMemories();
            }
            break;
          case 'memory_deleted':
            if (data.payload.success) {
              Toast.success('记忆已删除');
              loadMemories();
            }
            break;
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
      loadMemories();
    }
  }, [ws, userId]);

  // 按标签分组
  const groupedMemories = memories.reduce((acc, mem) => {
    const primaryTag = mem.tags[0] || 'other';
    if (!acc[primaryTag]) acc[primaryTag] = [];
    acc[primaryTag].push(mem);
    return acc;
  }, {});

  return (
    <div className="memory-panel">
      <div className="memory-header">
        <div className="memory-search">
          <Input
            prefix={<IconSearch />}
            placeholder="搜索记忆..."
            value={searchQuery}
            onChange={setSearchQuery}
            onEnterPress={searchMemories}
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
          Object.entries(groupedMemories).map(([tag, mems]) => (
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
                  onDelete={() => deleteMemory(mem.id)}
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
