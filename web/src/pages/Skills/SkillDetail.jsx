import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Breadcrumb, Button, Tag, Spin, Typography, Tabs, TabPane, Toast
} from '@douyinfe/semi-ui';
import { IconArrowLeft, IconDownload, IconFolder, IconFile, IconGithubLogo } from '@douyinfe/semi-icons';
import MarkdownRenderer from '../../components/common/markdown/MarkdownRenderer';
import { getUserIdFromLocalStorage } from '../../helpers/utils';

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
  if (icon && (icon.startsWith('data:image/') || icon.startsWith('http://') || icon.startsWith('https://'))) {
    return <img src={icon} alt="icon" style={{ width: size, height: size, borderRadius: 6, objectFit: 'cover' }} />;
  }
  return <span style={{ fontSize: size * 0.64 }}>{icon || DEFAULT_ICONS[type] || '✨'}</span>;
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

function formatCount(n) {
  if (!n) return '0';
  if (n >= 10000) return (n / 10000).toFixed(1) + '万';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
  return String(n);
}

function FileTreeNode({ node, depth = 0 }) {
  const [expanded, setExpanded] = useState(depth < 2);
  const indent = depth * 20;

  if (node.is_dir) {
    return (
      <>
        <div
          onClick={() => setExpanded(!expanded)}
          style={{
            padding: '6px 8px', paddingLeft: indent + 8, cursor: 'pointer',
            display: 'flex', alignItems: 'center', gap: 6, fontSize: 13,
            borderRadius: 4,
          }}
          onMouseEnter={e => e.currentTarget.style.background = 'var(--semi-color-fill-0)'}
          onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
        >
          <span style={{ fontSize: 10, width: 12, textAlign: 'center' }}>{expanded ? '▼' : '▶'}</span>
          <IconFolder size="small" style={{ color: 'var(--semi-color-warning)' }} />
          <span style={{ fontWeight: 500 }}>{node.name}</span>
        </div>
        {expanded && node.children && node.children.map((child, i) => (
          <FileTreeNode key={child.name + i} node={child} depth={depth + 1} />
        ))}
      </>
    );
  }

  return (
    <div
      style={{
        padding: '5px 8px', paddingLeft: indent + 28, display: 'flex', alignItems: 'center', gap: 6, fontSize: 13,
        borderRadius: 4,
      }}
      onMouseEnter={e => e.currentTarget.style.background = 'var(--semi-color-fill-0)'}
      onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
    >
      <IconFile size="small" style={{ color: 'var(--semi-color-text-2)' }} />
      <span style={{ flex: 1 }}>{node.name}</span>
      {node.size > 0 && <span style={{ color: 'var(--semi-color-text-2)', fontSize: 12 }}>{formatSize(node.size)}</span>}
    </div>
  );
}

function RelatedCard({ item, onClick }) {
  return (
    <div
      onClick={onClick}
      style={{
        padding: '10px 12px', display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer',
        borderRadius: 8, border: '1px solid var(--semi-color-border)', marginBottom: 8,
        transition: 'background 0.15s',
      }}
      onMouseEnter={e => e.currentTarget.style.background = 'var(--semi-color-fill-0)'}
      onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
    >
      <SkillIcon icon={item.icon} type={item.type} size={36} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontWeight: 500, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {item.display_name || item.name}
        </div>
        <Text type="tertiary" size="small">{item.author || ''}</Text>
      </div>
      <span style={{ fontSize: 12, color: 'var(--semi-color-text-2)', whiteSpace: 'nowrap' }}>⭐ {formatCount(item.install_count)}</span>
    </div>
  );
}

export default function SkillDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [item, setItem] = useState(null);
  const [readmeContent, setReadmeContent] = useState('');
  const [fileTree, setFileTree] = useState([]);
  const [allItems, setAllItems] = useState([]);
  const [installed, setInstalled] = useState([]);
  const [favorites, setFavorites] = useState([]);
  const [activeTab, setActiveTab] = useState('readme');

  const rawId = getUserIdFromLocalStorage();
  const userId = rawId && rawId !== -1 ? String(rawId) : '';

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiFetch(`/items/${id}`);
      if (data.error) {
        Toast.error(data.error);
        navigate('/skills');
        return;
      }
      setItem(data.item);
      setReadmeContent(data.readme_content || '');
      setFileTree(data.file_tree || []);
    } catch {
      Toast.error('加载失败');
      navigate('/skills');
    } finally {
      setLoading(false);
    }
  }, [id, navigate]);

  const loadAllItems = useCallback(async () => {
    const data = await apiFetch('/items');
    setAllItems(data.items || []);
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
    loadDetail();
    loadAllItems();
    loadInstalled();
    loadFavorites();
  }, [loadDetail, loadAllItems, loadInstalled, loadFavorites]);

  const isInstalled = item ? installed.includes(item.id) : false;
  const isFav = item ? favorites.includes(item.id) : false;

  const handleInstall = async () => {
    if (!item) return;
    const data = await apiFetch(`/user/install/${item.id}`, { method: 'POST', body: JSON.stringify({ user_id: userId }) });
    if (data.success) {
      setInstalled(prev => [...new Set([...prev, item.id])]);
      setItem(prev => prev ? { ...prev, install_count: (prev.install_count || 0) + 1 } : prev);
      Toast.success('安装成功');
    } else {
      Toast.error(data.error || '安装失败');
    }
  };

  const handleUninstall = async () => {
    if (!item) return;
    const data = await apiFetch(`/user/uninstall/${item.id}?user_id=${userId}`, { method: 'DELETE' });
    if (data.success) {
      setInstalled(prev => prev.filter(i => i !== item.id));
      setItem(prev => prev ? { ...prev, install_count: Math.max(0, (prev.install_count || 0) - 1) } : prev);
      Toast.success('已卸载');
    } else {
      Toast.error(data.error || '卸载失败');
    }
  };

  const handleFavorite = async () => {
    if (!item) return;
    const data = await apiFetch(`/user/favorite/${item.id}`, { method: 'POST', body: JSON.stringify({ user_id: userId }) });
    if (data.success) {
      setFavorites(prev => data.favorited ? [...new Set([...prev, item.id])] : prev.filter(i => i !== item.id));
    }
  };

  const handleDownload = () => {
    if (!item) return;
    const user = JSON.parse(localStorage.getItem('user') || '{}');
    const url = `/coworker/store/items/${item.id}/download`;
    fetch(url, {
      headers: user.token ? { Authorization: 'Bearer ' + user.token } : {},
    })
      .then(res => {
        if (!res.ok) throw new Error('Download failed');
        return res.blob();
      })
      .then(blob => {
        const blobUrl = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = blobUrl;
        a.download = (item.name || 'skill') + '.zip';
        a.click();
        URL.revokeObjectURL(blobUrl);
      })
      .catch(() => Toast.error('下载失败'));
  };

  const relatedItems = useMemo(() => {
    if (!item || allItems.length === 0) return [];
    let related = allItems.filter(i => i.id !== item.id && i.category && i.category === item.category);
    if (related.length < 5) {
      const ids = new Set(related.map(i => i.id));
      ids.add(item.id);
      const extra = allItems
        .filter(i => !ids.has(i.id))
        .sort((a, b) => (b.install_count || 0) - (a.install_count || 0));
      related = related.concat(extra.slice(0, 5 - related.length));
    }
    return related.slice(0, 5);
  }, [item, allItems]);

  if (loading) {
    return (
      <div className="mt-[60px] px-2" style={{ maxWidth: 1200, margin: '60px auto 0', textAlign: 'center', paddingTop: 100 }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!item) return null;

  const createdDate = item.created_at ? new Date(item.created_at).toLocaleDateString() : '';
  const updatedDate = item.updated_at ? new Date(item.updated_at).toLocaleDateString() : '';

  return (
    <div className="mt-[60px] px-2" style={{ maxWidth: 1200, margin: '60px auto 0', paddingBottom: 60 }}>
      {/* Breadcrumb */}
      <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
        <Button
          icon={<IconArrowLeft />}
          theme="borderless"
          size="small"
          onClick={() => navigate('/skills')}
        />
        <Breadcrumb>
          <Breadcrumb.Item onClick={() => navigate('/skills')} style={{ cursor: 'pointer' }}>技能商店</Breadcrumb.Item>
          <Breadcrumb.Item>{item.display_name || item.name}</Breadcrumb.Item>
        </Breadcrumb>
      </div>

      {/* Header Card */}
      <div style={{
        background: 'var(--semi-color-bg-2)', borderRadius: 12, padding: '24px 28px',
        border: '1px solid var(--semi-color-border)', borderLeft: '4px solid var(--semi-color-primary)',
        marginBottom: 24,
      }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
          <SkillIcon icon={item.icon} type={item.type} size={72} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <Title heading={3} style={{ margin: 0, marginBottom: 4 }}>
              {item.display_name || item.name}
            </Title>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
              {item.author && (
                <Text type="tertiary" size="small" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  👤 {item.author}
                </Text>
              )}
              {item.github_url && (
                <a href={item.github_url} target="_blank" rel="noopener noreferrer" onClick={e => e.stopPropagation()}>
                  <IconGithubLogo size="small" style={{ color: 'var(--semi-color-text-2)' }} />
                </a>
              )}
            </div>
            <Text type="secondary" style={{ lineHeight: '1.6em', display: 'block', marginBottom: 12 }}>
              {item.display_desc || item.description || ''}
            </Text>
          </div>
        </div>

        {/* Stats row */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginTop: 12, flexWrap: 'wrap', fontSize: 13, color: 'var(--semi-color-text-2)' }}>
          <span>⭐ {formatCount(item.install_count)}</span>
          {createdDate && <span>📅 上线于 {createdDate}</span>}
          {updatedDate && <span>🕐 更新于 {updatedDate}</span>}
          <button
            onClick={handleFavorite}
            style={{
              border: 'none', background: isFav ? '#fee2e2' : 'var(--semi-color-fill-0)',
              borderRadius: 16, padding: '4px 12px', cursor: 'pointer', fontSize: 13,
              display: 'flex', alignItems: 'center', gap: 4,
            }}
          >
            {isFav ? '❤️ 已收藏' : '🤍 收藏'}
          </button>
        </div>

        {/* Tags row */}
        <div style={{ display: 'flex', gap: 6, marginTop: 12 }}>
          {item.category && <Tag type="ghost" size="large">{item.category}</Tag>}
          <Tag type="ghost" size="large" color={TYPE_COLORS[item.type]}>{TYPE_LABELS[item.type]}</Tag>
        </div>
      </div>

      {/* Content area: left + right */}
      <div style={{ display: 'flex', gap: 24 }}>
        {/* Left: Tabs */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{
            background: 'var(--semi-color-bg-2)', borderRadius: 12, padding: '16px 20px',
            border: '1px solid var(--semi-color-border)', minHeight: 400,
          }}>
            <Tabs activeKey={activeTab} onChange={setActiveTab}>
              <TabPane tab={<span>📄 介绍</span>} itemKey="readme">
                <div style={{ paddingTop: 8 }}>
                  {readmeContent ? (
                    <MarkdownRenderer content={readmeContent} />
                  ) : (
                    <Text type="tertiary" style={{ padding: 40, display: 'block', textAlign: 'center' }}>
                      暂无文档内容
                    </Text>
                  )}
                </div>
              </TabPane>
              <TabPane tab={<span>📁 文件列表</span>} itemKey="files">
                <div style={{ paddingTop: 8 }}>
                  {fileTree && fileTree.length > 0 ? (
                    fileTree.map((node, i) => <FileTreeNode key={node.name + i} node={node} />)
                  ) : (
                    <Text type="tertiary" style={{ padding: 40, display: 'block', textAlign: 'center' }}>
                      暂无本地文件
                    </Text>
                  )}
                </div>
              </TabPane>
            </Tabs>
          </div>
        </div>

        {/* Right sidebar */}
        <div style={{ width: 280, flexShrink: 0 }}>
          {/* Download card */}
          <div style={{
            background: 'var(--semi-color-bg-2)', borderRadius: 12, padding: 20,
            border: '1px solid var(--semi-color-border)', marginBottom: 16,
          }}>
            <Text strong style={{ display: 'block', marginBottom: 12, fontSize: 15 }}>📥 下载技能</Text>
            <Button
              theme="solid"
              icon={<IconDownload />}
              block
              size="large"
              onClick={handleDownload}
              disabled={!fileTree || fileTree.length === 0}
            >
              下载 ZIP
            </Button>
            <Text type="tertiary" size="small" style={{ display: 'block', marginTop: 8 }}>
              包含 SKILL.md 和所有相关文件
            </Text>
            <div style={{ marginTop: 12, borderTop: '1px solid var(--semi-color-border)', paddingTop: 12 }}>
              {isInstalled ? (
                <Button block type="tertiary" onClick={handleUninstall}>卸载</Button>
              ) : (
                <Button block theme="solid" type="primary" onClick={handleInstall}>安装</Button>
              )}
            </div>
          </div>

          {/* Related items */}
          {relatedItems.length > 0 && (
            <div style={{
              background: 'var(--semi-color-bg-2)', borderRadius: 12, padding: 20,
              border: '1px solid var(--semi-color-border)',
            }}>
              <Text strong style={{ display: 'block', marginBottom: 12, fontSize: 15 }}>≡ 相关技能</Text>
              {relatedItems.map(ri => (
                <RelatedCard
                  key={ri.id}
                  item={ri}
                  onClick={() => navigate('/skills/' + ri.id)}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
