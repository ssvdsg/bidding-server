import React, { useState, useRef, useEffect } from 'react';
import { Modal, Input, Button, Space, Spin, Typography, Select } from 'antd';
import { SendOutlined, RobotOutlined, UserOutlined } from '@ant-design/icons';

const { TextArea } = Input;
const { Text } = Typography;

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  thinking?: string;
}

interface MascotChatProps {
  open: boolean;
  onClose: () => void;
}

function getAccessPassword(): string {
  try {
    return sessionStorage.getItem('access_password') || '';
  } catch { return ''; }
}

export default function MascotChat({ open, onClose }: MascotChatProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([
    { role: 'assistant', content: '你好呀！我是小招AI ✦ 有什么想聊的吗？投标咨询、系统问题，或者随便聊聊都可以哦～' },
  ]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [model, setModel] = useState('TEXT_A14');
  const [enableWebSearch, setEnableWebSearch] = useState(true);
  const [enableThinking, setEnableThinking] = useState(true);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim() || loading) return;

    const userMsg: ChatMessage = { role: 'user', content: input };
    setMessages(prev => [...prev, userMsg]);
    setInput('');

    const assistantMsg: ChatMessage = { role: 'assistant', content: '' };
    setMessages(prev => [...prev, assistantMsg]);
    setLoading(true);

    try {
      const resp = await fetch('/api/chat/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Access-Password': getAccessPassword(),
        },
        body: JSON.stringify({
          prompt: input,
          model,
          web_search: enableWebSearch,
          enable_thinking: enableThinking,
        }),
      });

      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);

      const reader = resp.body?.getReader();
      if (!reader) throw new Error('No reader');

      const decoder = new TextDecoder();
      let buffer = '';
      let thinkingText = '';
      let contentText = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed || !trimmed.startsWith('data:')) continue;
          const payload = trimmed.slice(5).trim();
          if (payload === '[DONE]') continue;
          try {
            const evt = JSON.parse(payload);
            for (const choice of evt.choices || []) {
              const delta = choice.delta || {};
              if (delta.reasoning_content) {
                thinkingText += delta.reasoning_content;
              }
              if (delta.content) {
                contentText += delta.content;
              }
            }
          } catch { /* skip */ }
        }
        setMessages(prev => {
          const updated = [...prev];
          updated[updated.length - 1] = { role: 'assistant', content: contentText, thinking: thinkingText };
          return updated;
        });
      }
    } catch {
      setMessages(prev => {
        const updated = [...prev];
        updated[updated.length - 1] = {
          role: 'assistant',
          content: '抱歉，小招暂时无法连接到 AI 服务。请稍后再试哦～ 😅',
        };
        return updated;
      });
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <Modal
      title={<span style={{ fontFamily: "'ZCOOL KuaiLe', 'PingFang SC', sans-serif", letterSpacing: '1px' }}><span style={{ color: '#fbbf24' }}>✦</span> 小招AI 聊天</span>}
      open={open}
      onCancel={onClose}
      footer={null}
      width={420}
      destroyOnClose
      styles={{ body: { padding: '12px 16px', minHeight: 400, display: 'flex', flexDirection: 'column' } }}
    >
      {/* 工具栏 */}
      <div style={{ marginBottom: 10, display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
        <span
          onClick={() => setEnableWebSearch(!enableWebSearch)}
          style={{
            fontSize: 12,
            cursor: 'pointer',
            color: enableWebSearch ? '#6d28d9' : '#bbb',
            fontWeight: enableWebSearch ? 500 : 400,
            userSelect: 'none',
            transition: 'color 0.2s',
            display: 'flex',
            alignItems: 'center',
            gap: 4,
          }}
        >
          <span style={{ fontSize: 14 }}>{enableWebSearch ? '🌐' : '🔒'}</span>
          {enableWebSearch ? '联网' : '离线'}
        </span>
        <span
          onClick={() => setEnableThinking(!enableThinking)}
          style={{
            fontSize: 12,
            cursor: 'pointer',
            color: enableThinking ? '#6d28d9' : '#bbb',
            fontWeight: enableThinking ? 500 : 400,
            userSelect: 'none',
            transition: 'color 0.2s',
            display: 'flex',
            alignItems: 'center',
            gap: 4,
          }}
        >
          <span style={{ fontSize: 14 }}>{enableThinking ? '💭' : '🧠'}</span>
          {enableThinking ? '思考' : '直出'}
        </span>
        <Select
          size="small"
          value={model}
          onChange={setModel}
          style={{ width: 150 }}
          options={[
            { value: 'TEXT_DEEPSEEK_V4', label: 'DeepSeek V4' },
            { value: 'TEXT_A14', label: 'GLM-5' },
            { value: 'TEXT_A22', label: '千问 3.5 plus' },
          ]}
        />
      </div>

      {/* 消息列表 */}
      <div style={{
        flex: 1,
        overflowY: 'auto',
        display: 'flex',
        flexDirection: 'column',
        gap: 10,
        padding: '4px 0',
        marginBottom: 12,
        maxHeight: 340,
      }}>
        {messages.map((msg, idx) => (
          <div
            key={idx}
            style={{
              display: 'flex',
              gap: 8,
              alignItems: 'flex-start',
              flexDirection: msg.role === 'user' ? 'row-reverse' : 'row',
            }}
          >
            <div style={{
              width: 28,
              height: 28,
              borderRadius: '50%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 14,
              flexShrink: 0,
              background: msg.role === 'user' ? '#f0e8de' : 'linear-gradient(135deg, #fbbf24, #ff6b9d)',
            }}>
              {msg.role === 'user'
                ? <UserOutlined style={{ fontSize: 12 }} />
                : <img src={import.meta.env.BASE_URL + 'assets/xiaozhao.jpg'} alt="小招" style={{ width: 22, height: 22, borderRadius: '50%', objectFit: 'cover' }} />}
            </div>
            <div style={{
              maxWidth: '75%',
              padding: '8px 12px',
              borderRadius: msg.role === 'user' ? '12px 4px 12px 12px' : '4px 12px 12px 12px',
              background: msg.role === 'user' ? '#1a1a2e' : 'rgba(245, 240, 235, 0.8)',
              color: msg.role === 'user' ? '#fff' : '#333',
              fontSize: 13,
              lineHeight: 1.6,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}>
              {msg.thinking && (
                <details open={loading && idx === messages.length - 1 ? true : undefined} style={{ marginBottom: 6, fontSize: 12, opacity: 0.6 }}>
                  <summary style={{ cursor: 'pointer', color: '#6d28d9', userSelect: 'none' }}>💭 思考过程</summary>
                  <div style={{ marginTop: 4, padding: '6px 8px', background: 'rgba(109,40,217,0.04)', borderRadius: 6, color: '#666', lineHeight: 1.5, whiteSpace: 'pre-wrap', fontSize: 11 }}>
                    {msg.thinking}
                  </div>
                </details>
              )}
              {msg.content || (loading && idx === messages.length - 1 ? '...' : '')}
            </div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      {/* 输入区 */}
      <div style={{ display: 'flex', gap: 8 }}>
        <TextArea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="和小招聊点什么吧..."
          autoSize={{ minRows: 1, maxRows: 4 }}
          style={{ borderRadius: 8, fontSize: 13 }}
        />
        <Button
          type="primary"
          icon={<SendOutlined />}
          onClick={handleSend}
          loading={loading}
          style={{
            borderRadius: 8,
            background: 'linear-gradient(135deg, #1a1a2e, #6d28d9)',
            border: 'none',
            height: 'auto',
            minWidth: 44,
          }}
        />
      </div>
    </Modal>
  );
}
