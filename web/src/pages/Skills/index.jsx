import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
  Button, Card, Tag, Modal, Input, Select, Toast, Tabs, TabPane, Popconfirm, Typography, TextArea
} from '@douyinfe/semi-ui';
import { IconPlus, IconEdit, IconDelete, IconGithubLogo } from '@douyinfe/semi-icons';
import { isAdmin, getUserIdFromLocalStorage } from '../../helpers/utils';

const { Text, Title } = Typography;

const TYPE_LABELS = { skill: '技能', agent: 'Agent', mcp: 'MCP', plugin: '插件' };
const TYPE_COLORS = { skill: 'blue', agent: 'purple', mcp: 'green', plugin: 'orange' };
const DEFAULT_ICONS = { skill: '✨', agent: '🤖', mcp: '🔔', plugin: '🔌' };

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

function SkillIcon({ icon, type, size = 28 }) {
  if (icon && icon.startsWith('data:image/')) {
    return <img src={icon} alt="icon" style={{ width: size, height: size, borderRadius: 4, objectFit: 'cover' }} />;
  }
  return <span style={{ fontSize: size * 0.64 }}>{icon || DEFAULT_ICONS[type] || '✨'}</span>;
}

function ItemCard({ item, installed, onInstall, onUninstall, onEdit, onDelete, admin }) {
  const isInstalled = installed.includes(item.id);

  const countByType = (type) => (item.sub_items || []).filter(s => s.type === type).length;

  return (
    <Card
      style={{ marginBottom: 12 }}
      bodyStyle={{ padding: '12px 16px' }}
      headerLine={false}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div style={{ flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            <SkillIcon icon={item.icon} type={item.type} size={28} />
            <Text strong>{item.display_name || item.name}</Text>
            <Tag color={TYPE_COLORS[item.type]} size="small">{TYPE_LABELS[item.type]}</Tag>
            {item.author && <Text type="tertiary" size="small">by {item.author}</Text>}
          </div>
          <Text type="secondary" size="small">{item.display_desc || item.description}</Text>
          {item.type === 'plugin' && item.sub_items && item.sub_items.length > 0 && (
            <div style={{ marginTop: 4, display: 'flex', gap: 8 }}>
              {countByType('agent') > 0 && <Tag size="small" color="purple">{countByType('agent')} Agents</Tag>}
              {countByType('skill') > 0 && <Tag size="small" color="blue">{countByType('skill')} Skills</Tag>}
              {countByType('command') > 0 && <Tag size="small" color="cyan">{countByType('command')} Commands</Tag>}
            </div>
          )}
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
  const iconInputRef = useRef(null);

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

  const handleIconUpload = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      Toast.error('请上传图片文件');
      return;
    }
    const reader = new FileReader();
    reader.onload = (event) => {
      setForm(f => ({ ...f, icon: event.target.result }));
    };
    reader.readAsDataURL(file);
    e.target.value = '';
  };

  return (
    <Modal
      title={item ? '编辑条目' : '新增条目'}
      visible={visible}
      onCancel={onClose}
      onOk={handleSave}
      okText="保存"
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>名称</Text>
          <Input value={form.name || ''} onChange={v => setForm(f => ({ ...f, name: v }))} />
        </div>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>类型</Text>
          <Select
            value={form.type}
            onChange={v => setForm(f => ({ ...f, type: v }))}
            optionList={[
              { label: '技能 (Skill)', value: 'skill' },
              { label: 'Agent', value: 'agent' },
              { label: 'MCP', value: 'mcp' },
              { label: '插件 (Plugin)', value: 'plugin' },
            ]}
            style={{ width: '100%' }}
          />
        </div>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>描述</Text>
          <TextArea value={form.description || ''} onChange={v => setForm(f => ({ ...f, description: v }))} />
        </div>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>显示名称</Text>
          <Input value={form.display_name || ''} onChange={v => setForm(f => ({ ...f, display_name: v }))} placeholder="留空则使用名称字段" />
        </div>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>显示描述</Text>
          <TextArea value={form.display_desc || ''} onChange={v => setForm(f => ({ ...f, display_desc: v }))} placeholder="留空则使用描述字段" />
        </div>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>图标</Text>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <div
              onClick={() => iconInputRef.current?.click()}
              style={{
                width: 48, height: 48, borderRadius: 8, border: '1px dashed var(--semi-color-border)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
                overflow: 'hidden', background: 'var(--semi-color-fill-0)',
              }}
            >
              <SkillIcon icon={form.icon} type={form.type} size={36} />
            </div>
            <Button size="small" onClick={() => iconInputRef.current?.click()}>上传图片</Button>
            {form.icon && (
              <Button size="small" type="danger" onClick={() => setForm(f => ({ ...f, icon: '' }))}>清除</Button>
            )}
            <input ref={iconInputRef} type="file" accept="image/*" style={{ display: 'none' }} onChange={handleIconUpload} />
          </div>
        </div>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>作者</Text>
          <Input value={form.author || ''} onChange={v => setForm(f => ({ ...f, author: v }))} />
        </div>
        <div>
          <Text size="small" style={{ display: 'block', marginBottom: 4 }}>GitHub URL</Text>
          <Input value={form.github_url || ''} onChange={v => setForm(f => ({ ...f, github_url: v }))} />
        </div>
        {form.type !== 'mcp' && (
          <div>
            <Text size="small" style={{ display: 'block', marginBottom: 4 }}>内容</Text>
            <TextArea
              placeholder={form.type === 'agent' ? '系统提示词内容' : 'Markdown 技能内容（留空则从 GitHub URL 获取）'}
              value={form.content || ''}
              onChange={v => setForm(f => ({ ...f, content: v }))}
              rows={6}
            />
          </div>
        )}
        {form.type === 'mcp' && (
          <div>
            <Text size="small" style={{ display: 'block', marginBottom: 4 }}>服务器 URL</Text>
            <Input value={form.server_url || ''} onChange={v => setForm(f => ({ ...f, server_url: v }))} />
          </div>
        )}
      </div>
    </Modal>
  );
}

function ImportModal({ visible, onClose, onDone }) {
  const [repoURL, setRepoURL] = useState('');
  const [importType, setImportType] = useState('skill');
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
        body: JSON.stringify({ repo_url: repoURL.trim(), import_type: importType }),
      });
      if (data.success) {
        Toast.success(`已导入 ${data.count} 个条目`);
        setRepoURL('');
        setImportType('skill');
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
      <div style={{ marginBottom: 12 }}>
        <Text size="small" style={{ display: 'block', marginBottom: 4 }}>导入类型</Text>
        <Select
          value={importType}
          onChange={setImportType}
          optionList={[
            { label: '技能 (Skill)', value: 'skill' },
            { label: 'Agent', value: 'agent' },
            { label: '插件 (Plugin)', value: 'plugin' },
          ]}
          style={{ width: '100%' }}
        />
      </div>
      <div style={{ marginBottom: 12 }}>
        <Text size="small" style={{ display: 'block', marginBottom: 4 }}>GitHub 仓库</Text>
        <Input
          placeholder="owner/repo 或 https://github.com/owner/repo"
          value={repoURL}
          onChange={setRepoURL}
        />
      </div>
      <Text type="tertiary" size="small">
        {importType === 'plugin'
          ? '插件导入：将 agents + skills + commands 作为整体安装，支持 marketplace.json 和 plugin.json'
          : importType === 'agent'
          ? 'Agent 导入：遍历 agents/ 目录中的 .md 文件，根据 frontmatter 创建独立 Agent 条目'
          : '技能导入：导入 skills 目录中的独立技能，支持 SKILL.md 格式'
        }
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

  const rawId = getUserIdFromLocalStorage();
  const userId = rawId && rawId !== -1 ? String(rawId) : '';

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
    const newInstalled = [...new Set([...installed, item.id])];
    const data = await apiFetch('/user', { method: 'PUT', body: JSON.stringify({ user_id: userId, item_ids: newInstalled }) });
    if (data.success) {
      setInstalled(newInstalled);
      Toast.success(`已安装 ${item.display_name || item.name}`);
    } else {
      Toast.error(data.error || '安装失败');
    }
  };

  const handleUninstall = async (itemId) => {
    const newInstalled = installed.filter(id => id !== itemId);
    const data = await apiFetch('/user', { method: 'PUT', body: JSON.stringify({ user_id: userId, item_ids: newInstalled }) });
    if (data.success) {
      setInstalled(newInstalled);
      Toast.success('已卸载');
    } else {
      Toast.error(data.error || '卸载失败');
    }
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
        <TabPane tab="插件" itemKey="plugin" />
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
