import React from 'react';
import { useRequest } from 'ahooks';
import { Button, Card, Descriptions, Empty, Grid, Row, Col, Skeleton, Space, Spin, Typography, Timeline, Tag } from 'antd';
import { useNavigate, useParams } from 'react-router-dom';
import { client, unwrapData } from '@/api/client';
import { Bid } from '@/types/api';
import dayjs from 'dayjs';
import { CheckCircleOutlined, DownloadOutlined } from '@ant-design/icons';
import AnalysisInsights from './AnalysisInsights';
import PDFPreview from './PDFPreview';
import { prettyModelLabel } from '@/utils/model';

type ParsedField = { key: string; value: string };

function hasText(value?: string | null): value is string {
  return typeof value === 'string' && value.trim() !== '';
}

function normalizeComparableText(value?: string | null): string {
  if (!hasText(value)) return '';
  return value.replace(/\s+/g, ' ').trim();
}

function buildField(key: string, value?: string | null): ParsedField | null {
  return hasText(value) ? { key, value: value.trim() } : null;
}

export default function BidDetail() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.md;

  const { data, loading } = useRequest(async () => {
    if (!id) return null;
    const res = await client.get<{ data: Bid | null }>(`/bids/detail?id=${encodeURIComponent(id)}`);
    return unwrapData(res);
  }, { refreshDeps: [id] });

  if (loading) {
    return (
      <div className="page-container" style={{ padding: 24 }}>
        <Skeleton.Input active size="small" style={{ width: 120, marginBottom: 16 }} />
        <Card className="glass-card">
          <Skeleton.Input active style={{ width: '60%', marginBottom: 16 }} />
          <Skeleton active paragraph={{ rows: 2 }} />
          <div style={{ display: 'flex', gap: 16, marginTop: 24 }}>
            <div style={{ flex: 2 }}>
              <Skeleton active paragraph={{ rows: 4 }} />
            </div>
            <div style={{ flex: 1 }}>
              <Skeleton active paragraph={{ rows: 3 }} />
            </div>
          </div>
        </Card>
      </div>
    );
  }

  if (!data) {
    return <Empty description="项目不存在或已删除" />;
  }

  const region = [data.area, data.city, data.district].filter(hasText).join(' ');
  const basicFields: ParsedField[] = [
    buildField('项目编号', data.serial),
    buildField('采购方', data.buyer),
    buildField('行业', data.industry),
    buildField('地区', region),
    buildField('数据来源', data.source),
  ].filter((field): field is ParsedField => field !== null);
  const projectFields: ParsedField[] = [
    buildField('项目编码', data.project_code),
    buildField('项目名称', data.project_name),
    buildField('公告类型', data.public_type),
    buildField('一级分类', data.top_type),
    buildField('二级分类', data.sub_type),
    buildField('采购内容', data.purchasing),
  ].filter((field): field is ParsedField => field !== null);
  // 关键词拆分为独立标签
  const keywordTags = hasText(data.keywords)
    ? data.keywords!.split(/[;；,，\s]+/).filter(Boolean).map(kw => kw.trim())
    : [];
  const sourceFields: ParsedField[] = [
    buildField('抓取时间', data.fetch_time),
    buildField('源站链接', data.site),
  ].filter((field): field is ParsedField => field !== null);
  const timelineItems = [
    data.publish_time ? { label: '发布时间', value: dayjs(data.publish_time * 1000).format('YYYY-MM-DD HH:mm:ss') } : null,
    data.bid_end_time ? { label: '截标时间', value: dayjs(data.bid_end_time * 1000).format('YYYY-MM-DD HH:mm:ss') } : null,
    hasText(data.created_at) ? { label: '入库时间', value: dayjs(data.created_at).format('YYYY-MM-DD HH:mm:ss') } : null,
  ].filter((item): item is { label: string; value: string } => item !== null);
  type FileLink = {
    key: string;
    label: string;
    previewable: boolean;
    isPdf: boolean;
    externalURL?: string;
    externalLabel?: string;
  };
  const skipOriginalEntry = !!data.has_original_file && !data.has_original_pdf && !data.original_is_external;
  const fileLinks: FileLink[] = ([
    data.has_current_file
      ? {
          key: 'current',
          label: data.has_current_pdf ? '打开当前 PDF' : '打开当前文件',
          previewable: true,
          isPdf: !!data.has_current_pdf,
          externalURL: data.current_is_external ? data.current_external_url : undefined,
          externalLabel: data.current_external_label,
        }
      : null,
    data.has_original_file && !skipOriginalEntry
      ? {
          key: 'original',
          label: data.has_original_pdf ? '打开原始 PDF' : '打开原始文件',
          previewable: true,
          isPdf: !!data.has_original_pdf,
          externalURL: data.original_is_external ? data.original_external_url : undefined,
          externalLabel: data.original_external_label,
        }
      : null,
  ].filter(Boolean) as FileLink[]);
  const descriptionText = hasText(data.description) ? data.description.trim() : '';
  const detailHTML = hasText(data.detail) ? data.detail.trim() : '';
  const showDescriptionCard = hasText(descriptionText)
    && normalizeComparableText(descriptionText) !== normalizeComparableText(data.detail);
  const modelLabel = prettyModelLabel(data.ai_model);
  const hasPreviewContent = Boolean(fileLinks.length > 0 || hasText(detailHTML));
  const hasTimelineOrSource = timelineItems.length > 0 || sourceFields.length > 0;

  return (
    <Space direction="vertical" size={isMobile ? 12 : 16} style={{ width: '100%' }} className="page-container">
      {/* ═══ 顶部操作栏 + 预算英雄数据 ═══ */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
        <Button onClick={() => navigate(-1)}>返回项目大厅</Button>
        {data.budget > 0 && (
          <div style={{ textAlign: 'right' }}>
            <span style={{ fontSize: 13, color: 'rgba(0,0,0,0.45)' }}>预算金额 </span>
            <span className="neon-glow-text" style={{ fontSize: 24, fontWeight: 700 }}>
              ¥{data.budget.toLocaleString()}
            </span>
          </div>
        )}
      </div>

      {/* ═══ 主卡片 — 标题 + 标签 + 双栏内容 ═══ */}
      <Card
        className="glass-card"
        title={<span className="page-title-star" style={{ fontSize: isMobile ? 16 : 20 }}>{data.title}</span>}
        extra={
          <Space wrap size={[6, 6]}>
            {typeof data.ai_score === 'number' && data.ai_score > 0 && (
              <Tag className="cute-tag" color="success">AI 评分 {data.ai_score}</Tag>
            )}
            {hasText(data.ai_match_level) && <Tag className="cute-tag" color="processing">{data.ai_match_level}</Tag>}
            {hasText(data.ai_priority) && <Tag className="cute-tag" color="error">{data.ai_priority}</Tag>}
            {modelLabel && <Tag className="cute-tag" color="warning">{modelLabel}</Tag>}
          </Space>
        }
      >
        <Row gutter={[24, 24]}>
          {/* ═══ 左侧主信息区 ═══ */}
          <Col xs={24} lg={hasTimelineOrSource ? 16 : 24}>
            <Space direction="vertical" size={20} style={{ width: '100%' }}>
              {/* 基本信息 + 项目分类 — 霓虹左侧竖线区块 */}
              {(basicFields.length > 0 || projectFields.length > 0) && (
                <div className="info-block-neon">
                  <Descriptions
                    title="基本与分类情报"
                    column={{ xs: 1, sm: 2, md: 3 }}
                    size="small"
                    layout="vertical"
                  >
                    {basicFields.map(f => (
                      <Descriptions.Item key={f.key} label={f.key}>
                        <span className="text-break-defend">{f.value}</span>
                      </Descriptions.Item>
                    ))}
                    {projectFields.map(f => (
                      <Descriptions.Item key={f.key} label={f.key}>
                        <span className="text-break-defend">{f.value}</span>
                      </Descriptions.Item>
                    ))}
                  </Descriptions>
                  {keywordTags.length > 0 && (
                    <div style={{ marginTop: 12 }}>
                      <Space wrap size={[4, 4]}>
                        {keywordTags.map(kw => (
                          <Tag key={kw} className="neon-tag-sub">{kw}</Tag>
                        ))}
                      </Space>
                    </div>
                  )}
                  {hasText(data.wechat_sent_at) && (
                    <div style={{ marginTop: 12 }}>
                      <Tag color="cyan" icon={<CheckCircleOutlined />} style={{ borderRadius: 12 }}>
                        已推送 {dayjs(data.wechat_sent_at).format('MM-DD HH:mm')}
                      </Tag>
                    </div>
                  )}
                </div>
              )}

              {/* AI 分析 — 小招品牌区块 */}
              {data.ai_analysis && (
                <div style={{ borderTop: '1px dashed var(--border-line)', paddingTop: 16 }}>
                  <Typography.Title level={5} style={{ marginTop: 0, color: 'var(--anime-purple)' }}>
                    ✨ 小招的 AI 分析报告
                  </Typography.Title>
                  <AnalysisInsights
                    value={data.ai_analysis}
                    thinking={data.ai_thinking_process}
                    model={modelLabel}
                  />
                </div>
              )}
            </Space>
          </Col>

          {/* ═══ 右侧侧边栏 — 时间轴 + 溯源 ═══ */}
          {hasTimelineOrSource && (
            <Col xs={24} lg={8}>
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                {timelineItems.length > 0 && (
                  <Card size="small" title="⌛ 关键时间节点" bordered={false} className="glass-card">
                    <Timeline
                      items={timelineItems.map((item, idx) => ({
                        children: (
                          <div>
                            <Typography.Text strong style={{ fontSize: 13 }}>{item.label}</Typography.Text>
                            <br />
                            <Typography.Text style={{ fontSize: 12 }} type="secondary">{item.value}</Typography.Text>
                          </div>
                        ),
                      }))}
                    />
                  </Card>
                )}
                {sourceFields.length > 0 && (
                  <Card size="small" title="🔍 数据溯源" bordered={false} className="glass-card">
                    <Descriptions column={1} size="small">
                      {sourceFields.map((field) => (
                        <Descriptions.Item key={field.key} label={field.key}>
                          {field.value.startsWith('http') ? (
                            <a href={field.value} target="_blank" rel="noreferrer" style={{ wordBreak: 'break-all' }}>{field.value}</a>
                          ) : field.value}
                        </Descriptions.Item>
                      ))}
                    </Descriptions>
                  </Card>
                )}
              </Space>
            </Col>
          )}
        </Row>
      </Card>

      {/* ═══ 下方 — 文件预览 + 正文 ═══ */}
      {hasPreviewContent && (
        <Card size="small" title="📄 公告原文与文件" bordered={false} className="glass-card">
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {fileLinks.length > 0 && (
              <Space wrap>
                {fileLinks.map((item) => (
                  <Button
                    key={item.key}
                    type="primary"
                    icon={<DownloadOutlined />}
                    href={item.externalURL || `/api/pdfProxy?id=${encodeURIComponent(data.id)}&file=${encodeURIComponent(item.key)}`}
                    target="_blank"
                    block={isMobile}
                  >
                    {item.externalURL ? `在 ${item.externalLabel || '外部平台'} 打开` : item.label}
                  </Button>
                ))}
              </Space>
            )}
            <PDFPreview
              bidId={data.id}
              links={fileLinks.map(({ key, label, isPdf, externalURL, externalLabel }) => ({ key, label, isPdf, externalURL, externalLabel }))}
              htmlContent={detailHTML}
              height={isMobile ? 420 : 720}
              isMobile={isMobile}
            />
          </Space>
        </Card>
      )}

      {showDescriptionCard && (
        <Card size="small" title="📝 正文" bordered={false} className="glass-card">
          <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.75 }}>{descriptionText}</div>
        </Card>
      )}
    </Space>
  );
}
