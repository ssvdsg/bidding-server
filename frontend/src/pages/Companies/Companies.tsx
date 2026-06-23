import React, { useEffect, useState } from 'react';
import { Card, Button, Input, Space, Tag, Modal, List, TimePicker, Form, message, Row, Col } from 'antd';
import { useRequest } from 'ahooks';
import { client, unwrapData } from '@/api/client';
import { SearchOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { Link, useSearchParams } from 'react-router-dom';
import { CompanyAwardRecord, CompanySummary } from '@/types/api';

// 战力评级：S/A/B/C 基于中标次数
function getPowerRank(count: number): { rank: string; color: string; glow: string } {
  if (count >= 100) return { rank: 'S', color: '#bd93f9', glow: '0 0 8px rgba(189,147,249,0.5)' };
  if (count >= 30)  return { rank: 'A', color: '#fbbf24', glow: '0 0 6px rgba(251,191,36,0.4)' };
  if (count >= 10)  return { rank: 'B', color: '#00f0ff', glow: '0 0 4px rgba(0,240,255,0.3)' };
  return { rank: 'C', color: '#888', glow: 'none' };
}

export default function Companies() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchKeyword, setSearchKeyword] = useState(searchParams.get('keyword') || '');
  const [settingsForm] = Form.useForm();
  const { data, loading, refresh } = useRequest(async () => {
    const qs = new URLSearchParams({ page: '1', size: '50', keyword: searchKeyword }).toString();
    return unwrapData(await client.get<{ data: { items: CompanySummary[]; total: number } }>(`/company-awards/companies?${qs}`));
  }, { refreshDeps: [searchKeyword] });
  const { data: autoFetchConfig, refresh: refreshAutoFetch } = useRequest(async () => {
    const res = await client.get<{ data: { company_auto_fetch_time: string } }>('/settings/company-auto-fetch');
    return unwrapData(res);
  });
  const { data: autoFetchStatus, refresh: refreshAutoFetchStatus } = useRequest(async () => {
    const res = await client.get<{ data: { running: boolean; started_at: string; finished_at: string; total_count: number; success_count: number; failed_count: number; last_message: string } }>('/company-awards/auto-fetch/status');
    return unwrapData(res);
  }, { pollingInterval: 5000 });

  const [searchModalVisible, setSearchModalVisible] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [searchResult, setSearchResult] = useState<CompanyAwardRecord[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);

  useEffect(() => {
    if (autoFetchConfig?.company_auto_fetch_time) {
      settingsForm.setFieldsValue({
        company_auto_fetch_time: dayjs(autoFetchConfig.company_auto_fetch_time, 'HH:mm'),
      });
    }
  }, [autoFetchConfig, settingsForm]);

  useEffect(() => {
    const keyword = searchParams.get('keyword') || '';
    setSearchKeyword(keyword);
  }, [searchParams]);

  const handleRealtimeSearch = async () => {
    if (!keyword) return;
    setSearchLoading(true);
    try {
      const res = unwrapData(await client.post<{ data: CompanyAwardRecord[] }>('/company-awards/search-realtime', { company: keyword }));
      setSearchResult(res || []);
    } finally {
      setSearchLoading(false);
    }
  };

  const saveAutoFetch = async (values: { company_auto_fetch_time: dayjs.Dayjs }) => {
    try {
      await client.post('/settings/company-auto-fetch', {
        company_auto_fetch_time: values.company_auto_fetch_time.format('HH:mm'),
      });
      message.success('企业库自动获取时间已保存');
      refreshAutoFetch();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '保存失败');
    }
  };

  const runAutoFetchNow = async () => {
    try {
      await client.post('/company-awards/auto-fetch/run', {});
      message.success('企业库全量更新已启动');
      refreshAutoFetchStatus();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '启动失败');
    }
  };

  return (
    <div className="page-container" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Card title={<span className="page-title-star">企业库设置</span>}>
        <Form form={settingsForm} layout="inline" onFinish={saveAutoFetch}>
          <Form.Item name="company_auto_fetch_time" label="每日自动获取时间" rules={[{ required: true }]}>
            <TimePicker format="HH:mm" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit">保存</Button>
          </Form.Item>
          <Form.Item>
            <Button type="primary" ghost onClick={runAutoFetchNow} disabled={!!autoFetchStatus?.running}>
              {autoFetchStatus?.running ? '正在运行中' : '立即运行'}
            </Button>
          </Form.Item>
          {autoFetchStatus?.running && (
            <Form.Item>
              <span style={{ color: 'var(--ant-color-text-secondary)' }}>
                已启动：{autoFetchStatus.started_at ? dayjs(autoFetchStatus.started_at).format('YYYY-MM-DD HH:mm:ss') : '-'}
              </span>
            </Form.Item>
          )}
        </Form>
        <div style={{ marginTop: 12, color: 'var(--ant-color-text-secondary)' }}>
          <div>最近结果：{autoFetchStatus?.last_message || '-'}</div>
          <div>最近开始：{autoFetchStatus?.started_at ? dayjs(autoFetchStatus.started_at).format('YYYY-MM-DD HH:mm:ss') : '-'}</div>
          <div>最近结束：{autoFetchStatus?.finished_at ? dayjs(autoFetchStatus.finished_at).format('YYYY-MM-DD HH:mm:ss') : '-'}</div>
          <div>总数 / 成功 / 失败：{autoFetchStatus?.total_count || 0} / {autoFetchStatus?.success_count || 0} / {autoFetchStatus?.failed_count || 0}</div>
        </div>
      </Card>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Input.Search
          placeholder="本地搜索企业..."
          prefix={<SearchOutlined />}
          style={{ width: 240 }}
          allowClear
          value={searchKeyword}
          onChange={(e) => setSearchKeyword(e.target.value)}
          onSearch={(value) => {
            const next = new URLSearchParams(searchParams);
            if (value) next.set('keyword', value);
            else next.delete('keyword');
            setSearchParams(next);
          }}
        />
        <Space>
          <Button type="primary" onClick={() => setSearchModalVisible(true)}>全网实时搜索</Button>
          <Button onClick={refresh} loading={loading}>刷新</Button>
        </Space>
      </div>

      <Row gutter={[0, 12]}>
        {(data?.items || []).map((item) => (
          <Col key={item.search_company} span={24}>
            <Card
              size="small"
              title={<span style={{ fontWeight: 600 }}>{item.search_company}</span>}
              extra={(() => {
                const r = getPowerRank(item.total_projects);
                return (
                  <span style={{
                    display: 'inline-flex', alignItems: 'center', gap: 6,
                    fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
                  }}>
                    <span style={{
                      display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                      width: 24, height: 24, borderRadius: 4,
                      background: r.color, color: '#fff',
                      fontSize: 13, fontWeight: 700,
                      boxShadow: r.glow,
                    }}>
                      {r.rank}
                    </span>
                    <span style={{ fontSize: 12, color: '#999' }}>{item.total_projects} 次</span>
                  </span>
                );
              })()}
            >
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <div>最近项目：{item.last_project_name || '-'}</div>
                <div>最近中标单位：{item.last_win_bidder || '-'}</div>
                <div>最近中标金额：{item.last_win_price || '-'}</div>
                <div style={{ color: 'var(--ant-color-text-secondary)' }}>
                  最近公告时间：{item.last_notice_time ? dayjs(item.last_notice_time).format('YYYY-MM-DD HH:mm:ss') : '-'}
                </div>
                <Link to={`/companies/${encodeURIComponent(item.search_company)}`} state={{ company: item.search_company }}>
                  <Button type="link" size="small" style={{ padding: 0 }}>查看全部中标记录</Button>
                </Link>
              </Space>
            </Card>
          </Col>
        ))}
      </Row>

      <Modal
        title="企业全网中标查询"
        open={searchModalVisible}
        onCancel={() => setSearchModalVisible(false)}
        footer={null}
        width={900}
      >
        <Space style={{ display: 'flex', marginBottom: 16 }}>
          <Input
            placeholder="输入企业全称..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onPressEnter={handleRealtimeSearch}
            style={{ width: 300 }}
          />
          <Button type="primary" onClick={handleRealtimeSearch} loading={searchLoading}>
            搜索
          </Button>
        </Space>

        <List
          loading={searchLoading}
          dataSource={searchResult}
          renderItem={(item) => (
            <List.Item
              actions={[
                <a key="view" href={item.notice_url} target="_blank" rel="noreferrer">查看公告</a>,
              ]}
            >
              <List.Item.Meta
                title={item.project_name}
                description={`中标方: ${item.win_bidder || '-'} | 金额: ${item.win_price || '-'} | 时间: ${item.notice_time || '-'}`}
              />
            </List.Item>
          )}
        />
      </Modal>
    </div>
  );
}
