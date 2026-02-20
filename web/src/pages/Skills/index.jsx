import React, { useState, useEffect, useCallback } from 'react';
import {
  Button, Card, Tag, Modal, Form, Input, Select, Toast, Tabs, TabPane, Popconfirm, Typography
} from '@douyinfe/semi-ui';
import { IconPlus, IconEdit, IconDelete, IconGithubLogo } from '@douyinfe/semi-icons';
import { isAdmin } from '../../helpers/utils';

const { Text, Title } = Typography;

const TYPE_LABELS = { skill: '技能', agent: 'Agent', mcp: 'MCP' };
const TYPE_COLORS = { skill: 'blue', agent: 'purple', mcp: 'green' };

const API_BASE = '/coworker/store';

async function apiFetch(path, options = {}) {
  const user = JSON.parse(localStorage.getItem('user') || '{}');
  const res = await fetch(API_BASE + path, {
    headers: {
      'Content-Type': 'application/json',
      ...(user.token ? { Authorization: 'Bearer ' + user.token } : {}),
    },
    ...options,
  });
  return res.json();
}

function ItemCard({ item, installed, onInstall, onUninstall, onEdit, onDelete, admin }) {
  const isInstalled = installed.some(i => i.item_id === item.id && i.enabled);

  return (
    <Card
      style={{ marginBottom: 12 }}
      bodyStyle={{ padding: '12px 16px' }}
      headerLine={false}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div style={{ flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            {item.icon && <span style={{ fontSize: 18 }}>{item.icon}</span>}
            <Text strong>{item.name}</Text>
            <Tag color={TYPE_COLORS[item.type]} size="small">{TYPE_LABELS[item.type]}</Tag>
            {item.author && <Text type="tertiary" size="small">by {item.author}</Text>}
          </div>
          <Text type="secondary" size="small">{item.description}</Text>
          {item.github_url && (
            <div style={{ marginTop: 4 }}>
              <a href={item.github_url} target="_blank" rel="noreferrer" style={{ fontSize: 12, display: 'flex', alignItems: 'center', gap: 4 }}>
                <IconGithubLogo size="small" /> 查看源码
              </a>
            </div>
          )}
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginLeft: 12 }}>
          {admin && (
            <>
              <Button size="small" icon={<IconEdit />} onClick={() => onEdit(item)} />
              <Popconfirm title="确认删除？" onConfirm={() => onDelete(item.id)}>
                <Button size="small" type="danger" icon={<IconDelete />} />
              </Popconfirm>
            </>
          )}
          {isInstalled
            ? <Button size="small" type="tertiary" onClick={() => onUninstall(item.id)}>已安装</Button>
            : <Button size="small" theme="solid" onClick={() => onInstall(item)}>安装</Button>
          }
        </div>
      </div>
    </Card>
  );
}

function EditModal({ visible, item, onClose, onSave }) {
  const [form, setForm] = useState({});

  useEffect(() => {
    setForm(item ? { ...item } : { type: 'skill' });
  }, [item, visible]);

  const handleSave = async () => {
    if (!form.name || !form.type) {
      Toast.error('名称和类型必填');
      return;
    }
    await onSave(form);
    onClose();
  };

  return (
    <Modal
      title={item ? '编辑条目' : '新增条目'}
      visible={visible}
      onCancel={onClose}
      onOk={handleSave}
      okText="保存"
    >
      <Form labelPosition="left" labelWidth={80}>
        <Form.Input label="名称" value={form.name || ''} onChange={v => setForm(f => ({ ...f, name: v }))} />
        <Form.Select
          label="类型"
          value={form.type}
          onChange={v => setForm(f => ({ ...f, type: v }))}
          optionList={[
            { label: '技能 (Skill)', value: 'skill' },
            { label: 'Agent', value: 'agent' },
            { label: 'MCP', value: 'mcp' },
          ]}
        />
        <Form.TextArea label="描述" value={form.description || ''} onChange={v => setForm(f => ({ ...f, description: v }))} />
        <Form.Input label="图标" placeholder="emoji 或留空" value={form.icon || ''} onChange={v => setForm(f => ({ ...f, icon: v }))} />
        <Form.Input label="作者" value={form.author || ''} onChange={v => setForm(f => ({ ...f, author: v }))} />
        <Form.Input label="GitHub URL" value={form.github_url || ''} onChange={v => setForm(f => ({ ...f, github_url: v }))} />
        {form.type !== 'mcp' && (
          <Form.TextArea
            label="内容"
            placeholder={form.type === 'agent' ? '系统提示词内容' : 'Markdown 技能内容（留空则从 GitHub URL 获取）'}
            value={form.content || ''}
            onChange={v => setForm(f => ({ ...f, content: v }))}
            rows={6}
          />
        )}
        {form.type === 'mcp' && (
          <Form.Input label="服务器 URL" value={form.server_url || ''} onChange={v => setForm(f => ({ ...f, server_url: v }))} />
        )}
      </Form>
    </Modal>
  );
}

function ImportModal({ visible, onClose, onDone }) {
  const [repoURL, setRepoURL] = useState('');
  const [loading, setLoading] = useState(false);

  const handleImport = async () => {
    if (!repoURL.trim()) {
      Toast.error('请输入仓库地址');
      return;
    }
    setLoading(true);
    try {
      const data = await apiFetch('/import', {
        method: 'POST',
        body: JSON.stringify({ repo_url: repoURL.trim() }),
      });
      if (data.success) {
        Toast.success(`已导入 ${data.count} 个条目`);
        setRepoURL('');
        onClose();
        onDone();
      } else {
        Toast.error(data.error || '导入失败');
      }
    } catch {
      Toast.error('网络错误');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title="从 GitHub 安装"
      visible={visible}
      onCancel={onClose}
      onOk={handleImport}
      okText="安装"
      confirmLoading={loading}
    >
      <Form labelPosition="left" labelWidth={100}>
        <Form.Input
          label="GitHub 仓库"
          placeholder="owner/repo 或 https://github.com/owner/repo"
          value={repoURL}
          onChange={setRepoURL}
        />
      </Form>
      <Text type="tertiary" size="small">
        支持格式：含 .claude-plugin/plugin.json、marketplace.json 或根目录 SKILL.md 的仓库
      </Text>
    </Modal>
  );
}

export default function Skills() {
  const [items, setItems] = useState([]);
  const [installed, setInstalled] = useState([]);
  const [editVisible, setEditVisible] = useState(false);
  const [editItem, setEditItem] = useState(null);
  const [importVisible, setImportVisible] = useState(false);
  const [activeTab, setActiveTab] = useState('all');
  const admin = isAdmin();

  const userId = localStorage.getItem('coworker_user_id') || '';

  const loadItems = useCallback(async () => {
    const data = await apiFetch('/items');
    setItems(data.items || []);
  }, []);

  const loadInstalled = useCallback(async () => {
    if (!userId) return;
    const data = await apiFetch(`/user?user_id=${userId}`);
    setInstalled(data.installed || []);
  }, [userId]);

  useEffect(() => {
    loadItems();
    loadInstalled();
  }, [loadItems, loadInstalled]);

  const handleSave = async (form) => {
    if (form.id) {
      await apiFetch(`/items/${form.id}`, { method: 'PUT', body: JSON.stringify(form) });
      Toast.success('已更新');
    } else {
      await apiFetch('/items', { method: 'POST', body: JSON.stringify(form) });
      Toast.success('已创建');
    }
    loadItems();
  };

  const handleDelete = async (id) => {
    await apiFetch(`/items/${id}`, { method: 'DELETE' });
    Toast.success('已删除');
    loadItems();
  };

  const handleInstall = async (item) => {
    const newInstalled = [...installed.filter(i => i.item_id !== item.id), { item_id: item.id, enabled: true }];
    await apiFetch('/user', { method: 'PUT', body: JSON.stringify({ user_id: userId, items: newInstalled }) });
    setInstalled(newInstalled);
    Toast.success(`已安装 ${item.name}`);
  };

  const handleUninstall = async (itemId) => {
    const newInstalled = installed.filter(i => i.item_id !== itemId);
    await apiFetch('/user', { method: 'PUT', body: JSON.stringify({ user_id: userId, items: newInstalled }) });
    setInstalled(newInstalled);
    Toast.success('已卸载');
  };

  const filtered = activeTab === 'all' ? items : items.filter(i => i.type === activeTab);

  return (
    <div className="mt-[60px] px-2" style={{ maxWidth: 900, margin: '60px auto 0' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title heading={4} style={{ margin: 0 }}>技能商店</Title>
        {admin && (
          <div style={{ display: 'flex', gap: 8 }}>
            <Button icon={<IconGithubLogo />} onClick={() => setImportVisible(true)}>
              从 GitHub 安装
            </Button>
            <Button icon={<IconPlus />} theme="solid" onClick={() => { setEditItem(null); setEditVisible(true); }}>
              新增
            </Button>
          </div>
        )}
      </div>

      <Tabs activeKey={activeTab} onChange={setActiveTab} style={{ marginBottom: 16 }}>
        <TabPane tab="全部" itemKey="all" />
        <TabPane tab="技能" itemKey="skill" />
        <TabPane tab="Agent" itemKey="agent" />
        <TabPane tab="MCP" itemKey="mcp" />
      </Tabs>

      {filtered.length === 0 && (
        <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>暂无条目</div>
      )}

      {filtered.map(item => (
        <ItemCard
          key={item.id}
          item={item}
          installed={installed}
          onInstall={handleInstall}
          onUninstall={handleUninstall}
          onEdit={i => { setEditItem(i); setEditVisible(true); }}
          onDelete={handleDelete}
          admin={admin}
        />
      ))}

      <EditModal
        visible={editVisible}
        item={editItem}
        onClose={() => setEditVisible(false)}
        onSave={handleSave}
      />

      <ImportModal
        visible={importVisible}
        onClose={() => setImportVisible(false)}
        onDone={loadItems}
      />
    </div>
  );
}
