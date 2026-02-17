/*
Copyright (C) 2025 QuantumNous
*/

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Button, Typography, Spin, TextArea, Toast } from '@douyinfe/semi-ui';
import { IconSend, IconStop, IconInfoCircle, IconClose } from '@douyinfe/semi-icons';
import MessageBubble from './components/MessageBubble';
import ToolCallCard from './components/ToolCallCard';
import InlineTaskCard from './components/InlineTaskCard';
import SessionSidebar from './components/SessionSidebar';
import * as api from './services/api';
import './styles.css';

const { Title, Text } = Typography;

// 格式化耗时
const formatElapsed = (ms) => {
  if (!ms) return '0s';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
};

// 会话统计 localStorage key 前缀
const SESSION_STATS_PREFIX = 'coworker_session_stats_';

// 会话存储 key
const SESSION_STORAGE_KEY = 'coworker_session_id';

const Coworker = () => {
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [thinking, setThinking] = useState(false);
  const [status, setStatus] = useState(null);
  const [mode, setMode] = useState('default');
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
  // 事项相关状态
  const [jobs, setJobs] = useState([]);
  const [jobsLoading, setJobsLoading] = useState(false);
  const [showRightPanel, setShowRightPanel] = useState(false);
  // Token 统计相关状态
  const [turnStats, setTurnStats] = useState(null);  // 本轮统计
  const [sessionStats, setSessionStats] = useState({
    totalInputTokens: 0, totalOutputTokens: 0, totalTokens: 0, totalCost: 0, turnCount: 0,
  });
  const [ratioConfig, setRatioConfig] = useState(null);  // 模型定价配置
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
  const pendingMessageRef = useRef(null);  // 待发送的消息（用于 WS 重连后发送）
  const tasksRef = useRef(tasks);  // 用于在 WebSocket 闭包中获取最新的 tasks
  const turnStatsRef = useRef(null);  // 用于在 done 闭包中获取最新的 turnStats
  const sessionIdRef = useRef(sessionId);  // 用于在闭包中获取最新的 sessionId
  const ratioConfigRef = useRef(null);  // 用于在 calculateCost 闭包中获取最新的 ratioConfig
  const turnCostRef = useRef(0);  // 本轮 cost（status 事件中计算，done 事件中直接读取）
  const turnStartTimeRef = useRef(0);  // 本轮消息发送时间（Unix秒），用于匹配日志

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

  // 同步 tasks 到 ref，解决 WebSocket 闭包问题
  useEffect(() => {
    tasksRef.current = tasks;
  }, [tasks]);

  // 同步 turnStats 到 ref，解决 done 闭包问题
  useEffect(() => {
    turnStatsRef.current = turnStats;
  }, [turnStats]);

  // 同步 sessionId 到 ref
  useEffect(() => {
    sessionIdRef.current = sessionId;
  }, [sessionId]);

  // 同步 ratioConfig 到 ref，解决 calculateCost 闭包问题
  useEffect(() => {
    ratioConfigRef.current = ratioConfig;
  }, [ratioConfig]);

  // 根据 model + input/output tokens 计算美元金额
  const calculateCost = useCallback((model, inputTokens, outputTokens) => {
    const config = ratioConfigRef.current;  // 从 ref 读取最新值
    if (!config || !model) {
      console.warn('[Coworker] calculateCost: ratioConfig or model missing', { ratioConfig: !!config, model });
      return 0;
    }
    const modelRatio = config.model_ratio?.[model] || 1;
    const completionRatio = config.completion_ratio?.[model] || 1;
    // 1 ratio = $0.002 / 1K tokens
    const inputCost = (inputTokens / 1000) * 0.002 * modelRatio;
    const outputCost = (outputTokens / 1000) * 0.002 * modelRatio * completionRatio;
    const totalCost = inputCost + outputCost;
    console.log('[Coworker] calculateCost:', { model, inputTokens, outputTokens, modelRatio, completionRatio, totalCost });
    return totalCost;
  }, []);

  // 从 /api/log/self/ 获取本轮实际计费数据（done 事件后调用）
  const fetchTurnBillingFromLogs = useCallback(async (model, startTime) => {
    try {
      const { API } = await import('../../helpers/api');
      const res = await API.get(`/api/log/self/?start_timestamp=${startTime}&model_name=${encodeURIComponent(model)}&p=1&page_size=50`);
      const items = res.data?.data?.items || res.data?.data || [];
      if (!Array.isArray(items) || items.length === 0) return null;

      let totalPrompt = 0, totalCompletion = 0, totalQuota = 0;
      for (const item of items) {
        totalPrompt += item.prompt_tokens || 0;
        totalCompletion += item.completion_tokens || 0;
        totalQuota += item.quota || 0;
      }

      // quota → USD: quota / quota_per_unit
      const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit')) || 500000;
      const costUSD = totalQuota / quotaPerUnit;

      console.log('[Coworker] fetchTurnBilling: found', items.length, 'log entries', {
        totalPrompt, totalCompletion, totalQuota, costUSD,
      });
      return { promptTokens: totalPrompt, completionTokens: totalCompletion, costUSD };
    } catch (e) {
      console.warn('[Coworker] fetchTurnBilling failed, falling back to local calc:', e);
      return null;
    }
  }, []);

  // 启动时加载模型定价配置
  useEffect(() => {
    (async () => {
      try {
        const { API } = await import('../../helpers/api');
        const res = await API.get('/coworker/ratio_config');
        if (res.data?.data) {
          setRatioConfig(res.data.data);
          console.log('[Coworker] Ratio config loaded:', res.data.data);
        } else {
          console.error('[Coworker] Ratio config response invalid:', res.data);
        }
      } catch (e) {
        console.error('[Coworker] Failed to load ratio config:', e);
      }
    })();
  }, []);

  // 加载会话列表 (REST API)
  const loadSessionsList = useCallback(async () => {
    setSessionsLoading(true);
    try {
      const data = await api.listSessions(userId);
      const sorted = [...(data.sessions || [])].sort((a, b) => b.updated_at - a.updated_at);
      setSessions(sorted);
      console.log('[Coworker] Loaded sessions list:', sorted.length);
    } catch (error) {
      console.error('[Coworker] Failed to load sessions:', error);
    } finally {
      setSessionsLoading(false);
    }
  }, [userId]);

  // 加载文件列表 (REST API)
  const loadFilesList = useCallback(async (path = '') => {
    setFilesLoading(true);
    try {
      const data = await api.listFiles(userId, path);
      setFiles(data.files || []);
      setCurrentPath(data.path || '');
      console.log('[Coworker] Loaded files list:', data.files?.length || 0, 'path:', data.path);
    } catch (error) {
      console.error('[Coworker] Failed to load files:', error);
    } finally {
      setFilesLoading(false);
    }
  }, [userId]);

  // 加载任务列表 (REST API)
  const loadTasksList = useCallback(async () => {
    setTasksLoading(true);
    try {
      const data = await api.listTasks(userId);
      setTasks((data.tasks || []).sort((a, b) => a.order - b.order));
      console.log('[Coworker] Loaded tasks:', data.tasks?.length || 0);
    } catch (error) {
      console.error('[Coworker] Failed to load tasks:', error);
    } finally {
      setTasksLoading(false);
    }
  }, [userId]);

  // 加载事项列表 (REST API)
  const loadJobsList = useCallback(async () => {
    setJobsLoading(true);
    try {
      const data = await api.listJobs(userId);
      setJobs((data.jobs || []).sort((a, b) => a.order - b.order));
      console.log('[Coworker] Loaded jobs:', data.jobs?.length || 0);
    } catch (error) {
      console.error('[Coworker] Failed to load jobs:', error);
    } finally {
      setJobsLoading(false);
    }
  }, [userId]);

  // 删除会话 (REST API)
  const deleteSession = useCallback(async (sessId) => {
    try {
      await api.deleteSession(sessId);
      setSessions(prev => prev.filter(s => s.id !== sessId));
      // 如果删除的是当前会话，清空
      if (sessId === sessionId) {
        setSessionId('');
        setMessages([]);
        setStatus(null);
        setTurnStats(null);
        setSessionStats({ totalInputTokens: 0, totalOutputTokens: 0, totalTokens: 0, totalCost: 0, turnCount: 0 });
        localStorage.removeItem(SESSION_STORAGE_KEY);
      }
      // 清理该会话的统计数据
      localStorage.removeItem(SESSION_STATS_PREFIX + sessId);
      console.log('[Coworker] Session deleted:', sessId);
    } catch (error) {
      console.error('[Coworker] Failed to delete session:', error);
      Toast.error('删除会话失败');
    }
  }, [sessionId]);

  // 连接 WebSocket
  const connectWebSocket = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/coworker/ws`;
    // 获取当前的 sessionId
    const currentSessionId = localStorage.getItem(SESSION_STORAGE_KEY) || '';

    try {
      wsRef.current = new WebSocket(wsUrl);
      wsRef.current.onopen = () => {
        setConnected(true);
        // 连接成功后使用 REST API 加载侧边栏数据
        loadSessionsList();
        loadFilesList('');
        loadTasksList();
        loadJobsList();
        // 如果有 session_id，使用 REST API 加载历史消息
        if (currentSessionId) {
          api.getSessionHistory(currentSessionId).then(data => {
            if (data.messages && data.messages.length > 0) {
              setMessages(data.messages);
              console.log('[Coworker] Loaded history on connect:', data.messages.length, 'messages');
            }
          }).catch(err => console.error('[Coworker] Failed to load history:', err));
          // 恢复会话统计
          const savedStats = localStorage.getItem(SESSION_STATS_PREFIX + currentSessionId);
          if (savedStats) {
            try {
              setSessionStats(JSON.parse(savedStats));
              console.log('[Coworker] Restored session stats on connect');
            } catch (e) {
              console.error('[Coworker] Failed to restore session stats:', e);
            }
          }
          // 恢复 Context left 状态
          const savedContext = localStorage.getItem(`coworker_context_${currentSessionId}`);
          if (savedContext) {
            try {
              const contextData = JSON.parse(savedContext);
              setStatus(contextData);
              console.log('[Coworker] Restored context state on connect:', contextData);
            } catch (e) {
              console.error('[Coworker] Failed to restore context state:', e);
            }
          }
        }
        // 如果有待发送的消息，发送它
        if (pendingMessageRef.current) {
          const pending = pendingMessageRef.current;
          pendingMessageRef.current = null;
          wsRef.current.send(JSON.stringify({
            type: 'chat',
            payload: pending
          }));
          console.log('[Coworker] Sent pending message after reconnect');
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
  }, [loadSessionsList, loadFilesList, loadTasksList, loadJobsList]);

  useEffect(() => {
    connectWebSocket();
    return () => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.close();
      }
    };
  }, [connectWebSocket]);

  // 断开 WebSocket 连接（用于测试 REST API）
  const disconnectWebSocket = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
      setConnected(false);
      Toast.info('WebSocket 已断开，侧边栏功能仍可通过 REST API 使用');
    }
  }, []);

  // 处理 WebSocket 消息
  const handleWebSocketMessage = (data) => {
    if (abortedRef.current && data.type !== 'done' && data.type !== 'error') {
      return;
    }

    const { type, payload } = data;

    // 保存 session_id（同步写 ref，确保 done 闭包能立即读到）
    if (payload?.session_id && payload.session_id !== sessionIdRef.current) {
      setSessionId(payload.session_id);
      sessionIdRef.current = payload.session_id;
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
          // 向后查找最近的 streaming assistant 消息（跳过中间的 task_progress 等）
          let streamIdx = -1;
          for (let i = prev.length - 1; i >= 0; i--) {
            if (prev[i].type === 'assistant' && prev[i].streaming) {
              streamIdx = i;
              break;
            }
            // 只跳过 task_progress，遇到其他类型就停止查找
            if (prev[i].type !== 'task_progress') break;
          }
          if (streamIdx >= 0) {
            const updated = [...prev];
            updated[streamIdx] = { ...updated[streamIdx], content: updated[streamIdx].content + payload.content };
            return updated;
          }
          const newTs = Date.now();
          return [...prev, { type: 'assistant', content: payload.content, streaming: true, timestamp: newTs }];
        });
        break;

      case 'thinking':
        // 处理 thinking 消息，用不同样式显示
        setMessages(prev => {
          let streamIdx = -1;
          for (let i = prev.length - 1; i >= 0; i--) {
            if (prev[i].type === 'thinking' && prev[i].streaming) {
              streamIdx = i;
              break;
            }
            if (prev[i].type !== 'task_progress') break;
          }
          if (streamIdx >= 0) {
            const updated = [...prev];
            updated[streamIdx] = { ...updated[streamIdx], content: updated[streamIdx].content + payload.content };
            return updated;
          }
          return [...prev, { type: 'thinking', content: payload.content, streaming: true, timestamp: Date.now() }];
        });
        break;

      case 'tool_start':
        setMessages(prev => [...prev, {
          type: 'tool',
          toolName: payload.name,
          toolId: payload.tool_id,
          input: payload.input,
          status: 'running',
          timestamp: Date.now(),
        }]);
        break;

      case 'tool_input':
        // 工具输入完成，更新输入参数（在执行前发送）
        console.log('[Coworker] tool_input received:', payload.tool_id, payload.input);
        setMessages(prev => {
          const updated = prev.map(msg =>
            msg.toolId === payload.tool_id
              ? { ...msg, input: payload.input }
              : msg
          );
          console.log('[Coworker] Messages updated for tool_input');
          return updated;
        });
        break;

      case 'tool_end':
        setMessages(prev => prev.map(msg =>
          msg.toolId === payload.tool_id
            ? {
              ...msg,
              status: 'completed',
              input: payload.input || msg.input,
              result: payload.result,
              isError: payload.is_error,
              elapsedMs: payload.elapsed_ms,
              timeoutMs: payload.timeout_ms,
              timedOut: payload.timed_out,
              execEnv: payload.exec_env,
            }
            : msg
        ));
        // 如果是 Task 相关工具，刷新任务列表 (REST API)
        if (payload.name && payload.name.startsWith('Task')) {
          loadTasksList();
        }
        break;

      case 'done':
        setLoading(false);
        setThinking(false);
        setMessages(prev => prev.map(msg =>
          msg.streaming ? { ...msg, streaming: false } : msg
        ));
        // 累加本轮消耗到会话统计
        {
          const turn = turnStatsRef.current;
          const startTime = turnStartTimeRef.current;
          const model = turn?.model || '';

          if (turn && turn.totalTokens > 0) {
            // token 立即累加（API 返回的数据准确）
            setSessionStats(prev => {
              const updated = {
                totalInputTokens: prev.totalInputTokens + turn.inputTokens,
                totalOutputTokens: prev.totalOutputTokens + turn.outputTokens,
                totalTokens: prev.totalTokens + turn.totalTokens,
                totalCost: prev.totalCost,  // cost 等日志数据，先不加
                turnCount: prev.turnCount + 1,
              };
              const sid = sessionIdRef.current;
              if (sid) {
                localStorage.setItem(SESSION_STATS_PREFIX + sid, JSON.stringify(updated));
              }
              return updated;
            });

            // cost 只从日志获取（实际扣费，含缓存/分组倍率）
            if (startTime > 0 && model) {
              fetchTurnBillingFromLogs(model, startTime).then(billing => {
                if (!billing || billing.costUSD <= 0) return;
                console.log('[Coworker] Log billing:', billing.costUSD.toFixed(6));
                setSessionStats(prev => {
                  const updated = {
                    ...prev,
                    totalCost: prev.totalCost + billing.costUSD,
                  };
                  const sid = sessionIdRef.current;
                  if (sid) {
                    localStorage.setItem(SESSION_STATS_PREFIX + sid, JSON.stringify(updated));
                  }
                  return updated;
                });
              });
            }
          }
        }
        break;

      case 'error':
        setLoading(false);
        setThinking(false);
        setMessages(prev => [...prev, { type: 'error', content: payload.error }]);
        break;

      case 'status':
        {
          const statusData = {
            model: payload.model,
            inputTokens: payload.input_tokens,
            outputTokens: payload.output_tokens,
            totalTokens: payload.total_tokens,
            contextUsed: payload.context_used,
            contextMax: payload.context_max,
            contextPercent: payload.context_percent,
            elapsedMs: payload.elapsed_ms,
            mode: payload.mode,
          };
          setStatus(statusData);

          // 持久化 contextPercent（Context left 显示需要）
          const sid = sessionIdRef.current;
          if (sid && statusData.contextPercent !== undefined) {
            localStorage.setItem(`coworker_context_${sid}`, JSON.stringify({
              contextPercent: statusData.contextPercent,
              contextUsed: statusData.contextUsed,
              contextMax: statusData.contextMax,
            }));
          }
        }
        // 记录本轮统计（用于 done 时累加）
        // 必须同步写 ref，因为 done 事件紧随 status 到达，useEffect 来不及同步
        {
          const turnData = {
            model: payload.model,
            inputTokens: payload.input_tokens || 0,
            outputTokens: payload.output_tokens || 0,
            totalTokens: payload.total_tokens || 0,
            elapsedMs: payload.elapsed_ms || 0,
          };
          setTurnStats(turnData);
          turnStatsRef.current = turnData;
          // 同步计算本轮 cost 并存入 ref，done 事件直接读取，不再重新算
          const config = ratioConfigRef.current;
          if (config && payload.model) {
            const modelRatio = config.model_ratio?.[payload.model] || 1;
            const completionRatio = config.completion_ratio?.[payload.model] || 1;
            const inputCost = ((payload.input_tokens || 0) / 1000) * 0.002 * modelRatio;
            const outputCost = ((payload.output_tokens || 0) / 1000) * 0.002 * modelRatio * completionRatio;
            turnCostRef.current = inputCost + outputCost;
          }
        }
        break;

      // AI 工具触发的任务变更事件 — 仅更新侧边栏状态
      case 'task_changed':
        {
          let newTasks;
          const currentTasks = tasksRef.current;
          if (payload.action === 'created' && payload.task) {
            newTasks = [...currentTasks, payload.task].sort((a, b) => a.order - b.order);
          } else if (payload.action === 'updated' && payload.task) {
            newTasks = currentTasks.map(t => t.id === payload.task.id ? payload.task : t);
          } else if (payload.action === 'deleted' && payload.task) {
            newTasks = currentTasks.filter(t => t.id !== payload.task.id);
          }
          if (newTasks) {
            setTasks(newTasks);
            tasksRef.current = newTasks;
          }
        }
        break;

      // 后端发送的任务进度快照 — 嵌入对话流作为历史记录
      case 'task_progress':
        if (payload.tasks && payload.tasks.length > 0) {
          // 同步侧边栏状态
          const sorted = [...payload.tasks].sort((a, b) => a.order - b.order);
          setTasks(sorted);
          tasksRef.current = sorted;
          // 插入对话流
          setMessages(prev => [...prev, {
            type: 'task_progress',
            tasks: sorted.map(t => ({ ...t })),
            timestamp: Date.now(),
          }]);
        }
        break;

      // 新会话创建事件
      case 'session_created':
        if (payload.session_id) {
          // 更新 sessionId（同步写 ref）
          setSessionId(payload.session_id);
          sessionIdRef.current = payload.session_id;
          localStorage.setItem(SESSION_STORAGE_KEY, payload.session_id);
          // 添加新会话到列表顶部
          setSessions(prev => [{
            id: payload.session_id,
            title: payload.title || '新对话',
            created_at: payload.created_at,
            updated_at: payload.updated_at,
            message_count: 0,
          }, ...prev]);
          console.log('[Coworker] Session created:', payload.session_id);
        }
        break;

      // 标题更新事件
      case 'title_updated':
        if (payload.session_id && payload.title) {
          setSessions(prev => prev.map(s =>
            s.id === payload.session_id
              ? { ...s, title: payload.title }
              : s
          ));
          console.log('[Coworker] Title updated:', payload.session_id, payload.title);
        }
        break;

      // Job 执行事件
      case 'job_execution':
        if (payload.job_id) {
          // 更新 job 状态
          setJobs(prev => prev.map(j =>
            j.id === payload.job_id
              ? { ...j, status: payload.status || 'running' }
              : j
          ));
          // 如果有命令，可以显示通知
          if (payload.command) {
            Toast.info(`事项 "${payload.name}" 正在执行...`);
          }
          console.log('[Coworker] Job execution:', payload.job_id, payload.status);
        }
        break;

      // Job 状态更新事件
      case 'job_status':
        if (payload.job_id) {
          setJobs(prev => prev.map(j =>
            j.id === payload.job_id
              ? {
                ...j,
                status: payload.status || j.status,
                last_run: payload.last_run || j.last_run,
                next_run: payload.next_run || j.next_run,
                last_error: payload.last_error || j.last_error,
              }
              : j
          ));
          console.log('[Coworker] Job status updated:', payload.job_id, payload.status);
        }
        break;
    }
  };

  // 发送消息
  const sendMessage = () => {
    if (!inputValue.trim() || loading) return;

    abortedRef.current = false;
    turnCostRef.current = 0;  // 重置本轮 cost
    turnStartTimeRef.current = Math.floor(Date.now() / 1000);  // 记录发送时间（Unix秒）
    const userMsg = { type: 'user', content: inputValue, timestamp: Date.now() };
    setMessages(prev => [...prev, userMsg]);
    const messageToSend = inputValue;
    setInputValue('');
    setLoading(true);
    setThinking(true);

    const payload = {
      message: messageToSend,
      session_id: sessionId,
      user_id: userId,
      mode,
      working_path: currentPath  // 传递当前文件路径
    };

    // 如果 WS 已连接，直接发送
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'chat',
        payload
      }));
    } else {
      // WS 断开，存储待发送消息并重连
      pendingMessageRef.current = payload;
      Toast.info('正在重新连接...');
      connectWebSocket();
    }
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
    setTurnStats(null);
    setSessionStats({ totalInputTokens: 0, totalOutputTokens: 0, totalTokens: 0, totalCost: 0, turnCount: 0 });
    localStorage.removeItem(SESSION_STORAGE_KEY);
  };

  // 选择会话 (REST API)
  const selectSession = async (sessId) => {
    if (sessId === sessionId) return;
    setSessionId(sessId);
    setMessages([]);
    setStatus(null);
    setTurnStats(null);
    localStorage.setItem(SESSION_STORAGE_KEY, sessId);

    // 恢复会话统计
    const savedStats = localStorage.getItem(SESSION_STATS_PREFIX + sessId);
    setSessionStats(savedStats
      ? JSON.parse(savedStats)
      : { totalInputTokens: 0, totalOutputTokens: 0, totalTokens: 0, totalCost: 0, turnCount: 0 }
    );

    // 恢复 Context left 状态
    const savedContext = localStorage.getItem(`coworker_context_${sessId}`);
    if (savedContext) {
      try {
        const contextData = JSON.parse(savedContext);
        setStatus(prev => prev ? { ...prev, ...contextData } : contextData);
        console.log('[Coworker] Restored context state:', contextData);
      } catch (e) {
        console.error('[Coworker] Failed to restore context state:', e);
      }
    }

    // 使用 REST API 加载历史消息
    try {
      const data = await api.getSessionHistory(sessId);
      if (data.messages && data.messages.length > 0) {
        setMessages(data.messages);
        console.log('[Coworker] Loaded history via REST:', data.messages.length, 'messages');
      } else if (data.not_found) {
        setSessionId('');
        localStorage.removeItem(SESSION_STORAGE_KEY);
        console.log('[Coworker] Session not found, cleared session_id');
      }
    } catch (error) {
      console.error('[Coworker] Failed to load history:', error);
    }
  };

  // 文件导航 (REST API)
  const navigateFile = (path) => {
    setCurrentPath(path);
    loadFilesList(path);
  };

  // 刷新文件列表 (REST API)
  const refreshFiles = () => {
    loadFilesList(currentPath);
  };

  // 创建任务 (REST API)
  const createTask = async (taskData) => {
    try {
      const data = await api.createTask(userId, taskData);
      if (data.success && data.task) {
        setTasks(prev => [...prev, data.task].sort((a, b) => a.order - b.order));
        console.log('[Coworker] Task created:', data.task.id);
      }
    } catch (error) {
      console.error('[Coworker] Failed to create task:', error);
      Toast.error('创建任务失败');
    }
  };

  // 更新任务 (REST API)
  const updateTask = async (taskId, updates) => {
    try {
      const data = await api.updateTask(userId, taskId, updates);
      if (data.success && data.task) {
        if (data.task.status === 'deleted') {
          setTasks(prev => prev.filter(t => t.id !== data.task.id));
        } else {
          setTasks(prev => prev.map(t => t.id === data.task.id ? data.task : t));
        }
        console.log('[Coworker] Task updated:', data.task.id);
      }
    } catch (error) {
      console.error('[Coworker] Failed to update task:', error);
      Toast.error('更新任务失败');
    }
  };

  // 刷新任务列表 (REST API)
  const refreshTasks = () => {
    loadTasksList();
  };

  // 任务排序 (REST API)
  const reorderTasks = async (taskIds) => {
    try {
      await api.reorderTasks(userId, taskIds);
      console.log('[Coworker] Tasks reordered');
      loadTasksList();
    } catch (error) {
      console.error('[Coworker] Failed to reorder tasks:', error);
      Toast.error('排序失败');
    }
  };

  // 创建事项 (REST API)
  const createJob = async (jobData) => {
    try {
      const data = await api.createJob(userId, jobData);
      if (data.success && data.job) {
        setJobs(prev => [...prev, data.job].sort((a, b) => a.order - b.order));
        console.log('[Coworker] Job created:', data.job.id);
      }
    } catch (error) {
      console.error('[Coworker] Failed to create job:', error);
      Toast.error('创建事项失败');
    }
  };

  // 更新事项 (REST API)
  const updateJob = async (jobId, updates) => {
    try {
      const data = await api.updateJob(userId, jobId, updates);
      if (data.success && data.job) {
        setJobs(prev => prev.map(j => j.id === data.job.id ? data.job : j));
        console.log('[Coworker] Job updated:', data.job.id);
      }
    } catch (error) {
      console.error('[Coworker] Failed to update job:', error);
      Toast.error('更新事项失败');
    }
  };

  // 删除事项 (REST API)
  const deleteJob = async (jobId) => {
    try {
      await api.deleteJob(userId, jobId);
      setJobs(prev => prev.filter(j => j.id !== jobId));
      console.log('[Coworker] Job deleted:', jobId);
    } catch (error) {
      console.error('[Coworker] Failed to delete job:', error);
      Toast.error('删除事项失败');
    }
  };

  // 运行事项 (REST API)
  const runJob = async (jobId) => {
    try {
      const data = await api.runJob(userId, jobId);
      if (data.success) {
        Toast.success('事项已触发');
        // 更新状态为 running
        setJobs(prev => prev.map(j => j.id === jobId ? { ...j, status: 'running' } : j));
        console.log('[Coworker] Job triggered:', jobId);
      }
    } catch (error) {
      console.error('[Coworker] Failed to run job:', error);
      Toast.error('触发事项失败');
    }
  };

  // 刷新事项列表 (REST API)
  const refreshJobs = () => {
    loadJobsList();
  };

  // 事项排序 (REST API)
  const reorderJobs = async (jobIds) => {
    try {
      await api.reorderJobs(userId, jobIds);
      console.log('[Coworker] Jobs reordered');
      loadJobsList();
    } catch (error) {
      console.error('[Coworker] Failed to reorder jobs:', error);
      Toast.error('排序失败');
    }
  };

  // 渲染消息项
  const renderMessage = (msg, index) => {
    // 调试：打印每条消息的 timestamp
    console.log('[Coworker] renderMessage:', index, msg.type, 'timestamp:', msg.timestamp);

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
          execEnv={msg.execEnv}
        />
      );
    }
    // 任务进度快照 — 嵌入对话流中作为历史记录
    if (msg.type === 'task_progress') {
      return (
        <InlineTaskCard
          key={`task-progress-${index}`}
          tasks={msg.tasks}
          editable={false}
        />
      );
    }
    return (
      <MessageBubble
        key={`msg-${index}`}
        role={msg.type}
        content={msg.content}
        timestamp={msg.timestamp}
        aborted={msg.aborted}
      />
    );
  };

  return (
    <div className='h-screen pt-[64px]'>
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
          onReorderTasks={reorderTasks}
          configContent={configContent}
          configLoading={configLoading}
          onConfigChange={setConfigContent}
          onConfigLoadingChange={setConfigLoading}
          jobs={jobs}
          jobsLoading={jobsLoading}
          onCreateJob={createJob}
          onUpdateJob={updateJob}
          onDeleteJob={deleteJob}
          onRunJob={runJob}
          onRefreshJobs={refreshJobs}
          onReorderJobs={reorderJobs}
          userId={userId}
          ws={wsRef.current}
        />

        {/* 主内容区 */}
        <div className="coworker-main">
          {/* 头部 */}
          <div className="coworker-header">
            <div className="coworker-title">
              <Title heading={4} style={{ margin: 0 }}>Coworker</Title>
              <Text type="tertiary">AI 编程助手</Text>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
              <Button
                icon={<IconInfoCircle />}
                theme={showRightPanel ? 'solid' : 'borderless'}
                size="small"
                onClick={() => setShowRightPanel(!showRightPanel)}
              >
                {showRightPanel ? '隐藏详情' : '显示详情'}
              </Button>
              <div className="connection-status">
                <span className={`status-dot ${connected ? 'connected' : 'disconnected'}`} />
                <Text size="small">{connected ? '已连接' : '未连接'}</Text>
              </div>
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
            {/* 动态状态栏 - 回复时实时更新，结束后保留最后一轮 */}
            {(loading || turnStats) && status && (
              <div className="status-bar dynamic">
                <span className="status-item">
                  <span className="status-label">Model:</span>
                  <span className="status-value">{status.model || 'claude-sonnet'}</span>
                </span>
                <span className="status-item">
                  <span className="status-label">Cost:</span>
                  <span className="status-value">
                    ${calculateCost(status.model, status.inputTokens || 0, status.outputTokens || 0).toFixed(4)}
                  </span>
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
              <div className="context-info">
                <span className="context-label">Context left:</span>
                <span className="context-value">
                  {status ? `${Math.max(0, 100 - (status.contextPercent || 0)).toFixed(0)}%` : '100%'}
                </span>
              </div>
              {sessionStats.turnCount > 0 && (
                <div className="session-stats">
                  <span className="stats-item">
                    <span className="stats-label">Session:</span>
                    <span className="stats-value">{sessionStats.totalTokens.toLocaleString()} tokens</span>
                  </span>
                  <span className="stats-item">
                    <span className="stats-label">Total:</span>
                    <span className="stats-value">${sessionStats.totalCost.toFixed(4)}</span>
                  </span>
                </div>
              )}
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
                  disabled={!inputValue.trim()}
                />
              )}
            </div>
          </div>
        </div>

        {/* 右侧详情面板 */}
        {showRightPanel && (
          <div className="coworker-right-panel">
            <div className="right-panel-header">
              <Title heading={5} style={{ margin: 0 }}>详情</Title>
              <Button
                icon={<IconClose />}
                theme="borderless"
                size="small"
                onClick={() => setShowRightPanel(false)}
              />
            </div>
            <div className="right-panel-content">
              {/* 模型信息 */}
              <div style={{ marginBottom: '16px' }}>
                <Text strong size="small">模型</Text>
                <div style={{ marginTop: '4px', fontFamily: 'Consolas, monospace', fontSize: '13px' }}>
                  {status?.model || '未连接'}
                </div>
              </div>

              {/* Context */}
              <div style={{ marginBottom: '16px' }}>
                <Text strong size="small">Context left</Text>
                <div style={{ marginTop: '4px', fontFamily: 'Consolas, monospace', fontSize: '13px' }}>
                  {status ? `${Math.max(0, 100 - (status.contextPercent || 0)).toFixed(0)}%` : '100%'}
                </div>
              </div>

              {/* 本轮统计 */}
              {turnStats && (
                <div style={{ marginBottom: '16px' }}>
                  <Text strong size="small">本轮统计</Text>
                  <div style={{ marginTop: '4px', fontSize: '12px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <div>Input: {turnStats.inputTokens?.toLocaleString()} tokens</div>
                    <div>Output: {turnStats.outputTokens?.toLocaleString()} tokens</div>
                    <div>Total: {turnStats.totalTokens?.toLocaleString()} tokens</div>
                    <div>Time: {formatElapsed(turnStats.elapsedMs)}</div>
                  </div>
                </div>
              )}

              {/* 会话累计 */}
              {sessionStats.turnCount > 0 && (
                <div style={{ marginBottom: '16px' }}>
                  <Text strong size="small">会话累计</Text>
                  <div style={{ marginTop: '4px', fontSize: '12px', display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <div>Turns: {sessionStats.turnCount}</div>
                    <div>Tokens: {sessionStats.totalTokens?.toLocaleString()}</div>
                    <div>Cost: ${sessionStats.totalCost?.toFixed(4)}</div>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default Coworker;
