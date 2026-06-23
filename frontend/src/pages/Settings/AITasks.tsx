import React, { useEffect, useState } from 'react';
import { Table, Button, Space, Tag, Modal, Form, Input, Switch, message, Drawer, Select } from 'antd';
import { useRequest } from 'ahooks';
import { client, unwrapData } from '@/api/client';
import { AIRole, AIScheduledTask, AITaskHistory } from '@/types/api';
import dayjs from 'dayjs';

export default function AITasks() {
  const [form] = Form.useForm();
  const { data, loading, refresh } = useRequest(() => client.get<AIScheduledTask[]>('/ai/tasks'));
  const { data: roles } = useRequest(async () => unwrapData(await client.get<{ data: AIRole[] }>('/ai/roles')));
  const [formVisible, setFormVisible] = useState(false);
  const [historyVisible, setHistoryVisible] = useState(false);
  const [currentTask, setCurrentTask] = useState<AIScheduledTask | null>(null);
  const { data: historyData, loading: historyLoading } = useRequest(
    async () => currentTask ? await client.get<AITaskHistory[]>(`/ai/tasks/${currentTask.id}/history`) : [],
    { ready: !!currentTask && historyVisible, refreshDeps: [currentTask?.id, historyVisible] },
  );

  useEffect(() => {
    if (!formVisible) return;

    if (currentTask) {
      form.setFieldsValue(currentTask);
      return;
    }

    form.setFieldsValue({
      schedule_type: 'daily',
      is_active: true,
      data_source: 'bids',
    });
  }, [currentTask, form, formVisible]);

  const toggleTaskStatus = async (task: AIScheduledTask, checked: boolean) => {
    try {
      await client.post(`/ai/tasks/${task.id}/toggle`, { is_active: checked });
      message.success('状态已切换');
      refresh();
    } catch (e) {
      message.error('切换失败');
    }
  };

  const handleExecute = async (id: number) => {
    try {
      await client.post(`/ai/tasks/${id}/execute`, {});
      message.success('任务已触发执行');
    } catch (e) {
      message.error('执行失败');
    }
  };

  const handleSave = async (values: Partial<AIScheduledTask>) => {
    try {
      if (currentTask) {
        await client.put(`/ai/tasks/${currentTask.id}`, { ...currentTask, ...values });
      } else {
        await client.post('/ai/tasks', {
          schedule_type: 'daily',
          is_active: true,
          data_source: 'bids',
          ...values,
        });
      }
      message.success('保存成功');
      setFormVisible(false);
      setCurrentTask(null);
      form.resetFields();
      refresh();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '保存失败');
    }
  };

  const columns = [
    {
      title: '任务名称',
      dataIndex: 'task_name',
      render: (text: string) => <span style={{ fontWeight: 500 }}>{text}</span>,
    },
    {
      title: '状态',
      dataIndex: 'is_active',
      width: 100,
      render: (active: boolean, record: AIScheduledTask) => (
        <Switch checked={active} onChange={(checked) => toggleTaskStatus(record, checked)} />
      ),
    },
    {
      title: '调度规则',
      dataIndex: 'cron_expression',
      width: 150,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '执行结果',
      dataIndex: 'last_run_status',
      width: 120,
      render: (status: string) => (
        <Tag color={status === 'success' ? 'green' : status === 'failed' ? 'red' : 'default'}>
          {status || '未执行'}
        </Tag>
      ),
    },
    {
      title: '下次执行',
      dataIndex: 'next_run_at',
      width: 180,
      render: (ts: string) => ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '-',
    },
    {
      title: '操作',
      width: 250,
      render: (_: any, record: AIScheduledTask) => (
        <Space size="small">
          <Button type="link" onClick={() => handleExecute(record.id)}>执行</Button>
          <Button type="link" onClick={() => { setCurrentTask(record); setHistoryVisible(true); }}>记录</Button>
          <Button type="link" onClick={() => { setCurrentTask(record); setFormVisible(true); }}>编辑</Button>
        </Space>
      ),
    },
  ];

  return (
    <div className="page-container" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ fontSize: 13, color: '#999', marginBottom: 4 }}>
        <span className="deco-star" style={{ marginRight: 6 }}>✦</span>
        AI 自动任务看板
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Space>
          <Button type="primary" onClick={() => { setCurrentTask(null); form.resetFields(); setFormVisible(true); }}>新建自动任务</Button>
          <Button onClick={refresh}>刷新</Button>
        </Space>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data || []}
        loading={loading}
        pagination={{ pageSize: 20 }}
        size="middle"
      />

      <Modal
        title={currentTask ? '编辑自动任务' : '新建自动任务'}
        open={formVisible}
        onCancel={() => { setFormVisible(false); setCurrentTask(null); }}
        footer={null}
        width={760}
      >
         <Form
            form={form}
            layout="vertical"
            onFinish={handleSave}
          >
            <Form.Item label="任务名称" name="task_name" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item label="描述" name="description">
              <Input />
            </Form.Item>
            <Form.Item label="执行时间" name="schedule_time">
              <Input placeholder="例如 09:00" />
            </Form.Item>
            <Form.Item label="Cron 表达式" name="cron_expression">
              <Input placeholder="为空时后端会自动生成" />
            </Form.Item>
            <Form.Item label="调度类型" name="schedule_type">
              <Select options={[{ value: 'daily', label: '每天' }, { value: 'weekly', label: '每周' }, { value: 'monthly', label: '每月' }]} />
            </Form.Item>
            <Form.Item label="AI 角色" name="ai_role">
              <Select options={(roles || []).map(role => ({ value: role.role_key, label: role.role_name }))} />
            </Form.Item>
            <Form.Item label="任务提示词" name="prompt_override">
              <Input.TextArea rows={6} placeholder="为空时使用角色提示词；填写后将覆盖角色提示词" />
            </Form.Item>
            <Form.Item label="问题" name="question" rules={[{ required: true }]}>
              <Input.TextArea rows={5} />
            </Form.Item>
            <Form.Item label="微信群 ID" name="wechat_room_id">
              <Input />
            </Form.Item>
            <Form.Item label="启用微信发送" name="enable_wechat" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="任务启用" name="is_active" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Button type="primary" htmlType="submit" block>保存</Button>
         </Form>
      </Modal>

      <Drawer
        title="执行记录"
        placement="right"
        width={600}
        onClose={() => setHistoryVisible(false)}
        open={historyVisible}
      >
        <Table
          rowKey="id"
          loading={historyLoading}
          dataSource={historyData || []}
          pagination={{ pageSize: 10 }}
          columns={[
            {
              title: '开始时间',
              dataIndex: 'started_at',
              width: 160,
              render: (val: string) => val ? dayjs(val).format('YYYY-MM-DD HH:mm') : '-',
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 100,
              render: (status: string) => <Tag color={status === 'success' ? 'green' : status === 'failed' ? 'red' : 'blue'}>{status}</Tag>,
            },
            { title: '数据量', dataIndex: 'data_count', width: 80 },
            { title: '错误信息', dataIndex: 'error_message', ellipsis: true },
          ]}
        />
      </Drawer>
    </div>
  );
}
