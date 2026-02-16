import React from 'react';
import { Modal, Button, Typography, Tag } from '@douyinfe/semi-ui';
import { IconAlertTriangle } from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

// 工具能力分类映射
const toolCategoryMap = {
  Write: { label: '写入', color: 'orange' },
  Edit: { label: '写入', color: 'orange' },
  Bash: { label: '执行', color: 'red' },
  WebFetch: { label: '网络', color: 'blue' },
  WebSearch: { label: '网络', color: 'blue' },
  Read: { label: '读取', color: 'green' },
  Glob: { label: '读取', color: 'green' },
  Grep: { label: '读取', color: 'green' },
};

const PermissionDialog = ({ request, onResponse }) => {
  if (!request) return null;

  const category = toolCategoryMap[request.tool] || { label: '其他', color: 'grey' };

  // 格式化输入内容显示
  let inputDisplay = request.input;
  if (typeof inputDisplay === 'string') {
    try {
      inputDisplay = JSON.parse(inputDisplay);
    } catch {
      // keep as string
    }
  }

  return (
    <Modal
      visible={!!request}
      onCancel={() => onResponse('deny')}
      footer={null}
      closable={false}
      width={480}
      centered
    >
      <div className="permission-dialog">
        <div className="permission-header">
          <IconAlertTriangle size="large" style={{ color: '#fa8c16' }} />
          <Title heading={5} style={{ marginLeft: 8 }}>权限请求</Title>
        </div>

        <div className="permission-content">
          <div className="permission-item">
            <Text strong>工具：</Text>
            <Text code>{request.tool}</Text>
            <Tag color={category.color} size="small" style={{ marginLeft: 8 }}>{category.label}</Tag>
          </div>

          {request.message && (
            <div className="permission-item">
              <Text strong>说明：</Text>
              <Text>{request.message}</Text>
            </div>
          )}

          {inputDisplay && (
            <div className="permission-item">
              <Text strong>输入：</Text>
              <pre className="permission-input">
                {typeof inputDisplay === 'object' ? JSON.stringify(inputDisplay, null, 2) : String(inputDisplay)}
              </pre>
            </div>
          )}
        </div>

        <div className="permission-actions">
          <Button onClick={() => onResponse('deny')}>拒绝</Button>
          <Button type="secondary" onClick={() => onResponse('allow_always')}>始终允许</Button>
          <Button type="primary" onClick={() => onResponse('allow_once')}>允许本次</Button>
        </div>
      </div>

      <style jsx>{`
        .permission-dialog {
          padding: 8px;
        }
        .permission-header {
          display: flex;
          align-items: center;
          margin-bottom: 16px;
        }
        .permission-content {
          margin-bottom: 20px;
        }
        .permission-item {
          margin-bottom: 12px;
        }
        .permission-input {
          background: var(--semi-color-fill-0, #f5f5f5);
          padding: 8px;
          border-radius: 4px;
          font-size: 12px;
          max-height: 200px;
          overflow: auto;
          margin-top: 4px;
          white-space: pre-wrap;
          word-break: break-all;
        }
        .permission-actions {
          display: flex;
          justify-content: flex-end;
          gap: 8px;
        }
      `}</style>
    </Modal>
  );
};

export default PermissionDialog;
