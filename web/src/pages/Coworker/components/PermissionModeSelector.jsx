import React from 'react';
import { Select } from '@douyinfe/semi-ui';

const PermissionModeSelector = ({ mode, onChange, disabled }) => {
  const modes = [
    { value: 'default', label: '标准模式' },
    { value: 'acceptEdits', label: '自动编辑' },
    { value: 'plan', label: '规划模式' },
    { value: 'bypassPermissions', label: '绕过权限' },
  ];

  return (
    <Select
      value={mode}
      onChange={onChange}
      disabled={disabled}
      style={{ width: 120 }}
      size="small"
    >
      {modes.map(m => (
        <Select.Option key={m.value} value={m.value}>
          {m.label}
        </Select.Option>
      ))}
    </Select>
  );
};

export default PermissionModeSelector;
