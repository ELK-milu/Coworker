/*
Copyright (C) 2025 QuantumNous
*/

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Button, Typography, Spin, TextArea, Toast } from '@douyinfe/semi-ui';
import { IconSend, IconStop } from '@douyinfe/semi-icons';
import MessageBubble from './components/MessageBubble';
import ToolCallCard from './components/ToolCallCard';
import SessionSidebar from './components/SessionSidebar';
import './styles.css';

const { Title, Text } = Typography;

// 格式化耗时
const formatElapsed = (ms) => {
  if (!ms) return '0s';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
};

// 获取模式标签
const getModeLabel = (mode) => {
  const labels = {
    normal: 'normal',
    acceptEdits: 'accept edits on',
    planMode: 'plan mode on',
    bypassPermissions: 'bypass permissions on',
  };
  return labels[mode] || mode;
};

// 会话存储 key
const SESSION_STORAGE_KEY = 'coworker_session_id';

const Coworker = () => {
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [thinking, setThinking] = useState(false);
  const [status, setStatus] = useState(null);
  const [mode, setMode] = useState('normal');
  const [sessionId, setSessionId] = useState(() => {
    // 从 localStorage 恢复 session_id
    return localStorage.getItem(SESSION_STORAGE_KEY) || '';
  });
  const [sessions, setSessions] = useState([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  // 文件管理相关状态
  const [files, setFiles] = useState([]);
  const [currentPath, setCurrentPath] = useState('');
  const [filesLoading, setFilesLoading] = useState(false);
  // 任务管理相关状态
  const [tasks, setTasks] = useState([]);
  const [tasksLoading, setTasksLoading] = useState(false);
  // 配置相关状态
  const [configContent, setConfigContent] = useState('');
  const [configLoading, setConfigLoading] = useState(false);
  const [userId] = useState(() => {
    // 从 localStorage 获取或生成用户ID
    let uid = localStorage.getItem('coworker_user_id');
    if (!uid) {
      uid = 'user_' + Date.now();
      localStorage.setItem('coworker_user_id', uid);
    }
    return uid;
  });
  const wsRef = useRef(null);
  const messagesEndRef = useRef(null);
  const abortedRef = useRef(false);
  const currentPathRef = useRef(currentPath);  // 用于在闭包中获取最新的 currentPath

  // 滚动到底部
  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  // 同步 currentPath 到 ref，解决闭包问题
  useEffect(() => {
    currentPathRef.current = currentPath;
  }, [currentPath]);

  // 加载历史消息
  const loadHistory = useCallback((ws, sessId) => {
    if (sessId && ws?.readyState === WebSocket.OPEN) {
      console.log('[Coworker] Loading history for session:', sessId);
      ws.send(JSON.stringify({
        type: 'load_history',
        payload: { session_id: sessId }
      }));
    }
  }, []);

  // 加载会话列表
  const loadSessionsList = useCallback((ws) => {
    if (ws?.readyState === WebSocket.OPEN) {
      setSessionsLoading(true);
      ws.send(JSON.stringify({ type: 'list_sessions', payload: { user_id: userId } }));
    }
  }, [userId]);

  // 加载文件列表
  const loadFilesList = useCallback((ws, path = '') => {
    if (ws?.readyState === WebSocket.OPEN) {
      setFilesLoading(true);
      ws.send(JSON.stringify({
        type: 'list_files',
        payload: { user_id: userId, path }
      }));
    }
  }, [userId]);

  // 加载任务列表
  const loadTasksList = useCallback((ws) => {
    if (ws?.readyState === WebSocket.OPEN) {
      setTasksLoading(true);
      ws.send(JSON.stringify({
        type: 'task_list',
        payload: { user_id: userId, list_id: 'default' }
      }));
    }
  }, [userId]);

  // 删除会话
  const deleteSession = useCallback((sessId) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'delete_session',
        payload: { session_id: sessId }
      }));
    }
  }, []);

  // 连接 WebSocket
  const connectWebSocket = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/claudecli/ws`;
    // 获取当前的 sessionId
    const currentSessionId = localStorage.getItem(SESSION_STORAGE_KEY) || '';

    try {
      wsRef.current = new WebSocket(wsUrl);
      wsRef.current.onopen = () => {
        setConnected(true);
        // 连接成功后加载会话列表、文件列表和任务列表
        loadSessionsList(wsRef.current);
        loadFilesList(wsRef.current, '');
        loadTasksList(wsRef.current);
        // 如果有 session_id，加载历史消息
        if (currentSessionId) {
          loadHistory(wsRef.current, currentSessionId);
        }
      };
      wsRef.current.onerror = () => setConnected(false);
      wsRef.current.onclose = () => setConnected(false);
      wsRef.current.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          handleWebSocketMessage(data);
        } catch (error) {
          console.error('[Coworker] Parse error:', error);
        }
      };
    } catch (error) {
      console.error('[Coworker] WebSocket error:', error);
    }
  }, [loadHistory, loadSessionsList, loadFilesList, loadTasksList]);

  useEffect(() => {
    connectWebSocket();
    return () => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.close();
      }
    };
  }, [connectWebSocket]);

  // 处理 WebSocket 消息
  const handleWebSocketMessage = (data) => {
    if (abortedRef.current && data.type !== 'done' && data.type !== 'error') {
      return;
    }

    const { type, payload } = data;

    // 保存 session_id
    if (payload?.session_id && payload.session_id !== sessionId) {
      setSessionId(payload.session_id);
      localStorage.setItem(SESSION_STORAGE_KEY, payload.session_id);
    }

    switch (type) {
      case 'history':
        // 加载历史消息
        if (payload.messages && payload.messages.length > 0) {
          setMessages(payload.messages);
          console.log('[Coworker] Loaded history:', payload.messages.length, 'messages');
        } else if (payload.not_found) {
          // 会话不存在，清除本地存储的 session_id
          setSessionId('');
          localStorage.removeItem(SESSION_STORAGE_KEY);
          console.log('[Coworker] Session not found, cleared session_id');
        }
        break;

      case 'text':
        setThinking(false);
        setMessages(prev => {
          const last = prev[prev.length - 1];
          if (last?.type === 'assistant' && last.streaming) {
            return [...prev.slice(0, -1), { ...last, content: last.content + payload.content }];
          }
          return [...prev, { type: 'assistant', content: payload.content, streaming: true }];
        });
        break;

      case 'thinking':
        // 处理 thinking 消息，用不同样式显示
        setMessages(prev => {
          const last = prev[prev.length - 1];
          if (last?.type === 'thinking' && last.streaming) {
            return [...prev.slice(0, -1), { ...last, content: last.content + payload.content }];
          }
          return [...prev, { type: 'thinking', content: payload.content, streaming: true }];
        });
        break;

      case 'tool_start':
        setMessages(prev => [...prev, {
          type: 'tool',
          toolName: payload.name,
          toolId: payload.tool_id,
          input: payload.input,
          status: 'running',
        }]);
        break;

      case 'tool_end':
        setMessages(prev => prev.map(msg =>
          msg.toolId === payload.tool_id
            ? {
                ...msg,
                status: 'completed',
                result: payload.result,
                isError: payload.is_error,
                elapsedMs: payload.elapsed_ms,
                timeoutMs: payload.timeout_ms,
                timedOut: payload.timed_out,
              }
            : msg
        ));
        // 如果是 Task 相关工具，刷新任务列表
        if (payload.name && payload.name.startsWith('Task')) {
          loadTasksList(wsRef.current);
        }
        break;

      case 'done':
        setLoading(false);
        setThinking(false);
        setMessages(prev => prev.map(msg =>
          msg.streaming ? { ...msg, streaming: false } : msg
        ));
        break;

      case 'error':
        setLoading(false);
        setThinking(false);
        setMessages(prev => [...prev, { type: 'error', content: payload.error }]);
        break;

      case 'status':
        setStatus({
          model: payload.model,
          inputTokens: payload.input_tokens,
          outputTokens: payload.output_tokens,
          totalTokens: payload.total_tokens,
          contextUsed: payload.context_used,
          contextMax: payload.context_max,
          contextPercent: payload.context_percent,
          elapsedMs: payload.elapsed_ms,
          mode: payload.mode,
        });
        break;

      case 'sessions_list':
        setSessionsLoading(false);
        if (payload.sessions) {
          // 按更新时间排序
          const sorted = [...payload.sessions].sort((a, b) => b.updated_at - a.updated_at);
          setSessions(sorted);
          console.log('[Coworker] Loaded sessions list:', sorted.length);
        }
        break;

      case 'session_deleted':
        if (payload.success) {
          setSessions(prev => prev.filter(s => s.id !== payload.session_id));
          // 如果删除的是当前会话，清空
          if (payload.session_id === sessionId) {
            setSessionId('');
            setMessages([]);
            setStatus(null);
            localStorage.removeItem(SESSION_STORAGE_KEY);
          }
          console.log('[Coworker] Session deleted:', payload.session_id);
        }
        break;

      case 'files_list':
        setFilesLoading(false);
        // 即使文件列表为空也要更新状态
        setFiles(payload.files || []);
        setCurrentPath(payload.path || '');
        console.log('[Coworker] Loaded files list:', payload.files?.length || 0, 'path:', payload.path);
        break;

      case 'folder_created':
        if (payload.success) {
          console.log('[Coworker] Folder created:', payload.path);
          // 刷新文件列表，使用 ref 获取最新的 currentPath
          loadFilesList(wsRef.current, currentPathRef.current);
        }
        break;

      case 'file_deleted':
        if (payload.success) {
          console.log('[Coworker] File deleted:', payload.path);
          // 刷新文件列表，使用 ref 获取最新的 currentPath
          loadFilesList(wsRef.current, currentPathRef.current);
        }
        break;

      case 'file_renamed':
        if (payload.success) {
          console.log('[Coworker] File renamed:', payload.old_path, '->', payload.new_name);
          // 刷新文件列表，使用 ref 获取最新的 currentPath
          loadFilesList(wsRef.current, currentPathRef.current);
        }
        break;

      // 任务相关消息
      case 'tasks_list':
        setTasksLoading(false);
        setTasks(payload.tasks || []);
        console.log('[Coworker] Loaded tasks:', payload.tasks?.length || 0);
        break;

      case 'task_created':
        if (payload.success && payload.task) {
          setTasks(prev => [...prev, payload.task]);
          console.log('[Coworker] Task created:', payload.task.id);
        }
        break;

      case 'task_updated':
        if (payload.success && payload.task) {
          if (payload.task.status === 'deleted') {
            setTasks(prev => prev.filter(t => t.id !== payload.task.id));
          } else {
            setTasks(prev => prev.map(t => t.id === payload.task.id ? payload.task : t));
          }
          console.log('[Coworker] Task updated:', payload.task.id);
        }
        break;

      // 配置相关消息
      case 'config_loaded':
        setConfigLoading(false);
        setConfigContent(payload.content || '');
        console.log('[Coworker] Config loaded');
        break;

      case 'config_saved':
        setConfigLoading(false);
        if (payload.success) {
          Toast.success('配置已保存');
          console.log('[Coworker] Config saved');
        } else {
          Toast.error('保存失败: ' + (payload.error || '未知错误'));
        }
        break;
    }
  };

  // 发送消息
  const sendMessage = () => {
    if (!inputValue.trim() || !connected || loading) return;

    abortedRef.current = false;
    const userMsg = { type: 'user', content: inputValue, timestamp: Date.now() };
    setMessages(prev => [...prev, userMsg]);
    setInputValue('');
    setLoading(true);
    setThinking(true);

    wsRef.current.send(JSON.stringify({
      type: 'chat',
      payload: {
        message: inputValue,
        session_id: sessionId,
        user_id: userId,
        mode,
        working_path: currentPath  // 传递当前文件路径
      }
    }));
  };

  // 中断对话
  const abortMessage = () => {
    abortedRef.current = true;
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'abort' }));
    }
    setLoading(false);
    setThinking(false);
    setMessages(prev => {
      const last = prev[prev.length - 1];
      if (last?.streaming) {
        return [...prev.slice(0, -1), { ...last, streaming: false, aborted: true }];
      }
      return prev;
    });
  };

  // 新建对话
  const newChat = () => {
    setSessionId('');
    setMessages([]);
    setStatus(null);
    localStorage.removeItem(SESSION_STORAGE_KEY);
  };

  // 选择会话
  const selectSession = (sessId) => {
    if (sessId === sessionId) return;
    setSessionId(sessId);
    setMessages([]);
    setStatus(null);
    localStorage.setItem(SESSION_STORAGE_KEY, sessId);
    loadHistory(wsRef.current, sessId);
  };

  // 文件导航
  const navigateFile = (path) => {
    setCurrentPath(path);
    loadFilesList(wsRef.current, path);
  };

  // 刷新文件列表
  const refreshFiles = () => {
    loadFilesList(wsRef.current, currentPath);
  };

  // 创建任务
  const createTask = (taskData) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'task_create',
        payload: {
          user_id: userId,
          list_id: 'default',
          ...taskData
        }
      }));
    }
  };

  // 更新任务
  const updateTask = (taskId, updates) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'task_update',
        payload: {
          user_id: userId,
          list_id: 'default',
          task_id: taskId,
          ...updates
        }
      }));
    }
  };

  // 刷新任务列表
  const refreshTasks = () => {
    loadTasksList(wsRef.current);
  };

  // 渲染消息项
  const renderMessage = (msg, index) => {
    if (msg.type === 'tool') {
      return (
        <ToolCallCard
          key={`tool-${msg.toolId}-${index}`}
          toolName={msg.toolName}
          toolId={msg.toolId}
          input={msg.input}
          result={msg.result}
          status={msg.status}
          isError={msg.isError}
          elapsedMs={msg.elapsedMs}
          timeoutMs={msg.timeoutMs}
          timedOut={msg.timedOut}
        />
      );
    }
    // 在最后一条 assistant 消息中显示任务卡片
    const isLastAssistant = msg.type === 'assistant' &&
      index === messages.length - 1 &&
      tasks.length > 0;
    return (
      <MessageBubble
        key={`msg-${index}`}
        role={msg.type}
        content={msg.content}
        timestamp={msg.timestamp}
        aborted={msg.aborted}
        tasks={isLastAssistant ? tasks : null}
      />
    );
  };

  return (
    <div className='mt-[60px] px-2'>
      <div className="coworker-container">
        {/* 会话侧边栏 */}
        <SessionSidebar
          sessions={sessions}
          currentSessionId={sessionId}
          onNewChat={newChat}
          onSelectSession={selectSession}
          onDeleteSession={deleteSession}
          loading={sessionsLoading}
          files={files}
          currentPath={currentPath}
          filesLoading={filesLoading}
          onNavigateFile={navigateFile}
          onRefreshFiles={refreshFiles}
          tasks={tasks}
          tasksLoading={tasksLoading}
          onCreateTask={createTask}
          onUpdateTask={updateTask}
          onRefreshTasks={refreshTasks}
          configContent={configContent}
          configLoading={configLoading}
          onConfigChange={setConfigContent}
          onConfigLoadingChange={setConfigLoading}
          wsRef={wsRef}
          userId={userId}
        />

        {/* 主内容区 */}
        <div className="coworker-main">
          {/* 头部 */}
          <div className="coworker-header">
            <div className="coworker-title">
              <Title heading={4} style={{ margin: 0 }}>Coworker</Title>
              <Text type="tertiary">AI 编程助手</Text>
            </div>
            <div className="connection-status">
              <span className={`status-dot ${connected ? 'connected' : 'disconnected'}`} />
              <Text size="small">{connected ? '已连接' : '未连接'}</Text>
            </div>
          </div>

          {/* 消息列表 */}
          <div className="messages-container">
          {messages.map(renderMessage)}
          {thinking && (
            <div className="thinking-indicator">
              <Spin size="small" />
              <Text type="tertiary">Claude 正在思考...</Text>
            </div>
          )}
          <div ref={messagesEndRef} />
          </div>

          {/* 输入区域 */}
          <div className="input-container">
          {/* 动态状态栏 - 仅在回复时显示 */}
          {loading && status && (
            <div className="status-bar dynamic">
              <span className="status-item">
                <span className="status-label">Model:</span>
                <span className="status-value">{status.model || 'claude-sonnet'}</span>
              </span>
              <span className="status-item">
                <span className="status-label">Tokens:</span>
                <span className="status-value">{status.totalTokens || 0}</span>
              </span>
              <span className="status-item">
                <span className="status-label">Time:</span>
                <span className="status-value">{formatElapsed(status.elapsedMs)}</span>
              </span>
            </div>
          )}

          {/* 常驻状态栏 */}
          <div className="status-bar persistent">
            <div className="mode-buttons">
              {['normal', 'acceptEdits', 'planMode', 'bypassPermissions'].map((m) => (
                <button
                  key={m}
                  className={`mode-btn ${mode === m ? 'active' : ''}`}
                  onClick={() => setMode(m)}
                >
                  {getModeLabel(m)}
                </button>
              ))}
            </div>
            <div className="context-info">
              <span className="context-label">Context left:</span>
              <span className="context-value">
                {status ? `${Math.max(0, 100 - (status.contextPercent || 0)).toFixed(0)}%` : '100%'}
              </span>
            </div>
          </div>

          <div className="input-wrapper">
            <TextArea
              value={inputValue}
              onChange={setInputValue}
              placeholder={loading ? "Claude 正在回复..." : "输入消息，按 Enter 发送..."}
              autosize={{ minRows: 1, maxRows: 5 }}
              onEnterPress={(e) => {
                if (!e.shiftKey && !loading) {
                  e.preventDefault();
                  sendMessage();
                }
              }}
              disabled={!connected}
            />
            {loading ? (
              <Button
                icon={<IconStop />}
                theme="solid"
                type="danger"
                onClick={abortMessage}
              />
            ) : (
              <Button
                icon={<IconSend />}
                theme="solid"
                onClick={sendMessage}
                disabled={!connected || !inputValue.trim()}
              />
            )}
          </div>
        </div>
        </div>
      </div>
    </div>
  );
};

export default Coworker;
