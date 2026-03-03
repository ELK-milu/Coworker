import React, { useEffect, useState } from 'react';
import { Button, Col, Form, Row, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';

const { Text } = Typography;

export default function SettingsBuiltinModel() {
  const [loading, setLoading] = useState(false);
  const [models, setModels] = useState([]);
  const [currentModel, setCurrentModel] = useState('');

  const loadModels = async () => {
    try {
      const res = await API.get('/api/user/models');
      if (res.data?.success) {
        const modelList = res.data.data || [];
        setModels(modelList.map((m) => ({ label: m.id || m, value: m.id || m })));
      }
    } catch {
      // ignore
    }
  };

  const loadCurrentModel = async () => {
    try {
      const res = await API.get('/coworker/builtin-model');
      if (res.data?.success) {
        setCurrentModel(res.data.model || 'gpt-4o-mini');
      }
    } catch {
      // ignore
    }
  };

  const handleSave = async () => {
    if (!currentModel) return;
    setLoading(true);
    try {
      const res = await API.put('/coworker/builtin-model', { model: currentModel });
      if (res.data?.success) {
        showSuccess('内置模型设置已保存');
      } else {
        showError(res.data?.error || '保存失败');
      }
    } catch {
      showError('保存失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadModels();
    loadCurrentModel();
  }, []);

  return (
    <Spin spinning={loading}>
      <Form style={{ marginBottom: 15 }}>
        <Form.Section text='内置模型设置'>
          <Text type='tertiary' style={{ display: 'block', marginBottom: 12 }}>
            设置系统内部 AI 调用使用的模型（如导入时的翻译、分类等）。
            模型列表从当前系统可用模型中获取。
          </Text>
          <Row gutter={16}>
            <Col xs={24} sm={16} md={12} lg={10} xl={8}>
              <Form.Select
                label='内置模型'
                field='BuiltinModel'
                placeholder='选择模型'
                initValue={currentModel}
                value={currentModel}
                optionList={models}
                filter
                style={{ width: '100%' }}
                onChange={(value) => setCurrentModel(value)}
              />
            </Col>
          </Row>
          <Row style={{ marginTop: 8 }}>
            <Button size='default' onClick={handleSave}>
              保存内置模型设置
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
