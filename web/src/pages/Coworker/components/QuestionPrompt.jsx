import React, { useState } from 'react';
import { Button, RadioGroup, Radio, CheckboxGroup, Checkbox, Tag } from '@douyinfe/semi-ui';
import { TextArea } from '@douyinfe/semi-ui';
import { Typography } from '@douyinfe/semi-ui';
import { IconSend, IconClose } from '@douyinfe/semi-icons';
import './QuestionPrompt.css';

const { Text } = Typography;

const QuestionPrompt = ({ questions, onSubmit, onCancel }) => {
  // 每个问题一个回答状态
  const [answers, setAnswers] = useState(() => {
    const init = {};
    questions.forEach((_, i) => {
      init[i] = { selected: null, otherText: '' };
    });
    return init;
  });
  const [activeTab, setActiveTab] = useState(0);

  const updateAnswer = (qIndex, field, value) => {
    setAnswers(prev => ({
      ...prev,
      [qIndex]: { ...prev[qIndex], [field]: value },
    }));
  };

  const handleSubmit = () => {
    const parts = [];
    questions.forEach((q, i) => {
      const ans = answers[i];
      let answerText = '';

      if (q.multiSelect) {
        const selected = ans.selected || [];
        const labels = selected.map(idx => {
          if (idx === '__other__') return null;
          return q.options[idx]?.label;
        }).filter(Boolean);
        if (ans.otherText.trim()) {
          labels.push(ans.otherText.trim());
        }
        answerText = labels.length > 0 ? labels.join(', ') : '(no answer)';
      } else {
        if (ans.selected === '__other__') {
          answerText = ans.otherText.trim() || '(no answer)';
        } else if (ans.selected !== null && ans.selected !== undefined) {
          answerText = q.options[ans.selected]?.label || '(no answer)';
        } else {
          answerText = ans.otherText.trim() || '(no answer)';
        }
      }

      parts.push(`Q: ${q.question}\nA: ${answerText}`);
    });

    onSubmit(parts.join('\n\n'));
  };

  const q = questions[activeTab];
  const ans = answers[activeTab];

  return (
    <div className="question-prompt">
      {/* Tab 切换（多问题时） */}
      {questions.length > 1 && (
        <div className="question-tabs">
          {questions.map((question, i) => (
            <Tag
              key={i}
              className="question-tab"
              color={i === activeTab ? 'blue' : undefined}
              type={i === activeTab ? 'light' : 'ghost'}
              onClick={() => setActiveTab(i)}
            >
              {question.header || `Q${i + 1}`}
            </Tag>
          ))}
        </div>
      )}

      {/* 问题内容 */}
      <div className="question-body">
        {q.header && questions.length <= 1 && (
          <Tag color="blue" type="light" size="small" className="question-header-tag">{q.header}</Tag>
        )}
        <Text className="question-text">{q.question}</Text>

        {/* 选项 */}
        <div className="question-options">
          {q.multiSelect ? (
            <CheckboxGroup
              value={ans.selected || []}
              onChange={(val) => updateAnswer(activeTab, 'selected', val)}
              direction="vertical"
            >
              {q.options.map((opt, j) => (
                <Checkbox key={j} value={j}>
                  <span className="option-label">{opt.label}</span>
                  {opt.description && (
                    <span className="option-desc"> -- {opt.description}</span>
                  )}
                </Checkbox>
              ))}
              <Checkbox value="__other__">
                <span className="option-label">Other</span>
              </Checkbox>
            </CheckboxGroup>
          ) : (
            <RadioGroup
              value={ans.selected}
              onChange={(e) => updateAnswer(activeTab, 'selected', e.target.value)}
              direction="vertical"
            >
              {q.options.map((opt, j) => (
                <Radio key={j} value={j}>
                  <span className="option-label">{opt.label}</span>
                  {opt.description && (
                    <span className="option-desc"> -- {opt.description}</span>
                  )}
                </Radio>
              ))}
              <Radio value="__other__">
                <span className="option-label">Other</span>
              </Radio>
            </RadioGroup>
          )}
        </div>

        {/* "Other" 自由输入 */}
        {(ans.selected === '__other__' ||
          (Array.isArray(ans.selected) && ans.selected.includes('__other__'))) && (
          <TextArea
            className="question-other-input"
            value={ans.otherText}
            onChange={(val) => updateAnswer(activeTab, 'otherText', val)}
            placeholder="Please enter your answer..."
            autosize={{ minRows: 1, maxRows: 3 }}
            autoFocus
          />
        )}
      </div>

      {/* 操作按钮 */}
      <div className="question-actions">
        <Button
          size="small"
          type="tertiary"
          icon={<IconClose />}
          onClick={onCancel}
        >
          Skip
        </Button>
        <Button
          size="small"
          theme="solid"
          icon={<IconSend />}
          onClick={handleSubmit}
        >
          Submit
        </Button>
      </div>
    </div>
  );
};

export default QuestionPrompt;
