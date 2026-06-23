import React, { useState } from 'react';
import { Button, message, Space, Tag, Input, Card, Row, Col } from 'antd';
import { useRequest } from 'ahooks';
import { client, unwrapData } from '@/api/client';
import { TrackedBid } from '@/types/api';
import dayjs from 'dayjs';
import { Link } from 'react-router-dom';

export default function Tracked() {
  const [search, setSearch] = useState('');
  const { data, loading, refresh } = useRequest(async () => {
    const qs = new URLSearchParams({ page: '1', pageSize: '50', search }).toString();
    return unwrapData(await client.get<{ data: { list: TrackedBid[]; total: number } }>(`/tracked-bids?${qs}`));
  }, { refreshDeps: [search] });

  const toggleTracking = async (id: string, isFetching: boolean) => {
    try {
      const action = isFetching ? 'stop-fetch' : 'start-fetch';
      await client.post(`/tracked-bids/${id}/${action}`, { is_active: !isFetching });
      message.success('状态已更新');
      refresh();
    } catch (e) {
      message.error('操作失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await client.delete(`/tracked-bids/${id}`);
      message.success('已删除跟踪');
      refresh();
    } catch (e) {
      message.error('删除失败');
    }
  };

  return (
    <div className="page-container" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ fontSize: 13, color: '#999', marginBottom: -4 }}>
        <span className="deco-star" style={{ marginRight: 6 }}>✦</span>
        追踪中标项目的实时动态
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
        <Input.Search
          placeholder="搜索标题/采购方/编号"
          allowClear
          style={{ width: 320 }}
          onSearch={setSearch}
        />
        <Button onClick={refresh}>刷新状态</Button>
      </div>
      <Row gutter={[0, 12]}>
        {(data?.list || []).map((item) => (
          <Col key={item.id} span={24}>
            <Card
              size="small"
              title={
                <Link to={`/bids/${item.id}`} style={{ color: 'inherit', fontWeight: 600 }}>
                  {item.title}
                </Link>
              }
              bodyStyle={{ padding: 12 }}
            >
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <div>
                  {item.track_completed ? <Tag>已结束</Tag> : item.winner_fetched ? <Tag color="green">已命中</Tag> : item.winner_fetch_enabled ? <Tag color="blue">追踪中</Tag> : <Tag>未启动</Tag>}
                </div>
                <div>{item.winner || '-'}</div>
                <div style={{ color: 'var(--ant-color-text-secondary)' }}>
                  {(item.winner_amount || 0) > 0 ? `¥${(item.winner_amount || 0).toLocaleString()}` : '-'} · {item.last_check_time ? dayjs(item.last_check_time).format('YYYY-MM-DD HH:mm') : '-'}
                </div>
                <Space size={12}>
                  <Button type="link" size="small" style={{ padding: 0 }} onClick={() => toggleTracking(item.id, !!item.winner_fetch_enabled)}>
                    {item.winner_fetch_enabled ? '停止监控' : '启动监控'}
                  </Button>
                  <Button type="link" size="small" danger style={{ padding: 0 }} onClick={() => handleDelete(item.id)}>
                    删除
                  </Button>
                </Space>
              </Space>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
}
