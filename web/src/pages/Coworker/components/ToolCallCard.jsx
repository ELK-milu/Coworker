import React, { useState } from 'react';
import { Typography, Tag, Spin } from '@douyinfe/semi-ui';
import { IconChevronDown, IconChevronRight, IconTick, IconClose, IconTerminal } from '@douyinfe/semi-icons';

const { Text } = Typography;

// 工具调用卡片组件
const ToolCallCard = ({ toolName, toolId, input, result, status, isError, elapsedMs, timeoutMs, timedOut, execEnv }) => {
  const [expanded, setExpanded] = useState(true); // 默认展开

  // 格式化执行时间
  const formatElapsed = (ms) => {
    if (!ms) return '';
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  // 获取简短的工具调用描述
  const getToolSummary = () => {
    if (!input) return toolName;
    try {
      const inputObj = typeof input === 'string' ? JSON.parse(input) : input;
      // 根据不同工具类型提取关键信息
      if (inputObj.command) {
        // Bash 工具：显示命令的前50个字符
        const cmd = inputObj.command.split('\n')[0].substring(0, 50);
        return `${toolName}(${cmd}${inputObj.command.length > 50 ? '...' : ''})`;
      }
      if (inputObj.file_path) {
        // Read/Write/Edit 工具：显示文件路径
        const fileName = inputObj.file_path.split(/[/\\]/).pop();
        return `${toolName}(${fileName})`;
      }
      if (inputObj.pattern) {
        // Glob/Grep 工具：显示模式
        return `${toolName}(${inputObj.pattern})`;
      }
      return toolName;
    } catch {
      return toolName;
    }
  };

  // 状态标签
  const renderStatus = () => {
    if (status === 'running') {
      return <Spin size="small" />;
    }
    if (status === 'completed') {
      return (
        <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          {elapsedMs > 0 && (
            <Text type="tertiary" size="small">{formatElapsed(elapsedMs)}</Text>
          )}
          {execEnv && (
            <Tag color={execEnv === 'microsandbox' ? 'cyan' : 'grey'} size="small">
              {execEnv === 'microsandbox' ? 'MicroVM' : 'Local'}
            </Tag>
          )}
          {timedOut && (
            <Tag color="orange" size="small">超时</Tag>
          )}
          {isError ? (
            <Tag color="red" size="small"><IconClose size="small" /> 失败</Tag>
          ) : (
            <Tag color="green" size="small"><IconTick size="small" /> 完成</Tag>
          )}
        </span>
      );
    }
    return null;
  };

  // 格式化输出内容
  const formatContent = (content, maxLines = 20) => {
    if (!content) return '';
    const lines = content.split('\n');
    if (lines.length > maxLines) {
      return lines.slice(0, maxLines).join('\n') + `\n... (${lines.length - maxLines} more lines)`;
    }
    return content;
  };

  return (
    <div className="tool-call-card">
      <div
        className="tool-call-header"
        onClick={() => setExpanded(!expanded)}
      >
        <span className="tool-icon"><IconTerminal /></span>
        <Text strong className="tool-name">{getToolSummary()}</Text>
        <span className="tool-status">{renderStatus()}</span>
        <span className="tool-expand">
          {expanded ? <IconChevronDown /> : <IconChevronRight />}
        </span>
      </div>

      {expanded && (
        <div className="tool-call-content">
          {input && (
            <div className="tool-section">
              <Text type="tertiary" size="small">输入参数:</Text>
              <pre className="tool-code">{formatContent(input)}</pre>
            </div>
          )}
          {result && (
            <div className="tool-section">
              <Text type="tertiary" size="small">执行结果:</Text>
              <pre className={`tool-code ${isError ? 'error' : ''}`}>
                {formatContent(result)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ToolCallCard;
