/*
Copyright (C) 2025 QuantumNous
*/

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Button, Typography, Spin, TextArea } from '@douyinfe/semi-ui';
import { IconSend, IconStop } from '@douyinfe/semi-icons';
import MessageBubble from './components/MessageBubble';
import ToolCallCard from './components/ToolCallCard';
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

const Coworker = () => {
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [thinking, setThinking] = useState(false);
  const [status, setStatus] = useState(null);
  const [mode, setMode] = useState('normal');
  const wsRef = useRef(null);
  const messagesEndRef = useRef(null);
  const abortedRef = useRef(false);

  // 滚动到底部
  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  // 连接 WebSocket
  const connectWebSocket = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/claudecli/ws`;

    try {
      wsRef.current = new WebSocket(wsUrl);
      wsRef.current.onopen = () => setConnected(true);
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
  }, []);

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

    switch (type) {
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
            ? { ...msg, status: 'completed', result: payload.result, isError: payload.is_error }
            : msg
        ));
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
      payload: { message: inputValue, user_id: 'user_' + Date.now(), mode }
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
    <div className='mt-[60px] px-2'>
      <div className="coworker-container">
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
  );
};

export default Coworker;
