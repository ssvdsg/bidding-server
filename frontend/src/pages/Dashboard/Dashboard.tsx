import React from 'react';
import { Card, Row, Col, Spin, Progress, Table, Tag, Skeleton } from 'antd';
import { useRequest } from 'ahooks';
import { client, unwrapData } from '@/api/client';
import { WorkerStatistics } from '@/types/api';
import dayjs from 'dayjs';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Cell,
  ResponsiveContainer, LineChart, Line, Area, AreaChart,
} from 'recharts';

/* ── 时间问候 ── */
function getGreeting(): string {
  const h = new Date().getHours();
  if (h < 6) return '夜深了还在工作吗？注意休息哦 🌙';
  if (h < 9) return '早上好！新的一天也要打起精神来 ✦';
  if (h < 12) return '上午好！适合投标的好时光 ☀️';
  if (h < 14) return '中午好～记得好好吃饭 🍱';
  if (h < 18) return '下午好！向着目标冲刺吧 ⚡';
  return '晚上好！今天也辛苦了 ✨';
}

/* ── 骨架屏 — 与真实布局等高同宽，零 Layout Shift ── */
function DashboardSkeleton() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* Hero 区域骨架 */}
      <Card className="glass-card" bordered={false}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          <Skeleton.Avatar active size={48} shape="circle" />
          <div style={{ flex: 1 }}>
            <Skeleton.Input active size="small" style={{ width: '60%', marginBottom: 8 }} />
            <Skeleton.Input active size="small" style={{ width: '40%' }} />
          </div>
        </div>
      </Card>

      {/* 核心指标行 */}
      <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', padding: '0 4px' }}>
        {[1, 2, 3, 4].map(i => (
          <Skeleton.Button key={i} active size="small" style={{ width: 100 }} />
        ))}
      </div>

      {/* 分析面板 — 两列 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card className="glass-card" bordered={false}>
            <Skeleton.Input active style={{ width: 120, marginBottom: 16 }} />
            <Skeleton.Input active size="small" style={{ width: '100%', marginBottom: 12 }} />
            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
              <Skeleton.Button active size="small" style={{ width: 80 }} />
              <Skeleton.Button active size="small" style={{ width: 80 }} />
              <Skeleton.Button active size="small" style={{ width: 80 }} />
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card className="glass-card" bordered={false}>
            <Skeleton.Input active style={{ width: 100, marginBottom: 16 }} />
            <div style={{ display: 'flex', gap: 24 }}>
              {[1, 2, 3].map(i => (
                <div key={i} style={{ flex: 1, textAlign: 'center' }}>
                  <Skeleton.Input active size="small" style={{ width: 40 }} />
                </div>
              ))}
            </div>
          </Card>
        </Col>
      </Row>

      {/* 趋势图表骨架 */}
      <Card className="glass-card" bordered={false}>
        <Skeleton.Input active style={{ width: 180, marginBottom: 16 }} />
        <Skeleton.Node active style={{ width: '100%', height: 280 }}>
          <div style={{ width: '100%', height: '100%' }} />
        </Skeleton.Node>
      </Card>

      {/* 地区分布 — 图表 + 表格 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card className="glass-card" bordered={false}>
            <Skeleton.Input active style={{ width: 160, marginBottom: 16 }} />
            <Skeleton.Node active style={{ width: '100%', height: 360 }}>
              <div style={{ width: '100%', height: '100%' }} />
            </Skeleton.Node>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card className="glass-card" bordered={false}>
            <Skeleton.Input active style={{ width: 100, marginBottom: 16 }} />
            {[1, 2, 3, 4, 5].map(i => (
              <Skeleton.Input key={i} active size="small" block style={{ marginBottom: 8 }} />
            ))}
          </Card>
        </Col>
      </Row>
    </div>
  );
}

/* ── 主组件 ── */
export default function Dashboard() {
  const { data: stats, loading } = useRequest(async () => unwrapData(
    await client.get<{ data: WorkerStatistics }>('/statistics'),
  ));

  if (loading || !stats) {
    return <DashboardSkeleton />;
  }

  const chartData = (stats.regions || []).map(item => ({ name: item.area, value: item.count }));
  const trendData = stats.trend || [];
  const scoreRatio = stats.total > 0 ? Math.round((stats.highPriority / stats.total) * 100) : 0;
  const suitableRatio = stats.total > 0 ? ((stats.suitable / stats.total) * 100).toFixed(1) : '0';

  return (
    <div className="fade-in-up" style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* ═══ 角色区 — 小招来啦 ═══ */}
      <div className="mascot-section">
        <div style={{ display: 'flex', alignItems: 'center', gap: 14, position: 'relative', zIndex: 1, flexWrap: 'nowrap' }}>
          {/* 角色头像 */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3, flexShrink: 0 }}>
            <div className="mascot-avatar" style={{ width: 48, height: 48, overflow: 'hidden' }}>
              <img
                src={import.meta.env.BASE_URL + 'assets/xiaozhao.jpg'}
                alt="小招"
                width={48}
                height={48}
                style={{ objectFit: 'cover', borderRadius: '50%' }}
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = 'none';
                  const p = (e.target as HTMLImageElement).parentElement;
                  if (p) p.innerHTML = '<svg width="48" height="48" viewBox="0 0 48 48"><circle cx="24" cy="24" r="23" fill="#fbbf24"/><circle cx="17" cy="20" r="4" fill="#333"/><circle cx="31" cy="20" r="4" fill="#333"/><circle cx="17" cy="20" r="1.5" fill="#fff"/><circle cx="31" cy="20" r="1.5" fill="#fff"/><path d="M16 31 Q24 38 32 31" stroke="#e84d4d" stroke-width="2.5" fill="none" stroke-linecap="round"/><circle cx="24" cy="10" r="5" fill="#ff6b9d" opacity="0.6"/></svg>';
                }}
              />
            </div>
            <span className="mascot-name-tag" style={{ fontSize: 10, padding: '1px 10px' }}>✦ 小招 AI</span>
          </div>

          {/* 对话气泡 + 数据行 */}
          <div style={{ flex: 1, minWidth: 0 }}>
            <div className="mascot-speech" style={{ maxWidth: 'none' }}>
              <span className="deco-star" style={{ marginRight: 4, fontSize: 11 }}>✦</span>
              {getGreeting()}
            </div>
            <div style={{ marginTop: 6, display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
              <span style={{ fontSize: 12, opacity: 0.7 }}>
                监控到 <strong style={{ color: '#fbbf24' }}>{stats.today}</strong> 条新标
              </span>
              <span style={{ fontSize: 11, opacity: 0.3 }}>|</span>
              <span style={{ fontSize: 12, opacity: 0.6 }}>
                推荐 <strong style={{ color: '#10b981' }}>{stats.suitable}</strong> 项
              </span>
            </div>
          </div>

          {/* 右侧数据概要 */}
          <div style={{
            background: 'rgba(255,255,255,0.06)',
            backdropFilter: 'blur(8px)',
            borderRadius: 8,
            padding: '6px 14px',
            fontSize: 11,
            opacity: 0.65,
            textAlign: 'right',
            flexShrink: 0,
          }}>
            <div style={{ fontSize: 18 }}>📊</div>
            <div>招标追踪系统</div>
            <div style={{ fontSize: 9, opacity: 0.4 }}>By: 000</div>
          </div>
        </div>
      </div>

      {/* ═══ 核心指标 — 彩色毛玻璃统计卡 ═══ */}
      <Row gutter={[16, 16]}>
        <Col xs={12} sm={6}>
          <div className="medal-card card-navy" onClick={() => window.location.href = '/bids'}>
            <div className="medal-card-top">
              <span className="medal-icon">📋</span>
              <span className="medal-label">项目总数</span>
            </div>
            <div className="medal-card-bottom">
              <span className="medal-value">{stats.total}</span>
            </div>
          </div>
        </Col>
        <Col xs={12} sm={6}>
          <div className="medal-card card-green" onClick={() => window.location.href = '/bids?minScore=70'}>
            <div className="medal-card-top">
              <span className="medal-icon">✅</span>
              <span className="medal-label">推荐项目</span>
            </div>
            <div className="medal-card-bottom">
              <span className="medal-value">{stats.suitable}</span>
            </div>
          </div>
        </Col>
        <Col xs={12} sm={6}>
          <div className="medal-card card-gold" onClick={() => window.location.href = '/bids?minScore=80'}>
            <div className="medal-card-top">
              <span className="medal-icon">⭐</span>
              <span className="medal-label">高优先级</span>
            </div>
            <div className="medal-card-bottom">
              <span className="medal-value">{stats.highPriority}</span>
            </div>
          </div>
        </Col>
        <Col xs={12} sm={6}>
          <div className="medal-card card-purple" onClick={() => window.location.href = `/bids?publishDate=${dayjs().format('YYYY-MM-DD')}`}>
            <div className="medal-card-top">
              <span className="medal-icon">🆕</span>
              <span className="medal-label">今日新增</span>
            </div>
            <div className="medal-card-bottom">
              <span className="medal-value">{stats.today}</span>
            </div>
          </div>
        </Col>
      </Row>

      {/* ═══ 分析面板 ═══ */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card
            className="glass-card"
            title={<span><span className="deco-star" style={{ marginRight: 6, fontSize: 12 }}>✦</span>高分项目占比</span>}
            bordered={false}
          >
            <Progress
              percent={scoreRatio}
              strokeColor={{ '0%': '#1a1a2e', '100%': '#6d28d9' }}
              trailColor="#ede4d8"
              size="small"
            />
            <div style={{ marginTop: 14, display: 'flex', justifyContent: 'space-between', fontSize: 13, color: '#888' }}>
              <span>高优先级占全部项目</span>
              <span style={{ color: '#1a1a2e', fontWeight: 600 }}>{scoreRatio}%</span>
            </div>
            <div style={{
              marginTop: 12,
              background: 'linear-gradient(135deg, #f5f3ff, #f0fdf4)',
              borderRadius: 8,
              padding: '10px 14px',
              fontSize: 13,
              color: '#555',
            }}>
              <span style={{ color: '#059669' }}>●</span> 推荐率 <strong>{suitableRatio}%</strong>
              {' · '}
              <span style={{ color: '#d97706' }}>●</span> 非常适合投标的项目共 <strong>{stats.suitable}</strong> 项
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card
            className="glass-card"
            title={<span><span className="deco-star" style={{ marginRight: 6, fontSize: 12 }}>✦</span>适配一览</span>}
            bordered={false}
          >
            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', marginBottom: 14 }}>
              <Tag className="cute-tag" color="success">✦ 推荐投标: {stats.suitable}</Tag>
              <Tag className="cute-tag" color="error">✗ 不建议: {stats.notSuitable}</Tag>
              <Tag className="cute-tag" color="warning">◆ 重点关注: {stats.highPriority}</Tag>
            </div>
            <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap' }}>
              {[
                { label: '推荐占比', value: `${suitableRatio}%`, color: '#059669' },
                { label: '高优先比', value: `${scoreRatio}%`, color: '#d97706' },
                { label: '排除项目', value: stats.notSuitable, color: '#dc2626' },
              ].map(item => (
                <div key={item.label} style={{ textAlign: 'center', flex: 1, minWidth: 60 }}>
                  <div style={{ fontSize: 20, fontWeight: 700, color: item.color }}>{item.value}</div>
                  <div style={{ fontSize: 11, color: '#999', marginTop: 2 }}>{item.label}</div>
                </div>
              ))}
            </div>
          </Card>
        </Col>
      </Row>

      {/* ═══ 开标趋势（二次元版） ═══ */}
      <Card
        className="glass-card"
        title={<span><span className="deco-star" style={{ marginRight: 6, fontSize: 12 }}>✦</span>近期开标趋势 <span style={{ fontSize: 11, color: '#ccc', fontWeight: 400 }}>· 数据随时间绽放 ✿</span></span>}
        bordered={false}
      >
        <div style={{ height: 300, width: '100%' }}>
          <ResponsiveContainer>
            <AreaChart data={trendData} margin={{ top: 20, right: 20, left: 0, bottom: 8 }}>
              <defs>
                <linearGradient id="trendGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#00f0ff" stopOpacity={0.3} />
                  <stop offset="40%" stopColor="#bd93f9" stopOpacity={0.15} />
                  <stop offset="100%" stopColor="#00f0ff" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="2 2" vertical={false} stroke="rgba(26,26,46,0.05)" strokeWidth={0.5} />
              <XAxis dataKey="date" tick={{ fontSize: 11, fill: '#bbb', fontFamily: 'ZCOOL KuaiLe, sans-serif' }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 11, fill: '#bbb' }} axisLine={false} tickLine={false} />
              <Tooltip
                wrapperClassName="chart-tooltip-dark"
                contentStyle={{ borderRadius: 8, border: 'none', background: 'transparent' }}
                labelStyle={{ fontFamily: 'ZCOOL KuaiLe, sans-serif', fontSize: 12, color: '#a78bfa' }}
                itemStyle={{ color: '#00f0ff', fontWeight: 600, fontFamily: "'JetBrains Mono', 'Fira Code', monospace" }}
              />
              <Area type="monotone" dataKey="count" stroke="#00f0ff" strokeWidth={2.5} fill="url(#trendGrad)" dot={{ stroke: '#00f0ff', strokeWidth: 1, r: 3, fill: '#fff' }} activeDot={{ r: 5, strokeWidth: 0, fill: '#bd93f9' }} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </Card>

      {/* ═══ 地区分布（二次元版） ═══ */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card
            className="glass-card"
            title={<span><span className="deco-star" style={{ marginRight: 6, fontSize: 12 }}>✦</span>地区分布统计 <span style={{ fontSize: 11, color: '#ccc', fontWeight: 400 }}>· 每一朵都是投标之花 ✿</span></span>}
            bordered={false}
          >
            <div style={{ height: 380, width: '100%' }}>
              <ResponsiveContainer>
                <BarChart data={chartData} margin={{ top: 20, right: 16, left: 0, bottom: 50 }}>
                  <defs>
                    {['#ff6b9d', '#a78bfa', '#fbbf24', '#34d399', '#60a5fa', '#f472b6'].map((c, i) => (
                      <linearGradient key={i} id={`barGrad${i}`} x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor={c} stopOpacity={0.85} />
                        <stop offset="100%" stopColor={c} stopOpacity={0.3} />
                      </linearGradient>
                    ))}
                  </defs>
                  <CartesianGrid strokeDasharray="2 2" vertical={false} stroke="rgba(26,26,46,0.05)" strokeWidth={0.5} />
                  <XAxis dataKey="name" angle={-30} textAnchor="end" height={65} interval={0} tick={{ fontSize: 11, fill: '#bbb', fontFamily: 'ZCOOL KuaiLe, sans-serif' }} />
                  <YAxis tick={{ fontSize: 11, fill: '#bbb' }} axisLine={false} tickLine={false} />
                  <Tooltip
                    cursor={{ fill: 'rgba(24,144,255,0.08)' }}
                    wrapperClassName="chart-tooltip-dark"
                    contentStyle={{ borderRadius: 8, border: 'none', background: 'transparent' }}
                    labelStyle={{ fontFamily: 'ZCOOL KuaiLe, sans-serif', fontSize: 12, color: '#a78bfa' }}
                    itemStyle={{ color: '#00ffcc', fontWeight: 600 }}
                  />
                  <Bar dataKey="value" radius={[8, 8, 0, 0]} maxBarSize={40}>
                    {chartData.map((_entry, idx) => (
                      <Cell key={idx} fill={`url(#barGrad${idx % 6})`} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card
            className="glass-card"
            title={<span><span className="deco-star" style={{ marginRight: 6, fontSize: 12 }}>✦</span>地区明细</span>}
            bordered={false}
          >
            <Table
              rowKey="area"
              pagination={false}
              size="small"
              dataSource={stats.regions || []}
              columns={[
                { title: '📍 地区', dataIndex: 'area' },
                { title: '📊 项目数', dataIndex: 'count', align: 'right' },
              ]}
            />
          </Card>
        </Col>
      </Row>

      {/* 底部装饰分隔 */}
      <div style={{ textAlign: 'center', padding: '8px 0 4px', fontSize: 12, color: '#ccc', letterSpacing: 4 }}>
        ✦ ✦ ✦ ✦ ✦
      </div>
    </div>
  );
}
