import React from 'react';
import { useRequest } from 'ahooks';
import { Button, Card, Empty, Row, Col, Space } from 'antd';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { client, unwrapData } from '@/api/client';
import { CompanyAwardRecord } from '@/types/api';
import dayjs from 'dayjs';

export default function CompanyRecords() {
  const { company = '' } = useParams<{ company: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const companyName = decodeURIComponent(company || (location.state as { company?: string } | null)?.company || '');

  const { data, loading } = useRequest(async () => {
    if (!companyName) return { items: [], total: 0, company: '' };
    const qs = new URLSearchParams({ company: companyName, page: '1', size: '100' }).toString();
    return unwrapData(await client.get<{ data: { items: CompanyAwardRecord[]; total: number; company: string } }>(`/company-awards/records?${qs}`));
  }, { refreshDeps: [companyName] });

  if (!companyName) return <Empty description="缺少企业名称" />;

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }} className="page-container">
      <Button onClick={() => navigate(-1)}>返回企业库</Button>

      <Card title={<span className="page-title-star">{companyName} 中标记录</span>} loading={loading}>
        <Row gutter={[0, 12]}>
          {(data?.items || []).map((item) => (
            <Col key={item.id} span={24}>
              <Card
                size="small"
                title={<Link to={`/companies/record/${item.id}`}>{item.project_name}</Link>}
              >
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <div>中标方：{item.win_bidder || '-'}</div>
                  <div>中标金额：{item.win_price || '-'}</div>
                  <div style={{ color: 'var(--ant-color-text-secondary)' }}>
                    公告时间：{item.notice_time ? dayjs(item.notice_time).format('YYYY-MM-DD HH:mm:ss') : '-'}
                  </div>
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
      </Card>
    </Space>
  );
}
