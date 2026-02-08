import React from 'react';
import { Modal, Button, Typography } from '@douyinfe/semi-ui';
import { IconAlertTriangle } from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const PermissionDialog = ({ request, onApprove, onDeny }) => {
  if (!request) return null;

  return (
    <Modal
      visible={!!request}
      onCancel={onDeny}
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
          </div>

          {request.message && (
            <div className="permission-item">
              <Text strong>说明：</Text>
              <Text>{request.message}</Text>
            </div>
          )}

          {request.input && (
            <div className="permission-item">
              <Text strong>输入：</Text>
              <pre className="permission-input">
                {JSON.stringify(request.input, null, 2)}
              </pre>
            </div>
          )}
        </div>

        <div className="permission-actions">
          <Button onClick={onDeny}>拒绝</Button>
          <Button type="primary" onClick={onApprove}>允许</Button>
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
          background: #f5f5f5;
          padding: 8px;
          border-radius: 4px;
          font-size: 12px;
          max-height: 200px;
          overflow: auto;
          margin-top: 4px;
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
