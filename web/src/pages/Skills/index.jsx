import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import {
  Button, Card, Tag, Modal, Input, Select, Toast, Popconfirm, Typography, TextArea
} from '@douyinfe/semi-ui';
import { IconPlus, IconEdit, IconDelete, IconGithubLogo, IconSearch } from '@douyinfe/semi-icons';
import { isAdmin, getUserIdFromLocalStorage } from '../../helpers/utils';

const { Text, Title } = Typography;

const TYPE_LABELS = { skill: '技能', agent: 'Agent', mcp: 'MCP', plugin: '插件' };
const TYPE_COLORS = { skill: 'blue', agent: 'purple', mcp: 'green', plugin: 'orange' };
const DEFAULT_ICONS = { skill: '✨', agent: '🤖', mcp: '🔔', plugin: '🔌' };

const CATEGORIES = [
  { key: 'all', label: '全部', icon: '📋' },
  { key: '_favorites', label: '收藏', icon: '❤️' },
  { key: '_installed', label: '已安装', icon: '✓' },
  { key: '自动化', label: '自动化', icon: '⚙️' },
  { key: '工具', label: '工具', icon: '🔧' },
  { key: '开发', label: '开发', icon: '💻' },
  { key: 'API', label: 'API', icon: '🔌' },
  { key: '文档', label: '文档', icon: '📄' },
  { key: '数据', label: '数据', icon: '📊' },
  { key: '创作', label: '创作', icon: '🎨' },
  { key: '搜索', label: '搜索', icon: '🔍' },
  { key: '其他', label: '其他', icon: '📦' },
];

const TYPE_FILTERS = [
  { key: 'all', label: '全部' },
  { key: 'skill', label: '技能' },
  { key: 'agent', label: 'Agent' },
  { key: 'mcp', label: 'MCP' },
  { key: 'plugin', label: '插件' },
];

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
  if (icon && (icon.startsWith('data:image/') || icon.startsWith('http://') || icon.startsWith('https://'))) {
    return <img src={icon} alt="icon" style={{ width: size, height: size, borderRadius: 4, objectFit: 'cover' }} />;
  }
  return <span style={{ fontSize: size * 0.64 }}>{icon || DEFAULT_ICONS[type] || '✨'}</span>;
}

const cardStyle = {
  borderRadius: 12, cursor: 'pointer', transition: 'transform 0.2s, box-shadow 0.2s',
  border: '1px solid var(--semi-color-border)', overflow: 'hidden',
};
const cardHoverStyle = { transform: 'translateY(-4px)', boxShadow: '0 8px 24px rgba(59,130,246,0.15)' };

function SkillCard({ item, installed, favorites, onInstall, onUninstall, onFavorite, onEdit, onDelete, admin }) {
  const isInstalled = installed.includes(item.id);
  const isFav = favorites.includes(item.id);
  const [hover, setHover] = useState(false);
  const desc = item.display_desc || item.description || '';
  const dateStr = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';

  return (
    <div
      style={{ ...cardStyle, ...(hover ? cardHoverStyle : {}), background: 'var(--semi-color-bg-2)', padding: 16, display: 'flex', flexDirection: 'column', minHeight: 200 }}
      onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}
    >
      {/* Header: icon + name + author */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
        <SkillIcon icon={item.icon} type={item.type} size={48} />
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 600, fontSize: 15, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {item.display_name || item.name}
          </div>
          {item.author && <Text type="tertiary" size="small">by {item.author}</Text>}
        </div>
        {admin && (
          <div style={{ display: 'flex', gap: 4 }} onClick={e => e.stopPropagation()}>
            <Button size="small" icon={<IconEdit />} theme="borderless" onClick={() => onEdit(item)} />
            <Popconfirm title="确认删除？" onConfirm={() => onDelete(item.id)}>
              <Button size="small" type="danger" icon={<IconDelete />} theme="borderless" />
            </Popconfirm>
          </div>
        )}
      </div>

      {/* Description: 3 lines max */}
      <Text type="secondary" size="small" style={{ flex: 1, display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical', overflow: 'hidden', lineHeight: '1.5em', marginBottom: 10 }}>
        {desc}
      </Text>

      {/* Footer */}
      <div style={{ borderTop: '1px solid var(--semi-color-border)', paddingTop: 8, display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, color: 'var(--semi-color-text-2)' }}>
          <span>⭐ {item.install_count || 0}</span>
          {dateStr && <span>📅 {dateStr}</span>}
          <button
            onClick={e => { e.stopPropagation(); onFavorite(item.id); }}
            style={{ width: 28, height: 28, borderRadius: '50%', border: 'none', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', background: isFav ? '#fee2e2' : 'var(--semi-color-fill-0)', fontSize: 14 }}
          >
            {isFav ? '❤️' : '🤍'}
          </button>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          {item.category && <Tag size="small" color="light-blue">{item.category}</Tag>}
          <Tag size="small" color={TYPE_COLORS[item.type]}>{TYPE_LABELS[item.type]}</Tag>
          {isInstalled
            ? <Button size="small" type="tertiary" onClick={e => { e.stopPropagation(); onUninstall(item.id); }}>已安装</Button>
            : <Button size="small" theme="solid" onClick={e => { e.stopPropagation(); onInstall(item); }}>安装</Button>
          }
        </div>
      </div>
    </div>
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
            <Text size="small" style={{ display: 'block', marginBottom: 4 }}>详情页 URL</Text>
            <Input
              value={form.server_url || ''}
              onChange={v => setForm(f => ({ ...f, server_url: v }))}
              placeholder="MCP 服务详情页链接（如魔搭详情页）"
            />
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

function ModelScopeImportModal({ visible, onClose, onDone }) {
  const [msURL, setMsURL] = useState('');
  const [loading, setLoading] = useState(false);

  const handleImport = async () => {
    if (!msURL.trim()) {
      Toast.error('请输入魔搭项目地址');
      return;
    }
    setLoading(true);
    try {
      const data = await apiFetch('/import-modelscope', {
        method: 'POST',
        body: JSON.stringify({ modelscope_url: msURL.trim() }),
      });
      if (data.success) {
        Toast.success(`已导入: ${data.item?.display_name || data.item?.name}`);
        setMsURL('');
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
      title="从魔搭安装 MCP"
      visible={visible}
      onCancel={onClose}
      onOk={handleImport}
      okText="安装"
      confirmLoading={loading}
    >
      <div style={{ marginBottom: 12 }}>
        <Text size="small" style={{ display: 'block', marginBottom: 4 }}>魔搭项目地址</Text>
        <Input
          placeholder="https://www.modelscope.cn/mcp/servers/@org/name"
          value={msURL}
          onChange={setMsURL}
        />
      </div>
      <Text type="tertiary" size="small">
        从魔搭 MCP 广场导入 MCP 服务器，自动获取名称、描述、图标等元数据
      </Text>
    </Modal>
  );
}

export default function Skills() {
  const [items, setItems] = useState([]);
  const [installed, setInstalled] = useState([]);
  const [favorites, setFavorites] = useState([]);
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [activeType, setActiveType] = useState('all');
  const [activeCategory, setActiveCategory] = useState('all');
  const [sortBy, setSortBy] = useState('popular');
  const [editVisible, setEditVisible] = useState(false);
  const [editItem, setEditItem] = useState(null);
  const [importVisible, setImportVisible] = useState(false);
  const [msImportVisible, setMsImportVisible] = useState(false);
  const admin = isAdmin();

  const rawId = getUserIdFromLocalStorage();
  const userId = rawId && rawId !== -1 ? String(rawId) : '';

  // 300ms debounce search
  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 300);
    return () => clearTimeout(t);
  }, [search]);

  const loadItems = useCallback(async () => {
    const data = await apiFetch('/items');
    setItems(data.items || []);
  }, []);

  const loadInstalled = useCallback(async () => {
    if (!userId) return;
    const data = await apiFetch(`/user?user_id=${userId}`);
    setInstalled(data.installed || []);
  }, [userId]);

  const loadFavorites = useCallback(async () => {
    if (!userId) return;
    const data = await apiFetch(`/user/favorites?user_id=${userId}`);
    setFavorites(data.favorites || []);
  }, [userId]);

  useEffect(() => {
    loadItems();
    loadInstalled();
    loadFavorites();
  }, [loadItems, loadInstalled, loadFavorites]);

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
    const data = await apiFetch(`/user/install/${item.id}`, { method: 'POST', body: JSON.stringify({ user_id: userId }) });
    if (data.success) {
      setInstalled(prev => [...new Set([...prev, item.id])]);
      setItems(prev => prev.map(i => i.id === item.id ? { ...i, install_count: (i.install_count || 0) + 1 } : i));
      Toast.success(`已安装 ${item.display_name || item.name}`);
    } else {
      Toast.error(data.error || '安装失败');
    }
  };

  const handleUninstall = async (itemId) => {
    const data = await apiFetch(`/user/uninstall/${itemId}?user_id=${userId}`, { method: 'DELETE' });
    if (data.success) {
      setInstalled(prev => prev.filter(id => id !== itemId));
      setItems(prev => prev.map(i => i.id === itemId ? { ...i, install_count: Math.max(0, (i.install_count || 0) - 1) } : i));
      Toast.success('已卸载');
    } else {
      Toast.error(data.error || '卸载失败');
    }
  };

  const handleFavorite = async (itemId) => {
    const data = await apiFetch(`/user/favorite/${itemId}`, { method: 'POST', body: JSON.stringify({ user_id: userId }) });
    if (data.success) {
      setFavorites(prev => data.favorited ? [...new Set([...prev, itemId])] : prev.filter(id => id !== itemId));
    }
  };

  // Combined filtering: type + category + search + favorites/installed
  const filtered = useMemo(() => {
    let list = items;
    if (activeType !== 'all') list = list.filter(i => i.type === activeType);
    if (activeCategory === '_favorites') list = list.filter(i => favorites.includes(i.id));
    else if (activeCategory === '_installed') list = list.filter(i => installed.includes(i.id));
    else if (activeCategory !== 'all') list = list.filter(i => i.category === activeCategory);
    if (debouncedSearch) {
      const q = debouncedSearch.toLowerCase();
      list = list.filter(i => (i.display_name || i.name || '').toLowerCase().includes(q) || (i.display_desc || i.description || '').toLowerCase().includes(q));
    }
    return sortBy === 'popular'
      ? [...list].sort((a, b) => (b.install_count || 0) - (a.install_count || 0))
      : [...list].sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0));
  }, [items, activeType, activeCategory, debouncedSearch, sortBy, favorites, installed]);

  // Category counts
  const categoryCounts = useMemo(() => {
    const counts = {};
    const typeFiltered = activeType === 'all' ? items : items.filter(i => i.type === activeType);
    counts.all = typeFiltered.length;
    counts._favorites = typeFiltered.filter(i => favorites.includes(i.id)).length;
    counts._installed = typeFiltered.filter(i => installed.includes(i.id)).length;
    CATEGORIES.filter(c => !c.key.startsWith('_') && c.key !== 'all').forEach(c => {
      counts[c.key] = typeFiltered.filter(i => i.category === c.key).length;
    });
    return counts;
  }, [items, activeType, favorites, installed]);

  const pillStyle = (active) => ({
    padding: '4px 14px', borderRadius: 16, border: 'none', cursor: 'pointer', fontSize: 13, fontWeight: active ? 600 : 400,
    background: active ? 'var(--semi-color-primary)' : 'var(--semi-color-fill-0)',
    color: active ? '#fff' : 'var(--semi-color-text-0)',
  });

  return (
    <div className="mt-[60px] px-2" style={{ maxWidth: 1400, margin: '60px auto 0' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title heading={4} style={{ margin: 0 }}>技能商店</Title>
        {admin && (
          <div style={{ display: 'flex', gap: 8 }}>
            <Button onClick={() => setMsImportVisible(true)}>从魔搭安装</Button>
            <Button icon={<IconGithubLogo />} onClick={() => setImportVisible(true)}>从 GitHub 安装</Button>
            <Button icon={<IconPlus />} theme="solid" onClick={() => { setEditItem(null); setEditVisible(true); }}>新增</Button>
          </div>
        )}
      </div>

      {/* Search */}
      <Input prefix={<IconSearch />} placeholder="搜索技能..." value={search} onChange={setSearch} showClear style={{ marginBottom: 12 }} />

      {/* Type pills + Sort pills */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16, flexWrap: 'wrap', gap: 8 }}>
        <div style={{ display: 'flex', gap: 6 }}>
          {TYPE_FILTERS.map(t => (
            <button key={t.key} style={pillStyle(activeType === t.key)} onClick={() => setActiveType(t.key)}>{t.label}</button>
          ))}
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button style={pillStyle(sortBy === 'popular')} onClick={() => setSortBy('popular')}>最热门</button>
          <button style={pillStyle(sortBy === 'newest')} onClick={() => setSortBy('newest')}>最新</button>
        </div>
      </div>

      {/* Sidebar + Grid */}
      <div style={{ display: 'flex', gap: 24 }}>
        {/* Category sidebar */}
        <div style={{ width: 220, flexShrink: 0, position: 'sticky', top: 76, alignSelf: 'flex-start' }}>
          <div style={{ background: 'var(--semi-color-bg-2)', borderRadius: 12, border: '1px solid var(--semi-color-border)', overflow: 'hidden' }}>
            {CATEGORIES.map((c, i) => (
              <div key={c.key}>
                {i === 3 && <div style={{ height: 1, background: 'var(--semi-color-border)' }} />}
                <div
                  onClick={() => setActiveCategory(c.key)}
                  style={{
                    padding: '10px 14px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 8, fontSize: 13,
                    borderLeft: activeCategory === c.key ? '3px solid var(--semi-color-primary)' : '3px solid transparent',
                    background: activeCategory === c.key ? 'var(--semi-color-primary-light-default)' : 'transparent',
                    fontWeight: activeCategory === c.key ? 600 : 400,
                  }}
                >
                  <span>{c.icon}</span>
                  <span style={{ flex: 1 }}>{c.label}</span>
                  <span style={{ color: 'var(--semi-color-text-2)', fontSize: 12 }}>{categoryCounts[c.key] || 0}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Grid */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {filtered.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '60px 0', color: '#999' }}>暂无条目</div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 20 }}>
              {filtered.map(item => (
                <SkillCard
                  key={item.id} item={item} installed={installed} favorites={favorites}
                  onInstall={handleInstall} onUninstall={handleUninstall} onFavorite={handleFavorite}
                  onEdit={i => { setEditItem(i); setEditVisible(true); }} onDelete={handleDelete} admin={admin}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      <EditModal visible={editVisible} item={editItem} onClose={() => setEditVisible(false)} onSave={handleSave} />
      <ImportModal visible={importVisible} onClose={() => setImportVisible(false)} onDone={loadItems} />
      <ModelScopeImportModal visible={msImportVisible} onClose={() => setMsImportVisible(false)} onDone={loadItems} />
    </div>
  );
}
