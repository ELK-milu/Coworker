/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState, useEffect, useRef } from 'react';
import { Card, Input, Button, Typography, Space, Spin, TextArea } from '@douyinfe/semi-ui';
import { IconSend } from '@douyinfe/semi-icons';

const { Title, Text } = Typography;

const Coworker = () => {
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef(null);
  const messagesEndRef = useRef(null);

  // 滚动到底部
  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  // 连接 WebSocket
  const connectWebSocket = () => {
    console.log('[Coworker] Starting WebSocket connection...');

    // 如果已有连接，先关闭
    if (wsRef.current) {
      console.log('[Coworker] Closing existing connection, readyState:', wsRef.current.readyState);
      wsRef.current.close();
      wsRef.current = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/claudecli/ws`;
    console.log('[Coworker] WebSocket URL:', wsUrl);

    try {
      wsRef.current = new WebSocket(wsUrl);
      console.log('[Coworker] WebSocket object created, readyState:', wsRef.current.readyState);

      wsRef.current.onopen = () => {
        console.log('[Coworker] WebSocket onopen event fired');
        setConnected(true);
      };

      wsRef.current.onmessage = (event) => {
        console.log('[Coworker] WebSocket message received:', event.data);
        try {
          const data = JSON.parse(event.data);
          handleWebSocketMessage(data);
        } catch (error) {
          console.error('[Coworker] Failed to parse message:', error);
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('[Coworker] WebSocket error:', error);
        setConnected(false);
      };

      wsRef.current.onclose = (event) => {
        console.log('[Coworker] WebSocket closed. Code:', event.code, 'Reason:', event.reason);
        setConnected(false);
      };
    } catch (error) {
      console.error('[Coworker] Failed to create WebSocket:', error);
    }
  };

  // 处理 WebSocket 消息
  const handleWebSocketMessage = (data) => {
    if (data.type === 'text') {
      setMessages(prev => {
        const lastMessage = prev[prev.length - 1];
        if (lastMessage && lastMessage.role === 'assistant' && lastMessage.loading) {
          return [
            ...prev.slice(0, -1),
            { ...lastMessage, content: lastMessage.content + data.payload.content, loading: false }
          ];
        }
        return [...prev, { role: 'assistant', content: data.payload.content }];
      });
    } else if (data.type === 'done') {
      setLoading(false);
    } else if (data.type === 'error') {
      setMessages(prev => [...prev, { role: 'error', content: data.payload.error }]);
      setLoading(false);
    }
  };

  // 发送消息
  const sendMessage = () => {
    if (!inputValue.trim() || !connected) return;

    const userMessage = { role: 'user', content: inputValue };
    setMessages(prev => [...prev, userMessage]);
    setInputValue('');
    setLoading(true);

    // 添加加载中的助手消息
    setMessages(prev => [...prev, { role: 'assistant', content: '', loading: true }]);

    // 发送到 WebSocket
    wsRef.current.send(JSON.stringify({
      type: 'chat',
      payload: {
        message: inputValue,
        user_id: 'user_' + Date.now()
      }
    }));
  };

  // 组件挂载时连接 WebSocket
  useEffect(() => {
    connectWebSocket();

    // 清理函数：只在组件真正卸载时关闭连接
    return () => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.close();
      }
    };
  }, []);

  return (
    <div className='mt-[60px] px-2'>
      <Card
        title={
          <Space>
            <Title heading={3} style={{ margin: 0 }}>Coworker - AI 助手</Title>
            <Text type={connected ? 'success' : 'danger'}>
              {connected ? '● 已连接' : '● 未连接'}
            </Text>
          </Space>
        }
        style={{ maxWidth: '1200px', margin: '0 auto' }}
      >
          {/* 消息列表 */}
          <div style={{
            height: '500px',
            overflowY: 'auto',
            marginBottom: '16px',
            padding: '16px',
            background: 'var(--semi-color-bg-1)',
            borderRadius: '8px'
          }}>
            {messages.map((msg, index) => (
              <div
                key={index}
                style={{
                  marginBottom: '12px',
                  textAlign: msg.role === 'user' ? 'right' : 'left'
                }}
              >
                <div
                  style={{
                    display: 'inline-block',
                    maxWidth: '70%',
                    padding: '12px 16px',
                    borderRadius: '8px',
                    background: msg.role === 'user'
                      ? 'var(--semi-color-primary)'
                      : msg.role === 'error'
                      ? 'var(--semi-color-danger)'
                      : 'var(--semi-color-bg-2)',
                    color: msg.role === 'user' || msg.role === 'error'
                      ? 'white'
                      : 'var(--semi-color-text-0)'
                  }}
                >
                  {msg.loading ? <Spin /> : <Text>{msg.content}</Text>}
                </div>
              </div>
            ))}
            <div ref={messagesEndRef} />
          </div>

          {/* 输入框 */}
          <Space style={{ width: '100%' }}>
            <TextArea
              value={inputValue}
              onChange={setInputValue}
              placeholder="输入消息..."
              autosize={{ minRows: 2, maxRows: 4 }}
              onEnterPress={(e) => {
                if (!e.shiftKey) {
                  e.preventDefault();
                  sendMessage();
                }
              }}
              style={{ flex: 1 }}
              disabled={!connected || loading}
            />
            <Button
              icon={<IconSend />}
              theme="solid"
              onClick={sendMessage}
              disabled={!connected || loading || !inputValue.trim()}
              loading={loading}
            >
              发送
            </Button>
          </Space>
        </Card>
      </div>
  );
};

export default Coworker;
