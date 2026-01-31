import React from 'react';
import { Typography, Avatar } from '@douyinfe/semi-ui';
import { IconUser } from '@douyinfe/semi-icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const { Text } = Typography;

const MessageBubble = ({ role, content, timestamp }) => {
  const isUser = role === 'user';
  const isError = role === 'error';

  return (
    <div className={`message-bubble ${isUser ? 'user' : 'assistant'}`}>
      <div className="message-avatar">
        {isUser ? (
          <Avatar size="small" style={{ backgroundColor: 'var(--semi-color-primary)' }}>
            <IconUser />
          </Avatar>
        ) : (
          <Avatar size="small" style={{ backgroundColor: '#6B4EE6' }}>
            AI
          </Avatar>
        )}
      </div>
      <div className={`message-content ${isError ? 'error' : ''}`}>
        <div className="message-header">
          <Text strong size="small">
            {isUser ? '你' : 'Claude'}
          </Text>
          {timestamp && (
            <Text type="tertiary" size="small">
              {new Date(timestamp).toLocaleTimeString()}
            </Text>
          )}
        </div>
        <div className="message-text">
          {isUser ? (
            <Text>{content}</Text>
          ) : (
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {content || ''}
            </ReactMarkdown>
          )}
        </div>
      </div>
    </div>
  );
};

export default MessageBubble;
