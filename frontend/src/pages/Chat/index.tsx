import React, { useState, useRef, useEffect } from 'react';
import {
  Layout, Input, Button, Select, Upload, Typography, Space, Spin,
  Tag, Flex, Card, Alert, Segmented,
} from 'antd';
import {
  SendOutlined, UploadOutlined, RobotOutlined, UserOutlined,
  BulbOutlined, GlobalOutlined, PaperClipOutlined,
} from '@ant-design/icons';
import type { UploadFile } from 'antd';

const { TextArea } = Input;
const { Text, Title } = Typography;
const { Content } = Layout;

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  thinking?: string;
  files?: { file_name: string }[];
}

interface ChatModel {
  id: string;
  name: string;
  desc: string;
}

const API_BASE = '/api';

function getAccessPassword(): string {
  try {
    return sessionStorage.getItem('access_password') || localStorage.getItem('ctyun_access_password') || '';
  } catch { return ''; }
}

// ── ChatInput — 输入状态锁在子组件内，避免高频 keystroke 触发父组件全量 diff ──
interface ChatInputProps {
  loading: boolean;
  selectedModel: string;
  enableWebSearch: boolean;
  enableThinking: boolean;
  onSend: (content: string, files: any[]) => void;
}

const ChatInput: React.FC<ChatInputProps> = React.memo(({
  loading, selectedModel, enableWebSearch, enableThinking, onSend,
}) => {
  const [input, setInput] = useState('');
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [uploading, setUploading] = useState(false);

  const uploadFile = async (file: File): Promise<any> => {
    const formData = new FormData();
    formData.append('file', file);
    const resp = await fetch(`${API_BASE}/chat/upload`, {
      method: 'POST',
      headers: { 'X-Access-Password': getAccessPassword() },
      body: formData,
    });
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({ error: resp.statusText }));
      throw new Error(err.error || '上传失败');
    }
    const data = await resp.json();
    return data?.data;
  };

  const handleSend = async () => {
    if (!input.trim() && fileList.length === 0) return;

    let finalFiles: any[] = [];
    if (fileList.length > 0) {
      setUploading(true);
      try {
        const results = await Promise.all(fileList.map(f => uploadFile(f.originFileObj as File)));
        finalFiles = results.flat();
      } catch (err: any) {
        onSend(`❌ 文件上传失败: ${err.message}`, []);
        setUploading(false);
        setFileList([]);
        return;
      }
      setUploading(false);
    }

    onSend(input, finalFiles);
    setInput('');
    setFileList([]);
  };

  const busy = loading || uploading;

  return (
    <div style={{
      borderTop: '1px solid #f0f0f0', padding: '16px 0',
      background: '#fff',
    }}>
      <Flex vertical gap={8}>
        <Flex align="center" gap={8}>
          <Upload
            multiple
            fileList={fileList}
            onChange={({ fileList: fl }) => setFileList(fl)}
            beforeUpload={() => false}
            showUploadList={{ showPreviewIcon: false }}
          >
            <Button icon={<UploadOutlined />}>文件</Button>
          </Upload>
          <TextArea
            value={input}
            onChange={e => setInput(e.target.value)}
            onPressEnter={e => {
              if (!e.shiftKey) { e.preventDefault(); handleSend(); }
            }}
            placeholder="输入问题（Shift+Enter 换行）"
            rows={2}
            style={{ flex: 1, resize: 'none' }}
          />
          <Button
            type="primary"
            icon={<SendOutlined />}
            onClick={handleSend}
            loading={busy}
            disabled={!input.trim() && fileList.length === 0}
          >
            发送
          </Button>
        </Flex>
        <Text type="secondary" style={{ fontSize: 12, textAlign: 'right' }}>
          模型 {selectedModel} · {enableWebSearch ? '联网搜索' : '不联网'} · {enableThinking ? '深度思考' : '不思考'}
        </Text>
      </Flex>
    </div>
  );
});

const ChatPage: React.FC = () => {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [models, setModels] = useState<ChatModel[]>([]);
  const [selectedModel, setSelectedModel] = useState('TEXT_DEEPSEEK_V4');
  const [enableWebSearch, setEnableWebSearch] = useState(true);
  const [enableThinking, setEnableThinking] = useState(true);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Load models on mount
  useEffect(() => {
    fetch(`${API_BASE}/chat/models`)
      .then(r => r.json())
      .then(d => {
        if (d?.data) setModels(d.data);
      })
      .catch(() => {});
  }, []);

  // ── Auto-scroll with mobile throttling ──
  // Desktop: smooth scrolling; Mobile: instant + rAF-throttled to avoid jank during streaming
  const scrollRafRef = useRef<number | null>(null);
  useEffect(() => {
    const isMobile = window.matchMedia('(max-width: 991px)').matches;
    if (scrollRafRef.current) cancelAnimationFrame(scrollRafRef.current);
    scrollRafRef.current = requestAnimationFrame(() => {
      messagesEndRef.current?.scrollIntoView({ behavior: isMobile ? 'auto' : 'smooth' });
    });
    return () => {
      if (scrollRafRef.current) cancelAnimationFrame(scrollRafRef.current);
    };
  }, [messages]);

  const handleSend = async (content: string, files: any[]) => {
    // content may be an error message already (from upload failure in ChatInput)
    if (content.startsWith('❌')) {
      setMessages(prev => [...prev, { role: 'assistant', content }]);
      return;
    }

    const userMsg: ChatMessage = {
      role: 'user',
      content,
    };
    setMessages(prev => [...prev, userMsg]);

    // Build the assistant message placeholder
    const assistantMsg: ChatMessage = { role: 'assistant', content: '' };
    setMessages(prev => [...prev, assistantMsg]);

    setLoading(true);

    try {
      const resp = await fetch(`${API_BASE}/chat/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Access-Password': getAccessPassword(),
        },
        body: JSON.stringify({
          prompt: content,
          files,
          model: selectedModel,
          web_search: enableWebSearch,
          enable_thinking: enableThinking,
        }),
      });

      if (!resp.ok) {
        const err = await resp.json().catch(() => ({ error: '请求失败' }));
        throw new Error(err.error || `HTTP ${resp.status}`);
      }

      const reader = resp.body?.getReader();
      if (!reader) throw new Error('无法读取响应流');

      const decoder = new TextDecoder();
      let buffer = '';
      let thinkingText = '';
      let contentText = '';
      let inThinking = false;

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
            const choices = evt.choices || [];
            for (const choice of choices) {
              const delta = choice.delta || {};

              if (delta.reasoning_content) {
                inThinking = true;
                thinkingText += delta.reasoning_content;
                setMessages(prev => {
                  const updated = [...prev];
                  const last = updated[updated.length - 1];
                  if (last?.role === 'assistant') {
                    updated[updated.length - 1] = { ...last, thinking: thinkingText, content: contentText };
                  }
                  return updated;
                });
              }

              if (delta.content) {
                if (inThinking) inThinking = false;
                contentText += delta.content;
                setMessages(prev => {
                  const updated = [...prev];
                  const last = updated[updated.length - 1];
                  if (last?.role === 'assistant') {
                    updated[updated.length - 1] = { ...last, content: contentText, thinking: thinkingText };
                  }
                  return updated;
                });
              }
            }
          } catch { /* ignore parse errors */ }
        }
      }
    } catch (err: any) {
      setMessages(prev => {
        const updated = [...prev];
        const last = updated[updated.length - 1];
        if (last?.role === 'assistant') {
          updated[updated.length - 1] = { ...last, content: `❌ ${err.message}` };
        }
        return updated;
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Layout className="page-container" style={{ height: 'calc(100vh - 64px)', background: '#fff', padding: '0 24px' }}>
      {/* Header Controls */}
      <Flex justify="space-between" align="center" style={{ padding: '16px 0', borderBottom: '1px solid #f0f0f0' }}>
        <Title level={4} style={{ margin: 0 }}>AI 聊天</Title>
        <Space>
          <Select
            value={selectedModel}
            onChange={setSelectedModel}
            style={{ width: 220 }}
            options={models.map(m => ({
              value: m.id,
              label: `${m.name} (${m.desc})`,
            }))}
            placeholder="选择模型"
          />
          <Segmented
            value={enableWebSearch ? 'on' : 'off'}
            onChange={v => setEnableWebSearch(v === 'on')}
            options={[
              { value: 'on', label: <><GlobalOutlined /> 联网</> },
              { value: 'off', label: '关闭' },
            ]}
          />
          <Segmented
            value={enableThinking ? 'on' : 'off'}
            onChange={v => setEnableThinking(v === 'on')}
            options={[
              { value: 'on', label: <><BulbOutlined /> 思考</> },
              { value: 'off', label: '关闭' },
            ]}
          />
        </Space>
      </Flex>

      {/* Messages Area */}
      <Content style={{
        flex: 1, overflow: 'auto', padding: '24px 0',
        display: 'flex', flexDirection: 'column', gap: 16,
      }}>
        {messages.length === 0 && (
          <Flex vertical align="center" justify="center" style={{ height: '100%', opacity: 0.5 }}>
            <RobotOutlined style={{ fontSize: 48, marginBottom: 16 }} />
            <Text type="secondary">选择模型，输入问题开始聊天</Text>
          </Flex>
        )}

        {messages.map((msg, i) => (
          <Flex key={i} justify={msg.role === 'user' ? 'flex-end' : 'flex-start'} style={{ maxWidth: '100%' }}>
            <Card
              size="small"
              style={{
                maxWidth: '80%',
                background: msg.role === 'user' ? '#e6f4ff' : '#fafafa',
                borderRadius: 12,
              }}
            >
              {/* File tags */}
              {msg.files && msg.files.length > 0 && (
                <div style={{ marginBottom: 8 }}>
                  {msg.files.map((f, j) => (
                    <Tag key={j} icon={<PaperClipOutlined />} color="blue">{f.file_name}</Tag>
                  ))}
                </div>
              )}

              {/* Thinking content */}
              {msg.thinking && (
                <div style={{
                  background: '#fffbe6', padding: '8px 12px', borderRadius: 8,
                  marginBottom: 8, fontSize: 13, color: '#ad8b00',
                  border: '1px solid #ffe58f',
                }}>
                  <div style={{ fontWeight: 600, marginBottom: 4 }}>
                    <BulbOutlined /> 思考过程
                  </div>
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontFamily: 'inherit' }}>
                    {msg.thinking}
                  </pre>
                </div>
              )}

              {/* Main content */}
              <pre style={{
                margin: 0, whiteSpace: 'pre-wrap', fontFamily: 'inherit',
                fontSize: 14, lineHeight: 1.6,
              }}>
                {msg.content || (loading && i === messages.length - 1 ? '' : '')}
              </pre>
            </Card>
          </Flex>
        ))}

        {loading && (
          <Flex justify="center" style={{ padding: 8 }}>
            <Spin size="small" />
          </Flex>
        )}

        <div ref={messagesEndRef} />
      </Content>

      {/* Input Area — isolated component prevents keystroke re-renders on ChatPage */}
      <ChatInput
        loading={loading}
        selectedModel={selectedModel}
        enableWebSearch={enableWebSearch}
        enableThinking={enableThinking}
        onSend={handleSend}
      />
    </Layout>
  );
};

export default ChatPage;
