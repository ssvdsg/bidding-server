import React, { useEffect, useMemo, useState } from 'react';
import { Button, Tag, Space, message, Tooltip, Input, Select, Drawer, Descriptions, Tabs, Form, Card, Row, Col, Modal, List, Typography } from 'antd';
import { useRequest } from 'ahooks';
import { client, unwrapData } from '@/api/client';
import { Bid, ReanalyzeDebugResult } from '@/types/api';
import { SearchOutlined, SyncOutlined, DeleteOutlined, WechatOutlined, FilePdfOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import AnalysisInsights from './AnalysisInsights';

export default function Bids() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [page, setPage] = useState(Number(searchParams.get('page') || 1));
  const [pageSize, setPageSize] = useState(Number(searchParams.get('pageSize') || 20));
  const [form] = Form.useForm();
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const quickDates = Array.from({ length: 3 }, (_, index) => dayjs().subtract(index, 'day'));
  const today = dayjs().format('YYYY-MM-DD');
  const selectedPublishDate = searchParams.get('publishDate');
  const [sourcePickerOpen, setSourcePickerOpen] = useState(false);
  const [sourceOptions, setSourceOptions] = useState<Bid[]>([]);
  const [reanalyzeLogOpen, setReanalyzeLogOpen] = useState(false);
  const [reanalyzeLog, setReanalyzeLog] = useState<ReanalyzeDebugResult | null>(null);
  
  // 详情抽屉状态
  const [detailVisible, setDetailVisible] = useState(false);
  const [currentBid, setCurrentBid] = useState<Bid | null>(null);

  useEffect(() => {
    form.setFieldsValue({
      keyword: searchParams.get('keyword') || undefined,
      region: searchParams.get('region') || undefined,
      source: searchParams.get('source') || undefined,
      minScore: searchParams.get('minScore') || undefined,
      buyer: searchParams.get('buyer') || undefined,
      publishDate: selectedPublishDate && selectedPublishDate !== 'all' ? selectedPublishDate : undefined,
    });
  }, [form, searchParams, selectedPublishDate]);

  useEffect(() => {
    setPage(Number(searchParams.get('page') || 1));
    setPageSize(Number(searchParams.get('pageSize') || 20));
  }, [searchParams]);

  useEffect(() => {
    const nextParams = new URLSearchParams(searchParams);
    let changed = false;
    if (nextParams.get('page') !== page.toString()) {
      nextParams.set('page', page.toString());
      changed = true;
    }
    if (nextParams.get('pageSize') !== pageSize.toString()) {
      nextParams.set('pageSize', pageSize.toString());
      changed = true;
    }
    if (!nextParams.get('publishDate')) {
      nextParams.set('publishDate', 'all');
      changed = true;
    }
    if (changed) {
      setSearchParams(nextParams, { replace: true });
    }
  }, [page, pageSize, searchParams, setSearchParams]);

  const { data, loading, refresh, run: searchRun } = useRequest(
    async (params: any = {}) => {
      // 清洗参数：剔除 null/undefined/""，避免 URLSearchParams 把 undefined 拼成字符串 "undefined"
      // （之前的 bug：日期快捷筛选会把 keyword/region 等 undefined 一并发到后端，触发 LIKE '%undefined%'）
      const merged: Record<string, string> = {
        pageNum: page.toString(),
        pageSize: pageSize.toString(),
        orderBy: 'ai_score',
        order: 'desc',
      };
      Object.entries(params).forEach(([key, value]) => {
        if (value === null || value === undefined) return;
        const str = String(value).trim();
        if (str === '' || str === 'undefined' || str === 'null') return;
        merged[key] = str;
      });
      const qs = new URLSearchParams(merged).toString();
      const response = await client.get<{ data: { list: Bid[]; total: number } }>(`/bids/list?${qs}`);
      return unwrapData(response);
    },
    { refreshDeps: [page, pageSize] }
  );

  // 把 form 值清成只剩有效字段的 plain object（剔除 undefined/null/空串/"undefined"）
  const sanitizeFilters = (input: Record<string, any>): Record<string, string> => {
    const out: Record<string, string> = {};
    Object.entries(input).forEach(([key, value]) => {
      if (value === null || value === undefined) return;
      const str = String(value).trim();
      if (str === '' || str === 'undefined' || str === 'null') return;
      out[key] = str;
    });
    return out;
  };

  const onSearch = () => {
    const cleanedValues = sanitizeFilters(form.getFieldsValue());
    const nextParams = new URLSearchParams();
    nextParams.set('page', '1');
    nextParams.set('pageSize', pageSize.toString());
    Object.entries(cleanedValues).forEach(([key, value]) => nextParams.set(key, value));
    setSearchParams(nextParams);
    searchRun({ ...cleanedValues, pageNum: '1', pageSize: pageSize.toString() });
  };

  const applyQuickFilter = (values: Record<string, string>) => {
    form.setFieldsValue({ ...form.getFieldsValue(), ...values });
    const merged = sanitizeFilters({ ...form.getFieldsValue(), ...values });
    const nextParams = new URLSearchParams();
    nextParams.set('page', '1');
    nextParams.set('pageSize', pageSize.toString());
    Object.entries(merged).forEach(([key, value]) => nextParams.set(key, value));
    setSearchParams(nextParams);
    searchRun({ ...merged, pageNum: '1', pageSize: pageSize.toString() });
  };

  const getScoreColor = (score: number) => {
    if (score >= 80) return 'green';
    if (score >= 60) return 'orange';
    return 'default';
  };

  // AI 是否已评分：分数 > 0 或者已有分析文本，才认为评分完成
  const isAIScored = (item: Bid) =>
    (Number(item.ai_score) || 0) > 0 || (item.ai_analysis ? item.ai_analysis.trim().length > 0 : false);

  // 返回 (label, color)，未评分时显示"待AI评分"
  const getSuitableTag = (item: Bid): { label: string; color: string } => {
    if (!isAIScored(item)) return { label: '待AI评分', color: 'default' };
    return item.ai_suitable === 1
      ? { label: '适合投标', color: 'green' }
      : { label: '不适合投标', color: 'red' };
  };

  const handleExclude = async (id: string) => {
    try {
      await client.post('/deleteBid', { id });
      message.success('已移至排除库');
      refresh();
    } catch (e) {
      message.error('操作失败');
    }
  };

  const handleBatchExclude = async () => {
    if (!selectedIds.length) return message.warning('请选择项目');
    try {
      await client.post('/batchExclude', { ids: selectedIds });
      message.success('批量排除成功');
      setSelectedIds([]);
      refresh();
    } catch (e) {
      message.error('批量排除失败');
    }
  };

  const handleReanalyze = async (id: string) => {
    message.loading({ content: '正在重新分析...', key: 'analyze' });
    try {
      const response = await client.post<{ data: ReanalyzeDebugResult }>('/reanalyzeBid', { id });
      setReanalyzeLog(unwrapData(response));
      setReanalyzeLogOpen(true);
      message.success({ content: '分析完成', key: 'analyze' });
      refresh();
    } catch (e) {
      message.error({ content: '分析失败', key: 'analyze' });
    }
  };

  const handleSendWechat = async (id: string) => {
    try {
      const response = await client.post<{ message?: string }>('/sendProjectToWechat', { id });
      message.success(response.message || '已推送到微信');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '推送失败');
    }
  };

  const showDetail = (bid: Bid) => {
    setCurrentBid(bid);
    setDetailVisible(true);
  };

  const groupedList = useMemo(() => {
    const groups: Bid[][] = [];
    for (const item of data?.list || []) {
      const matchedGroup = groups.find((group) => {
        const anchor = group[0];
        const sameTitle = (anchor.title || '').trim() === (item.title || '').trim();
        const withinTwoDays = Math.abs((anchor.publish_time || 0) - (item.publish_time || 0)) <= 2 * 24 * 60 * 60;
        return sameTitle && withinTwoDays;
      });

      if (!matchedGroup) {
        groups.push([item]);
        continue;
      }
      matchedGroup.push(item);
    }

    return groups.map((items) => {
      const sorted = [...items].sort((a, b) => {
        const chinaA = a.source === 'china' ? 1 : 0;
        const chinaB = b.source === 'china' ? 1 : 0;
        if (chinaA !== chinaB) return chinaB - chinaA;
        return (b.ai_score || 0) - (a.ai_score || 0);
      });
      return { ...sorted[0], sourceOptions: sorted };
    });
  }, [data?.list]);

  const openBid = (bid: Bid, mode: 'detail' | 'page') => {
    const options = bid.sourceOptions || [bid];
    if (options.length <= 1) {
      if (mode === 'detail') showDetail(options[0]);
      if (mode === 'page') navigate(`/bids/${options[0].id}`);
      return;
    }
    setSourceOptions(options);
    setSourcePickerOpen(true);
  };

  return (
    <div className="page-container" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ fontSize: 13, color: '#999', marginBottom: -4 }}>
        <span className="deco-star" style={{ marginRight: 6 }}>✦</span>
        浏览 · 筛选 · 分析招标项目
      </div>
      <Form form={form} layout="inline" style={{ rowGap: 16 }}>
        <Form.Item name="keyword">
          <Input placeholder="搜索项目标题..." prefix={<SearchOutlined />} style={{ width: 200 }} />
        </Form.Item>
        <Form.Item name="region">
          <Input placeholder="地区..." style={{ width: 150 }} />
        </Form.Item>
        <Form.Item name="source">
          <Input placeholder="来源..." style={{ width: 120 }} />
        </Form.Item>
        <Form.Item name="minScore">
          <Select placeholder="AI 分值" allowClear style={{ width: 120 }}>
            <Select.Option value="80">80分以上</Select.Option>
            <Select.Option value="60">60分以上</Select.Option>
            <Select.Option value="-1">已排除</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item name="buyer">
          <Input placeholder="搜索采购方..." style={{ width: 150 }} />
        </Form.Item>
        <Form.Item name="publishDate" hidden>
          <Input />
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={onSearch}>筛选</Button>
        </Form.Item>
        <Form.Item>
          <Button onClick={() => { form.resetFields(); onSearch(); }}>重置</Button>
        </Form.Item>
        <Form.Item>
          <Space>
            <Button onClick={() => applyQuickFilter({ region: '云南' })}>云南</Button>
            <Button onClick={() => applyQuickFilter({ minScore: '80' })}>高于80分</Button>
            <Button onClick={() => applyQuickFilter({ keyword: '烟' })}>烟</Button>
          </Space>
        </Form.Item>
        <Form.Item>
          <Space wrap>
            <Button
              type={!form.getFieldValue('publishDate') ? 'primary' : 'default'}
              onClick={() => applyQuickFilter({ publishDate: '' })}
            >
              全部
            </Button>
            {quickDates.map((date) => {
              const value = date.format('YYYY-MM-DD');
              const active = form.getFieldValue('publishDate') === value;
              return (
                <Button
                  key={value}
                  type={active ? 'primary' : 'default'}
                  onClick={() => applyQuickFilter({ publishDate: value })}
                >
                  {date.format('M-D')}
                </Button>
              );
            })}
          </Space>
        </Form.Item>
        <Form.Item style={{ marginLeft: 'auto', marginRight: 0 }}>
          <Space>
            <Button danger onClick={handleBatchExclude} disabled={!selectedIds.length}>
              批量排除
            </Button>
            <Button icon={<SyncOutlined />} onClick={refresh}>刷新</Button>
          </Space>
        </Form.Item>
      </Form>
      <Row gutter={[0, 12]}>
        {groupedList.map((item) => {
          const selected = selectedIds.includes(item.id);
          return (
            <Col key={item.id} span={24}>
              <Card
                size="small"
                style={{ borderColor: selected ? 'var(--ant-color-primary)' : undefined }}
                bodyStyle={{ padding: 12 }}
                title={
                  <Space size={8}>
                    <input
                      type="checkbox"
                      checked={selected}
                      onChange={(e) => {
                        setSelectedIds((prev) => e.target.checked ? [...prev, item.id] : prev.filter((id) => id !== item.id));
                      }}
                    />
                    <Tag color={getScoreColor(item.ai_score || 0)} style={{ marginInlineEnd: 0 }}>
                      AI {item.ai_score > 0 ? item.ai_score : '-'}
                    </Tag>
                    <Button type="link" style={{ padding: 0, height: 'auto', fontWeight: 600 }} onClick={() => openBid(item, 'page')}>
                      {item.title}
                    </Button>
                  </Space>
                }
              >
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <div>{item.buyer || '-'}</div>
                  <div style={{ color: 'var(--ant-color-text-secondary)' }}>
                    {item.budget > 0 ? `¥${item.budget.toLocaleString()}` : '预算未知'} · {item.publish_time ? dayjs(item.publish_time * 1000).format('YYYY-MM-DD') : '-'}
                  </div>
                  {(item.sourceOptions?.length || 0) > 1 && (
                    <div style={{ color: 'var(--ant-color-text-tertiary)', fontSize: 12 }}>
                      当前合并显示 {item.sourceOptions?.length} 个来源，默认优先 {item.source === 'china' ? 'china' : item.source}
                    </div>
                  )}
                  <Space size={12}>
                    <Button type="link" size="small" onClick={() => openBid(item, 'detail')} style={{ padding: 0 }}>预览</Button>
                    {(() => {
                      const suit = getSuitableTag(item);
                      return (
                        <Tag color={suit.color} style={{ margin: 0 }}>
                          {suit.label}
                        </Tag>
                      );
                    })()}
                    <Tooltip title="重新 AI 分析"><Button type="link" size="small" icon={<SyncOutlined />} onClick={() => handleReanalyze(item.id)} /></Tooltip>
                    <Tooltip title="推送至微信"><Button type="link" size="small" icon={<WechatOutlined />} onClick={() => handleSendWechat(item.id)} /></Tooltip>
                    <Button type="link" size="small" onClick={() => handleExclude(item.id)} danger style={{ padding: 0 }}>排除</Button>
                  </Space>
                </Space>
              </Card>
            </Col>
          );
        })}
      </Row>
      <div style={{ marginTop: 12, display: 'flex', justifyContent: 'center' }}>
        <Space>
          <Button disabled={page <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>上一页</Button>
          <span style={{ lineHeight: '32px' }}>第 {page} 页</span>
          <Button disabled={(data?.list?.length || 0) < pageSize} onClick={() => setPage((p) => p + 1)}>下一页</Button>
          <Button onClick={refresh} loading={loading} icon={<SyncOutlined />}>刷新列表</Button>
        </Space>
      </div>

      <Drawer
        title="项目详情"
        width={720}
        onClose={() => setDetailVisible(false)}
        open={detailVisible}
      >
        {currentBid && (
          <Tabs defaultActiveKey="info">
            <Tabs.TabPane tab="基础信息" key="info">
              <Descriptions column={1} bordered size="small">
                <Descriptions.Item label="项目编号">{currentBid.serial || '-'}</Descriptions.Item>
                <Descriptions.Item label="项目名称">
                  <a href={currentBid.site || '#'} target="_blank" rel="noreferrer">{currentBid.title}</a>
                </Descriptions.Item>
                <Descriptions.Item label="采购方">{currentBid.buyer || '-'}</Descriptions.Item>
                <Descriptions.Item label="预算金额">{currentBid.budget > 0 ? `¥${currentBid.budget.toLocaleString()}` : '-'}</Descriptions.Item>
                <Descriptions.Item label="行业">{currentBid.industry || '-'}</Descriptions.Item>
                <Descriptions.Item label="地区">{currentBid.area || '-'} {currentBid.city || '-'}</Descriptions.Item>
                <Descriptions.Item label="数据来源">{currentBid.source || '-'}</Descriptions.Item>
                <Descriptions.Item label="AI 模型">{currentBid.ai_model || '-'}</Descriptions.Item>
                <Descriptions.Item label="发布时间">{currentBid.publish_time ? dayjs(currentBid.publish_time * 1000).format('YYYY-MM-DD HH:mm:ss') : '-'}</Descriptions.Item>
              </Descriptions>
            </Tabs.TabPane>
            <Tabs.TabPane tab="AI 分析结果" key="ai">
              {currentBid.ai_analysis ? (
                <AnalysisInsights value={currentBid.ai_analysis} />
              ) : (
                <div style={{ textAlign: 'center', padding: 40, color: 'var(--ant-color-text-secondary)' }}>
                  暂无分析结果
                  <br />
                  <Button type="primary" style={{ marginTop: 16 }} onClick={() => handleReanalyze(currentBid.id)}>
                    立即执行 AI 分析
                  </Button>
                </div>
              )}
            </Tabs.TabPane>
            <Tabs.TabPane tab="源文件/正文" key="source">
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                {(currentBid.has_current_file || currentBid.has_original_file) && (
                  <Space wrap>
                    {currentBid.has_current_file && (
                      <Button
                        icon={<FilePdfOutlined />}
                        href={`/api/pdfProxy?id=${encodeURIComponent(currentBid.id)}&file=current`}
                        target="_blank"
                      >
                        {currentBid.has_current_pdf ? '打开当前 PDF' : '打开当前文件'}
                      </Button>
                    )}
                    {currentBid.has_original_file && (
                      <Button
                        icon={<FilePdfOutlined />}
                        href={`/api/pdfProxy?id=${encodeURIComponent(currentBid.id)}&file=original`}
                        target="_blank"
                      >
                        {currentBid.has_original_pdf ? '打开原始 PDF' : '打开原始文件'}
                      </Button>
                    )}
                  </Space>
                )}
                {(currentBid.description || currentBid.detail) && (
                  <div style={{ whiteSpace: 'pre-wrap', lineHeight: 1.7 }}>
                    {currentBid.description || currentBid.detail}
                  </div>
                )}
              </Space>
            </Tabs.TabPane>
          </Tabs>
        )}
      </Drawer>

      <Modal title="选择要查看的来源" open={sourcePickerOpen} onCancel={() => setSourcePickerOpen(false)} footer={null}>
        <List
          dataSource={sourceOptions}
          renderItem={(item) => (
            <List.Item
              actions={[
                <Button key="detail" type="link" onClick={() => { setSourcePickerOpen(false); showDetail(item); }}>预览</Button>,
                <Link key="page" to={`/bids/${item.id}`} onClick={() => setSourcePickerOpen(false)}>详情页</Link>,
              ]}
            >
              <List.Item.Meta
                title={`${item.source || '未知来源'}${item.source === 'china' ? '（优先）' : ''}`}
                description={`${item.buyer || '-'} | ${item.publish_time ? dayjs(item.publish_time * 1000).format('YYYY-MM-DD HH:mm') : '-'} | ${getSuitableTag(item).label}`}
              />
            </List.Item>
          )}
        />
      </Modal>

      <Modal
        title="重新分析日志"
        open={reanalyzeLogOpen}
        onCancel={() => setReanalyzeLogOpen(false)}
        footer={null}
        width={960}
      >
        {reanalyzeLog && (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Card size="small" title="基础信息">
              <Descriptions column={1} size="small">
                <Descriptions.Item label="项目ID">{reanalyzeLog.id}</Descriptions.Item>
                <Descriptions.Item label="项目编号">{reanalyzeLog.serial || '-'}</Descriptions.Item>
                <Descriptions.Item label="使用模型">{reanalyzeLog.ai_model || '-'}</Descriptions.Item>
                <Descriptions.Item label="分析时间">{reanalyzeLog.analyzed_at || '-'}</Descriptions.Item>
              </Descriptions>
            </Card>

            <Card size="small" title="发送给 AI 的请求">
              <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                {reanalyzeLog.prompt || '-'}
              </Typography.Paragraph>
            </Card>

            <Card size="small" title="AI 原始回复">
              <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                {reanalyzeLog.raw_response || '-'}
              </Typography.Paragraph>
            </Card>

            <Card size="small" title="思考过程">
              <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                {reanalyzeLog.thinking_process || '-'}
              </Typography.Paragraph>
            </Card>

            <Card size="small" title="解析后的字段">
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                {JSON.stringify(reanalyzeLog.analysis || {}, null, 2)}
              </pre>
            </Card>
          </Space>
        )}
      </Modal>
    </div>
  );
}
