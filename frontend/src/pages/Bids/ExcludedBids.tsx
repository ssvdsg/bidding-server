import React, { useState } from 'react';
import { Button, message, Space, Card, Row, Col } from 'antd';
import { useRequest } from 'ahooks';
import { client, unwrapData } from '@/api/client';
import { Bid } from '@/types/api';
import dayjs from 'dayjs';
import { Link } from 'react-router-dom';

export default function ExcludedBids() {
  const { data, loading, refresh } = useRequest(async () => unwrapData(
    await client.get<{ data: { items: Bid[]; total: number } }>('/bids/excluded?page=1&size=100'),
  ));
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  const handleRestore = async (id: string) => {
    try {
      await client.post('/restoreBid', { id });
      message.success('恢复成功');
      refresh();
    } catch (e) {
      message.error('恢复失败');
    }
  };

  const handleBatchDelete = async () => {
    if (!selectedRowKeys.length) return message.warning('请选择要彻底删除的项目');
    try {
      await client.post('/batchDelete', { ids: selectedRowKeys });
      message.success('批量删除成功');
      setSelectedRowKeys([]);
      refresh();
    } catch (e) {
      message.error('批量删除失败');
    }
  };

  return (
    <div className="page-container" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ fontSize: 13, color: '#999' }}>
        <span className="deco-star" style={{ marginRight: 6 }}>✦</span>
        已排除的项目 · 可恢复或彻底删除
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Space>
          <Button danger onClick={handleBatchDelete} disabled={!selectedRowKeys.length}>
            彻底删除所选
          </Button>
          <Button onClick={refresh}>刷新</Button>
        </Space>
      </div>
      <Row gutter={[0, 12]}>
        {(data?.items || []).map((item) => (
          <Col key={item.id} span={24}>
            <Card
              size="small"
              title={
                <Space size={8}>
                  <input
                    type="checkbox"
                    checked={selectedRowKeys.includes(item.id)}
                    onChange={(e) => {
                      setSelectedRowKeys((prev) => e.target.checked ? [...prev, item.id] : prev.filter((id) => id !== item.id));
                    }}
                  />
                  <Link to={`/bids/${item.id}`} style={{ color: 'inherit', fontWeight: 600 }}>
                    {item.title}
                  </Link>
                </Space>
              }
              bodyStyle={{ padding: 12 }}
            >
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <div>{item.buyer || '-'}</div>
                <div style={{ color: 'var(--ant-color-text-secondary)' }}>
                  {item.publish_time ? dayjs(item.publish_time * 1000).format('YYYY-MM-DD HH:mm') : '-'}
                </div>
                <Button type="link" size="small" style={{ padding: 0 }} onClick={() => handleRestore(item.id)}>
                  恢复
                </Button>
              </Space>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
}
