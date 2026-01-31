import React, { useState } from 'react';
import { Typography, Toast } from '@douyinfe/semi-ui';
import { IconCopy, IconTick } from '@douyinfe/semi-icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const { Text } = Typography;

const MessageBubble = ({ role, content, timestamp, aborted }) => {
  const [copied, setCopied] = useState(false);
  const isUser = role === 'user';
  const isError = role === 'error';

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content || '');
      setCopied(true);
      Toast.success('已复制到剪贴板');
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      Toast.error('复制失败');
    }
  };

  return (
    <div className={`message-bubble ${isUser ? 'user' : 'assistant'}`}>
      {/* 消息头部：用户名和时间 */}
      <div className="message-header">
        <Text strong size="small" className="message-sender">
          {isUser ? '你' : 'Claude'}
        </Text>
        {timestamp && (
          <Text type="tertiary" size="small">
            {new Date(timestamp).toLocaleTimeString()}
          </Text>
        )}
      </div>
      {/* 消息气泡 */}
      <div className={`message-content ${isUser ? 'user' : 'assistant'} ${isError ? 'error' : ''}`}>
        <div className="message-text">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>
            {content || ''}
          </ReactMarkdown>
          {aborted && <Text type="warning" size="small">（已中断）</Text>}
        </div>
      </div>
      {/* 操作按钮 */}
      <div className="message-actions">
        <button className="action-btn" onClick={handleCopy} title="复制">
          {copied ? <IconTick size="small" /> : <IconCopy size="small" />}
        </button>
      </div>
    </div>
  );
};

export default MessageBubble;
