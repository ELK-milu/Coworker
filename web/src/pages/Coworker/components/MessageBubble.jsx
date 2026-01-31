import React from 'react';
import { Typography } from '@douyinfe/semi-ui';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const { Text } = Typography;

const MessageBubble = ({ role, content, timestamp }) => {
  const isUser = role === 'user';
  const isError = role === 'error';

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
        </div>
      </div>
    </div>
  );
};

export default MessageBubble;
