import React, { useMemo, useRef } from 'react';
import { useRequest } from 'ahooks';
import { Button, Card, Descriptions, Empty, Space, Spin, Tag, Typography } from 'antd';
import { useNavigate, useParams } from 'react-router-dom';
import { client, unwrapData } from '@/api/client';
import { CompanyAwardRecord } from '@/types/api';
import dayjs from 'dayjs';

// 公告类型 → 中文名 + 颜色
const BULLETIN_TYPE_MAP: Record<string, { label: string; color: string }> = {
  '0': { label: '招标公告', color: 'blue' },
  '1': { label: '变更公告', color: 'orange' },
  '2': { label: '中标候选人公示', color: 'gold' },
  '3': { label: '澄清答疑', color: 'cyan' },
  '4': { label: '中标结果公告', color: 'green' },
};

const FIELD_LABELS: Record<string, string> = {
  NOTICE_NAME: '公告标题',
  TENDER_PROJECT_NAME: '项目名称',
  TENDER_PROJECT_ID: '项目 ID',
  BULLETIN_TYPE_NAME: '公告类型',
  NOTICE_SEND_TIME: '公告发送时间',
  DOC_GET_END_TIME: '招标文件获取截止',
  BID_DOC_REFER_END_TIME: '投标文件递交截止',
  BID_OPEN_TIME: '开标时间',
  TenderMode: '招标方式',
  FundSources: '资金来源',
  TotalFund: '项目预算',
  WinPrice: '中标金额',
  TenderBidder: '招标人',
  TenderAgency: '招标代理',
  REGION_NAME: '地区',
  REGION_PROVINCE: '所在省份',
  NOTICE_INDUSTRIES_NAME: '行业',
  ExpertSort: '项目类型',
  TRADE_PLAT: '交易平台',
  SERVER_PLAT: '服务平台',
  NOTICE_MEDIA: '发布媒介',
  NOTICE_URL: '公告地址',
  SUPERVISE_DEPT_NAME: '监督部门',
  BidSectionClassifyName: '标段分类',
};

const FIELD_ORDER = [
  'NOTICE_NAME', 'BULLETIN_TYPE_NAME', 'TENDER_PROJECT_NAME', 'TENDER_PROJECT_ID',
  'TenderBidder', 'TenderAgency', 'TenderMode', 'FundSources', 'TotalFund', 'WinPrice',
  'REGION_NAME', 'REGION_PROVINCE', 'NOTICE_INDUSTRIES_NAME', 'ExpertSort', 'BidSectionClassifyName',
  'NOTICE_SEND_TIME', 'DOC_GET_END_TIME', 'BID_DOC_REFER_END_TIME', 'BID_OPEN_TIME',
  'TRADE_PLAT', 'SERVER_PLAT', 'NOTICE_MEDIA', 'NOTICE_URL', 'SUPERVISE_DEPT_NAME',
];

type BulletinSection = {
  index: string;
  raw: any;
  parsedJSON: Record<string, any> | null;
  noticeContent: string;
  bulletinType: string;
  noticeTime?: string;
  noticeName?: string;
  pdfUrl?: string;
  relateBulletinTitle?: string;
};

function safeParseJSON(value: unknown): Record<string, any> | null {
  if (!value) return null;
  if (typeof value !== 'string') return null;
  try { return JSON.parse(value); } catch { return null; }
}

function formatScalar(value: unknown): string {
  if (value === null || value === undefined || value === '') return '-';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'boolean') return value ? '是' : '否';
  return String(value).trim() || '-';
}

function buildSections(details?: string): BulletinSection[] {
  if (!details) return [];
  let parsed: any;
  try { parsed = JSON.parse(details); } catch { return []; }
  const dataNode = parsed?.data ?? parsed;
  if (!dataNode || typeof dataNode !== 'object') return [];
  const sections: BulletinSection[] = [];
  Object.keys(dataNode).sort().forEach((key) => {
    const node = (dataNode as any)[key];
    if (!node || typeof node !== 'object') return;
    const parsedJSON = safeParseJSON(node.bulletinJSON);
    sections.push({
      index: key, raw: node, parsedJSON,
      noticeContent: parsedJSON?.NOTICE_CONTENT || '',
      bulletinType: String(node.type ?? key),
      noticeTime: node.noticeSendTime,
      noticeName: node.bulletinName || parsedJSON?.NOTICE_NAME,
      pdfUrl: node.pdfUrl,
      relateBulletinTitle: node.relateBulletinTitle,
    });
  });
  return sections;
}

function HTMLNoticeFrame({ html }: { html: string }) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const wrapped = `<!DOCTYPE html><html><head><meta charset="utf-8"><base target="_blank"><style>body{margin:12px;font-family:"Microsoft YaHei","PingFang SC",sans-serif;color:#222;line-height:1.7;font-size:14px;background:#fff;}img{max-width:100%;}</style></head><body>${html}</body></html>`;

  const handleLoad = () => {
    if (!iframeRef.current) return;
    try {
      const doc = iframeRef.current.contentDocument || iframeRef.current.contentWindow?.document;
      if (doc?.body) {
        iframeRef.current.style.height = `${doc.body.scrollHeight + 30}px`;
      }
    } catch { /* cross-origin guard — falls back to minHeight */ }
  };

  return (
    <iframe
      ref={iframeRef}
      title="公告内容"
      srcDoc={wrapped}
      onLoad={handleLoad}
      style={{ width: '100%', minHeight: 200, border: '1px solid #f0f0f0', borderRadius: 8, background: '#fff' }}
      sandbox="allow-same-origin"
    />
  );
}

function SectionCard({ section }: { section: BulletinSection }) {
  const typeMeta = BULLETIN_TYPE_MAP[section.bulletinType] || { label: '其他公告', color: 'default' };
  const json = section.parsedJSON || {};
  const items = FIELD_ORDER
    .filter((key) => json[key] !== undefined && json[key] !== '')
    .map((key) => ({ key, label: FIELD_LABELS[key] || key, value: formatScalar(json[key]) }));
  const extras = Object.keys(json)
    .filter((k) => k !== 'NOTICE_CONTENT' && !FIELD_ORDER.includes(k) && FIELD_LABELS[k])
    .map((key) => ({ key, label: FIELD_LABELS[key], value: formatScalar(json[key]) }));
  const allItems = [...items, ...extras];

  return (
    <Card
      size="small"
      className="glass-card"
      bordered={false}
      title={
        <Space size={8}>
          <Tag className="cute-tag" color={typeMeta.color}>{typeMeta.label}</Tag>
          <span>{section.noticeName || '未命名公告'}</span>
        </Space>
      }
      extra={
        <Space size={8}>
          {section.noticeTime && (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {dayjs(section.noticeTime).format('YYYY-MM-DD HH:mm')}
            </Typography.Text>
          )}
          {section.pdfUrl && (
            <Button size="small" type="link" href={`https://www.cebpubservice.com/${section.pdfUrl}`} target="_blank" rel="noreferrer">
              查看 PDF
            </Button>
          )}
        </Space>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {section.relateBulletinTitle && (
          <Typography.Text type="secondary">关联公告：{section.relateBulletinTitle}</Typography.Text>
        )}
        {allItems.length > 0 && (
          <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small" bordered={false}>
            {allItems.map((item) => (
              <Descriptions.Item key={item.key} label={item.label}><span className="text-break-defend">{item.value}</span></Descriptions.Item>
            ))}
          </Descriptions>
        )}
        {section.noticeContent ? (
          <div>
            <Typography.Title level={5} style={{ marginTop: 0, marginBottom: 8 }}>公告正文</Typography.Title>
            <HTMLNoticeFrame html={section.noticeContent} />
          </div>
        ) : (
          <Typography.Text type="secondary">该公告无正文内容</Typography.Text>
        )}
      </Space>
    </Card>
  );
}

export default function CompanyAwardDetail() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data, loading } = useRequest(async () => {
    if (!id) return null;
    const res = await client.get<{ data: CompanyAwardRecord | null }>(`/company-awards/detail?id=${encodeURIComponent(id)}`);
    return unwrapData(res);
  }, { refreshDeps: [id] });
  const sections = useMemo(() => buildSections(data?.details), [data?.details]);

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', padding: '48px 0' }}><Spin /></div>;
  if (!data) return <Empty description="记录不存在或已删除" />;

  const hasWinPrice = data.win_price && data.win_price !== '-';

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }} className="page-container">
      {/* ═══ 顶部操作栏 + 中标金额英雄数据 ═══ */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <Space>
          <Button onClick={() => navigate(-1)}>返回记录列表</Button>
          {data.notice_url && (
            <Button type="primary" href={data.notice_url} target="_blank" rel="noreferrer">
              打开公告链接
            </Button>
          )}
        </Space>
        {hasWinPrice && (
          <div style={{ textAlign: 'right' }}>
            <span style={{ fontSize: 13, color: 'rgba(0,0,0,0.45)' }}>中标金额 </span>
            <span className="neon-glow-text" style={{ fontSize: 24, fontWeight: 700 }}>
              {data.win_price}
            </span>
          </div>
        )}
      </div>

      {/* ═══ 主卡片 — 项目名 + 数据库摘要 ═══ */}
      <Card className="glass-card" title={data.project_name}>
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <div className="info-block-neon">
            <Descriptions
              title="数据库摘要"
              column={{ xs: 1, sm: 2, md: 4 }}
              size="small"
              layout="vertical"
            >
              <Descriptions.Item label="企业名称"><span className="text-break-defend">{data.search_company || '-'}</span></Descriptions.Item>
              <Descriptions.Item label="公告编号"><span className="text-break-defend">{data.bulletin_id || '-'}</span></Descriptions.Item>
              <Descriptions.Item label="中标方"><span className="text-break-defend">{data.win_bidder || '-'}</span></Descriptions.Item>
              {!hasWinPrice && <Descriptions.Item label="中标金额">{data.win_price || '-'}</Descriptions.Item>}
              <Descriptions.Item label="公告时间">
                {data.notice_time ? dayjs(data.notice_time).format('YYYY-MM-DD HH:mm:ss') : '-'}
              </Descriptions.Item>
            </Descriptions>
          </div>

          {sections.length > 0 ? (
            sections.map((section) => <SectionCard key={section.index} section={section} />)
          ) : (
            <Card size="small" bordered={false} className="glass-card">
              <Empty description="该记录暂无可解析的公告详情" />
            </Card>
          )}
        </Space>
      </Card>
    </Space>
  );
}
