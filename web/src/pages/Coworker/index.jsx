/*
Copyright (C) 2025 QuantumNous
*/

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Button, Typography, Spin, TextArea } from '@douyinfe/semi-ui';
import { IconSend } from '@douyinfe/semi-icons';
import MessageBubble from './components/MessageBubble';
import ToolCallCard from './components/ToolCallCard';
import './styles.css';

const { Title, Text } = Typography;

const Coworker = () => {
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [thinking, setThinking] = useState(false);
  const wsRef = useRef(null);
  const messagesEndRef = useRef(null);

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
    }
  };

  // 继续下一部分...

  // 发送消息
  const sendMessage = () => {
    if (!inputValue.trim() || !connected || loading) return;

    const userMsg = { type: 'user', content: inputValue, timestamp: Date.now() };
    setMessages(prev => [...prev, userMsg]);
    setInputValue('');
    setLoading(true);
    setThinking(true);

    wsRef.current.send(JSON.stringify({
      type: 'chat',
      payload: { message: inputValue, user_id: 'user_' + Date.now() }
    }));
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
      />
    );
  };

  // 继续渲染部分...

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
          <div className="input-wrapper">
            <TextArea
              value={inputValue}
              onChange={setInputValue}
              placeholder="输入消息，按 Enter 发送..."
              autosize={{ minRows: 1, maxRows: 5 }}
              onEnterPress={(e) => {
                if (!e.shiftKey) {
                  e.preventDefault();
                  sendMessage();
                }
              }}
              disabled={!connected || loading}
            />
            <Button
              icon={<IconSend />}
              theme="solid"
              onClick={sendMessage}
              disabled={!connected || loading || !inputValue.trim()}
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default Coworker;
