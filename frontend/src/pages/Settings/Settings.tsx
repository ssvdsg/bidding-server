import React, { useEffect, useState } from 'react';
import { Card, Form, Input, Button, Switch, message, Space, Typography, Tabs, Select, Modal } from 'antd';
import { useRequest } from 'ahooks';
import { accessPasswordStore, client, unwrapData } from '@/api/client';
import { AIModel, SystemSettings } from '@/types/api';

const AI_KEYS: Array<keyof SystemSettings> = ['RELAY_AI_BASE_URL', 'RELAY_AI_API_KEY', 'RELAY_AI_MODEL', 'RELAY_AI_FILE_MODEL', 'AI_PROVIDER', 'CTYUN_MODEL', 'ai_prompt'];
const WECHAT_KEYS: Array<keyof SystemSettings> = ['WECHAT_HOOK_URL', 'WECHAT_NOTICE_BASE_URL', 'WECHAT_HIGH_SCORE_THRESHOLD', 'WECHAT_HIGH_SCORE_ROOM', 'WECHAT_DEFAULT_ROOM'];
const SITE_KEYS: Array<keyof SystemSettings> = ['access_password', 'auto_exclude_days', 'auto_delete_days', 'AUTO_AI_ANALYSIS_ENABLED', 'LISTEN_ADDR', 'PORT'];
const DB_KEYS: Array<keyof SystemSettings> = ['DB_HOST', 'DB_PORT', 'DB_NAME', 'DB_USER', 'DB_PASSWORD'];

function buildFileModelOptions(models: AIModel[]) {
  const options = models.map((model) => ({
    value: model.model_key,
    label: `${model.model_name} (${model.model_key})`,
  }));

  if (!models.some((model) => model.model_key === 'lingxi')) {
    options.push({ value: 'lingxi', label: 'lingxi（共享上传/普通问答）' });
  }

  return options;
}

function toBooleanString(value: unknown, fallback = false): boolean {
  if (typeof value === 'boolean') return value;
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'true') return true;
    if (normalized === 'false') return false;
  }
  return fallback;
}

export default function Settings() {
  const [form] = Form.useForm<SystemSettings>();
  const [passwordForm] = Form.useForm<{ password: string }>();
  const [saving, setSaving] = useState(false);
  const [authStatus, setAuthStatus] = useState<'checking' | 'required' | 'verified'>('checking');
  const [verifying, setVerifying] = useState(false);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const isVerified = authStatus === 'verified';

  const { data, loading, refresh } = useRequest(async () => {
    const res = await client.get<{ data: SystemSettings }>('/settings/system');
    return unwrapData(res);
  }, {
    ready: isVerified,
  });
  const { data: models } = useRequest(async () => unwrapData(
    await client.get<{ data: AIModel[] }>('/ai/models'),
  ), {
    ready: isVerified,
  });

  useEffect(() => {
    if (data) {
      form.setFieldsValue({
        ...data,
        AUTO_AI_ANALYSIS_ENABLED: toBooleanString(data.AUTO_AI_ANALYSIS_ENABLED, true),
      });
      // 注意：initialValues 必须保留服务端原始值（可能是空字符串），
      // 否则 saveChangedKeys 比较时会把"空字符串 → true"误判成"无变化"，导致开关无法保存。
      setInitialValues({
        ...data,
      });
    }
  }, [data, form]);

  useEffect(() => {
    passwordForm.setFieldsValue({ password: accessPasswordStore.get() });
  }, [passwordForm]);

  useEffect(() => {
    void verifyPassword(accessPasswordStore.get(), true);
  }, []);

  const verifyPassword = async (password: string, silent = false) => {
    setVerifying(true);
    try {
      await client.post('/settings/auth/verify', { password });
      if (password.trim()) {
        accessPasswordStore.set(password.trim());
      } else {
        accessPasswordStore.clear();
      }
      setAuthStatus('verified');
      return true;
    } catch (error) {
      accessPasswordStore.clear();
      setAuthStatus('required');
      if (!silent) {
        message.error(error instanceof Error ? error.message : '密码验证失败');
      }
      return false;
    } finally {
      setVerifying(false);
    }
  };

  const normalizeFormValue = (value: unknown) => {
    if (typeof value === 'boolean') return String(value);
    if (value == null) return '';
    return String(value);
  };

  // 把开关字段强制归一化为 "true"/"false"，永远不会写入空字符串
  const normalizeBooleanSetting = (value: unknown, fallback = true): 'true' | 'false' => {
    if (typeof value === 'boolean') return value ? 'true' : 'false';
    if (typeof value === 'string') {
      const normalized = value.trim().toLowerCase();
      if (normalized === 'true') return 'true';
      if (normalized === 'false') return 'false';
    }
    return fallback ? 'true' : 'false';
  };

  const saveKeys = async (values: SystemSettings, keys: Array<keyof SystemSettings>) => {
    for (const key of keys) {
      const rawValue = values[key];
      const value = normalizeFormValue(rawValue);
      await client.post('/config', { key, value });
    }
  };

  const saveChangedKeys = async (values: SystemSettings, keys: Array<keyof SystemSettings>) => {
    const changedKeys = keys.filter((key) => {
      // 字段还未在 form store 中赋值（用户没切到对应 tab 且 setFieldsValue 也没写过），跳过，避免误覆盖。
      if (values[key] === undefined) return false;
      return normalizeFormValue(values[key]) !== normalizeFormValue(initialValues[key]);
    });
    if (!changedKeys.length) return changedKeys;
    await saveKeys(values, changedKeys);
    return changedKeys;
  };

  const handleSave = async () => {
    // 注意：必须用 getFieldsValue(true) 拿"全部 store 值"，否则 antd 默认只返回当前已挂载 tab 的字段，
    // 未切换过的 tab（如微信、数据库）字段会是 undefined，归一化成空字符串后会把数据库里的有效值刷成空。
    const values = form.getFieldsValue(true);
    const currentPassword = accessPasswordStore.get();
    const nextPassword = values.access_password || '';
    setSaving(true);
    try {
      await saveChangedKeys(values, AI_KEYS);
      await saveChangedKeys(values, WECHAT_KEYS);
      // 注意：把开关字段从批量保存里剔除，下面用 normalizeBooleanSetting 单独提交，
      // 防止 form 取值为 undefined 时被误写成空字符串导致开关被关闭。
      await saveChangedKeys(
        values,
        SITE_KEYS.filter((key) =>
          key !== 'auto_exclude_days'
          && key !== 'auto_delete_days'
          && key !== 'access_password'
          && key !== 'AUTO_AI_ANALYSIS_ENABLED',
        ),
      );
      await saveChangedKeys(values, DB_KEYS);

      // 单独处理"自动AI评分"开关：只在用户切换过时提交，且强制 true/false
      const nextAutoAI = normalizeBooleanSetting(values.AUTO_AI_ANALYSIS_ENABLED, true);
      const prevAutoAI = normalizeBooleanSetting(initialValues.AUTO_AI_ANALYSIS_ENABLED, true);
      if (nextAutoAI !== prevAutoAI) {
        await client.post('/config', { key: 'AUTO_AI_ANALYSIS_ENABLED', value: nextAutoAI });
      }

      const autoExcludeChanged =
        normalizeFormValue(values.auto_exclude_days) !== normalizeFormValue(initialValues.auto_exclude_days) ||
        normalizeFormValue(values.auto_delete_days) !== normalizeFormValue(initialValues.auto_delete_days);
      if (autoExcludeChanged) {
        await client.post('/settings/auto-exclude', {
          auto_exclude_days: values.auto_exclude_days || '3',
          auto_delete_days: values.auto_delete_days || '0',
        });
      }

      if (normalizeFormValue(nextPassword) !== normalizeFormValue(initialValues.access_password) && nextPassword !== currentPassword) {
        await client.post('/config', { key: 'access_password', value: nextPassword });
      }
      if (nextPassword.trim()) {
        accessPasswordStore.set(nextPassword.trim());
      } else {
        accessPasswordStore.clear();
      }
      message.success('系统设置已保存');
      await refresh();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleVerifySubmit = async () => {
    const values = await passwordForm.validateFields();
    const success = await verifyPassword(values.password || '');
    if (!success) {
      passwordForm.setFields([{ name: 'password', errors: ['访问密码错误'] }]);
      return;
    }
    passwordForm.resetFields();
  };

  const handleLogout = () => {
    accessPasswordStore.clear();
    form.resetFields();
    passwordForm.setFieldsValue({ password: '' });
    setAuthStatus('required');
    message.success('已退出系统设置验证');
  };

  return (
    <div className="page-container" style={{ maxWidth: 1100, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Card
        loading={loading || authStatus === 'checking'}
        title={<span className="page-title-star">系统设置</span>}
        extra={isVerified ? <Button onClick={handleLogout}>退出设置验证</Button> : null}
      >
        {isVerified ? (
          <Form form={form} layout="vertical">
            <Tabs
              items={[
                {
                  key: 'ai',
                  label: 'AI设置',
                  children: (
                    <div style={{ display: 'grid', gap: 16 }}>
                      <Form.Item name="RELAY_AI_BASE_URL" label="AI 服务地址"><Input /></Form.Item>
                      <Form.Item name="RELAY_AI_API_KEY" label="AI API Key"><Input.Password /></Form.Item>
                      <Form.Item name="RELAY_AI_MODEL" label="普通模型">
                        <Select
                          options={(models || []).map((model) => ({
                            value: model.model_key,
                            label: `${model.model_name} (${model.model_key})`,
                          }))}
                          showSearch
                          optionFilterProp="label"
                          placeholder="选择普通分析模型"
                        />
                      </Form.Item>
                      <Form.Item name="RELAY_AI_FILE_MODEL" label="文件模型">
                        <Select
                          options={buildFileModelOptions(models || [])}
                          placeholder="选择文件分析模型"
                          showSearch
                          optionFilterProp="label"
                        />
                      </Form.Item>
                      <Form.Item name="AI_PROVIDER" label="AI 提供者">
                        <Select
                          options={[
                            { value: 'ctyun', label: 'CTYun 直连（直接访问 eaichat.ctyun.cn）' },
                            { value: 'relay', label: 'Relay 中转（通过中转服务器）' },
                          ]}
                          placeholder="选择 AI 提供者"
                        />
                      </Form.Item>
                      <Form.Item name="CTYUN_MODEL" label="CTYun 默认模型">
                        <Select
                          placeholder="选择 CTYun 模型"
                          options={[
                            { value: 'TEXT_DEEPSEEK_V4', label: 'DeepSeek V4 (默认)' },
                            { value: 'TEXT_A14', label: 'GLM-5' },
                            { value: 'TEXT_A22', label: '千问 3.5 plus' },
                            { value: 'TEXT_A13', label: '千问 3 30B' },
                            { value: 'TEXT_A8', label: 'DeepSeek V3.2' },
                          ]}
                        />
                      </Form.Item>
                      <Form.Item name="ai_prompt" label="当前 AI 提示词"><Input.TextArea rows={12} /></Form.Item>
                    </div>
                  ),
                },
                {
                  key: 'wechat',
                  label: '微信设置',
                  children: (
                    <div style={{ display: 'grid', gap: 16 }}>
                      <Form.Item name="WECHAT_HOOK_URL" label="微信 Hook 地址"><Input /></Form.Item>
                      <Form.Item name="WECHAT_NOTICE_BASE_URL" label="通知链接域名"><Input placeholder="例如 https://your-domain.com" /></Form.Item>
                      <Form.Item name="WECHAT_HIGH_SCORE_THRESHOLD" label="高分推送阈值"><Input /></Form.Item>
                      <Form.Item name="WECHAT_HIGH_SCORE_ROOM" label="高分微信群 ID"><Input /></Form.Item>
                      <Form.Item name="WECHAT_DEFAULT_ROOM" label="默认微信群 ID"><Input /></Form.Item>
                    </div>
                  ),
                },
                {
                  key: 'site',
                  label: '网站设置',
                  children: (
                    <div style={{ display: 'grid', gap: 16 }}>
                      <Form.Item name="access_password" label="访问密码"><Input.Password placeholder="为空表示不启用访问密码" /></Form.Item>
                      <Form.Item name="AUTO_AI_ANALYSIS_ENABLED" label="自动评分开关" valuePropName="checked">
                        <Switch checkedChildren="开启" unCheckedChildren="关闭" />
                      </Form.Item>
                      <Form.Item name="auto_exclude_days" label="自动排除多少天前的招标文件"><Input /></Form.Item>
                      <Form.Item name="auto_delete_days" label="自动删除多少天前抓取的招标数据"><Input placeholder="0 表示不自动删除" /></Form.Item>
                      <Form.Item name="LISTEN_ADDR" label="监听地址"><Input /></Form.Item>
                      <Form.Item name="PORT" label="端口"><Input /></Form.Item>
                    </div>
                  ),
                },
                {
                  key: 'db',
                  label: '数据库设置',
                  children: (
                    <div style={{ display: 'grid', gap: 16 }}>
                      <Form.Item name="DB_HOST" label="数据库地址"><Input /></Form.Item>
                      <Form.Item name="DB_PORT" label="数据库端口"><Input /></Form.Item>
                      <Form.Item name="DB_NAME" label="数据库名"><Input /></Form.Item>
                      <Form.Item name="DB_USER" label="数据库用户"><Input /></Form.Item>
                      <Form.Item name="DB_PASSWORD" label="数据库密码"><Input.Password /></Form.Item>
                    </div>
                  ),
                },
              ]}
            />
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <Typography.Text type="secondary">
                {data?.ai_logic || '系统当前仅使用单一 AI。数据库和网站监听配置保存后通常需要重启服务。'}
              </Typography.Text>
              <Button type="primary" onClick={handleSave} loading={saving}>
                保存设置
              </Button>
            </Space>
          </Form>
        ) : (
          <Typography.Text type="secondary">
            系统设置已开启密码保护，请先完成验证。
          </Typography.Text>
        )}
      </Card>
      <Modal
        title="系统设置验证"
        open={authStatus === 'required'}
        onCancel={() => {}}
        footer={null}
        closable={false}
        maskClosable={false}
        keyboard={false}
      >
        <Typography.Paragraph type="secondary">
          请输入系统设置访问密码后再进入此页面。
        </Typography.Paragraph>
        <Form form={passwordForm} layout="vertical" onFinish={handleVerifySubmit}>
          <Form.Item
            name="password"
            label="访问密码"
            rules={[{ required: true, message: '请输入访问密码' }]}
          >
            <Input.Password
              autoFocus
              onPressEnter={() => void handleVerifySubmit()}
              placeholder="请输入系统设置密码"
            />
          </Form.Item>
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Button type="primary" htmlType="submit" block loading={verifying}>
              验证并进入
            </Button>
            <Button block onClick={() => window.history.back()}>
              退出
            </Button>
          </Space>
        </Form>
      </Modal>
    </div>
  );
}
