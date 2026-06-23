import React, { useState } from 'react';
import { Alert, Card, Col, Descriptions, Empty, Grid, Modal, Progress, Row, Space, Statistic, Tag, Typography } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, BulbOutlined } from '@ant-design/icons';
import { prettyModelLabel } from '@/utils/model';

type AnalysisInsightsProps = {
  value?: string;
  thinking?: string;
  model?: string;
};

type AnalysisSummary = {
  score?: number;
  matchLevel?: string;
  priority?: string;
  suitable?: boolean;
  recommendation?: string;
  dimensionScores: Array<{ label: string; score: number }>;
  advantages: string[];
  reasons: string[];
  risks: string[];
  extras: Array<{ label: string; value: React.ReactNode }>;
  rawText?: string;
};

const labelMap: Record<string, string> = {
  score: 'AI评分',
  ai_score: 'AI评分',
  matchlevel: '匹配度',
  match_level: '匹配度',
  ai_match_level: '匹配度',
  priority: '优先级',
  ai_priority: '优先级',
  suitable: '适配结论',
  ai_suitable: '适配结论',
  recommendation: '投标建议',
  suggestion: '投标建议',
  advice: '投标建议',
  summary: '结论',
  conclusion: '结论',
  reason: '补充说明',
  reasons: '匹配理由',
  advantages: '项目优势',
  risk: '潜在风险',
  risks: '潜在风险',
  dimensionscores: '维度评分',
};

function prettifyKey(rawKey: string): string {
  const trimmed = rawKey.trim();
  const normalized = trimmed.toLowerCase().replace(/[\s-]+/g, '_');
  return labelMap[normalized] || trimmed;
}

function parseMaybeJSON(value: unknown): unknown {
  if (typeof value !== 'string') return value;

  const text = value.trim();
  if (!text) return '';

  if (text === 'true') return true;
  if (text === 'false') return false;
  if (/^-?\d+(\.\d+)?$/.test(text)) return Number(text);

  if (
    (text.startsWith('[') && text.endsWith(']'))
    || (text.startsWith('{') && text.endsWith('}'))
  ) {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }

  return text;
}

function normalizeList(value: unknown): string[] {
  const parsed = parseMaybeJSON(value);

  if (Array.isArray(parsed)) {
    return parsed
      .map((item) => (typeof item === 'string' ? item.trim() : JSON.stringify(item)))
      .filter(Boolean);
  }

  if (typeof parsed === 'string' && parsed.trim()) {
    return parsed
      .split(/\n+/)
      .map((item) => item.trim().replace(/^[\-\d\.\)\s]+/, ''))
      .filter(Boolean);
  }

  return [];
}

function normalizeBoolean(value: unknown): boolean | undefined {
  const parsed = parseMaybeJSON(value);
  if (typeof parsed === 'boolean') return parsed;
  if (typeof parsed === 'number') return parsed > 0;
  return undefined;
}

function normalizeNumber(value: unknown): number | undefined {
  const parsed = parseMaybeJSON(value);
  return typeof parsed === 'number' && Number.isFinite(parsed) ? parsed : undefined;
}

function normalizeText(value: unknown): string | undefined {
  const parsed = parseMaybeJSON(value);
  return typeof parsed === 'string' && parsed.trim() ? parsed.trim() : undefined;
}

function normalizeDimensionScores(value: unknown): Array<{ label: string; score: number }> {
  const parsed = parseMaybeJSON(value);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return [];

  return Object.entries(parsed as Record<string, unknown>)
    .map(([key, itemValue]) => {
      const score = normalizeNumber(itemValue);
      if (score === undefined) return null;
      return {
        label: prettifyKey(key),
        score: Math.max(0, Math.min(100, score)),
      };
    })
    .filter((item): item is { label: string; score: number } => item !== null);
}

function parseAnalysisText(input?: string): AnalysisSummary {
  const emptyState: AnalysisSummary = {
    dimensionScores: [],
    advantages: [],
    reasons: [],
    risks: [],
    extras: [],
  };

  if (!input || !input.trim()) return emptyState;

  let parsed: unknown = null;
  try {
    parsed = JSON.parse(input);
  } catch {
    const fields: Record<string, unknown> = {};
    for (const line of input.split('\n').map((item) => item.trim()).filter(Boolean)) {
      const separator = line.includes('：') ? '：' : line.includes(':') ? ':' : '';
      if (!separator) continue;
      const index = line.indexOf(separator);
      const key = line.slice(0, index).trim();
      const value = line.slice(index + 1).trim();
      if (key && value) fields[key] = value;
    }
    parsed = Object.keys(fields).length ? fields : null;
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { ...emptyState, rawText: input.trim() };
  }

  const record = parsed as Record<string, unknown>;
  const summary: AnalysisSummary = {
    score: normalizeNumber(record.ai_score ?? record.score),
    matchLevel: normalizeText(record.matchLevel ?? record.match_level ?? record.ai_match_level),
    priority: normalizeText(record.priority ?? record.ai_priority),
    suitable: normalizeBoolean(record.suitable ?? record.ai_suitable),
    recommendation: normalizeText(record.recommendation ?? record.suggestion ?? record.advice),
    dimensionScores: normalizeDimensionScores(record.dimensionScores ?? record.dimension_scores),
    advantages: normalizeList(record.advantages),
    reasons: normalizeList(record.reasons ?? record.reason),
    risks: normalizeList(record.risks ?? record.risk),
    extras: [],
  };

  const consumedKeys = new Set([
    'ai_score',
    'score',
    'matchLevel',
    'match_level',
    'ai_match_level',
    'priority',
    'ai_priority',
    'suitable',
    'ai_suitable',
    'recommendation',
    'suggestion',
    'advice',
    'dimensionScores',
    'dimension_scores',
    'advantages',
    'reasons',
    'reason',
    'risks',
    'risk',
  ]);

  summary.extras = Object.entries(record)
    .filter(([key]) => !consumedKeys.has(key))
    .map(([key, value]) => {
      const parsedValue = parseMaybeJSON(value);
      let node: React.ReactNode = null;

      if (Array.isArray(parsedValue)) {
        const items = normalizeList(parsedValue);
        if (!items.length) return null;
        node = (
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {items.map((item) => <li key={item}>{item}</li>)}
          </ul>
        );
      } else if (parsedValue && typeof parsedValue === 'object') {
        node = (
          <Space direction="vertical" size={4}>
            {Object.entries(parsedValue as Record<string, unknown>).map(([childKey, childValue]) => (
              <Typography.Text key={childKey}>
                {prettifyKey(childKey)}: {String(childValue)}
              </Typography.Text>
            ))}
          </Space>
        );
      } else if (typeof parsedValue === 'boolean') {
        node = parsedValue ? '是' : '否';
      } else if (parsedValue !== undefined && parsedValue !== null && String(parsedValue).trim()) {
        node = String(parsedValue);
      }

      if (!node) return null;
      return {
        label: prettifyKey(key),
        value: node,
      };
    })
    .filter((item): item is NonNullable<typeof item> => item !== null);

  return summary;
}

function getScoreStatusColor(score?: number): string {
  if (!score && score !== 0) return 'default';
  if (score >= 80) return 'green';
  if (score >= 60) return 'orange';
  return 'red';
}

function getPriorityColor(priority?: string): string {
  const text = priority?.trim();
  if (!text) return 'default';
  if (text.includes('高')) return 'red';
  if (text.includes('中')) return 'orange';
  if (text.includes('低')) return 'blue';
  return 'default';
}

function getMatchColor(matchLevel?: string): string {
  const text = matchLevel?.trim();
  if (!text) return 'default';
  if (text.includes('高')) return 'green';
  if (text.includes('中') || text.includes('适')) return 'blue';
  if (text.includes('低')) return 'orange';
  return 'default';
}

function simplifyModelLabel(model?: string): string {
  return prettyModelLabel(model);
}

function BulletCard({ title, items }: { title: string; items: string[] }) {
  if (!items.length) return null;

  return (
    <Card size="small" title={title} style={{ height: '100%' }}>
      <ul style={{ margin: 0, paddingLeft: 18 }}>
        {items.map((item) => (
          <li key={`${title}-${item}`} style={{ marginBottom: 8 }}>
            <Typography.Text>{item}</Typography.Text>
          </li>
        ))}
      </ul>
    </Card>
  );
}

export default function AnalysisInsights({ value, thinking, model }: AnalysisInsightsProps) {
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.md;
  const analysis = parseAnalysisText(value);
  const thinkingText = thinking ? String(thinking).trim() : '';
  const modelLabel = simplifyModelLabel(model);
  const [thinkingOpen, setThinkingOpen] = useState(false);
  const thinkingSnippet = thinkingText ? thinkingText.slice(0, 120) + (thinkingText.length > 120 ? '…' : '') : '';
  const hasStructuredContent = Boolean(
    analysis.score !== undefined
    || analysis.matchLevel
    || analysis.priority
    || analysis.suitable !== undefined
    || analysis.recommendation
    || analysis.dimensionScores.length
    || analysis.advantages.length
    || analysis.reasons.length
    || analysis.risks.length
    || analysis.extras.length
  );

  if (!hasStructuredContent && !analysis.rawText) {
    return <Empty description="暂无 AI 分析结果" />;
  }

  return (
    <Space direction="vertical" size={isMobile ? 12 : 16} style={{ width: '100%' }}>
      {(analysis.score !== undefined || analysis.matchLevel || analysis.priority || analysis.suitable !== undefined) && (
        <Card size="small" title="分析概览">
          <Space direction="vertical" size={isMobile ? 12 : 16} style={{ width: '100%' }}>
            <Row gutter={[12, 12]}>
              {analysis.score !== undefined && (
                <Col xs={24} sm={12} lg={6}>
                  <Statistic
                    title="AI评分"
                    value={analysis.score}
                    suffix="/ 100"
                    valueStyle={{ color: `var(--ant-color-${getScoreStatusColor(analysis.score)})` }}
                  />
                </Col>
              )}
              {analysis.matchLevel && (
                <Col xs={24} sm={12} lg={6}>
                  <Space direction="vertical" size={8}>
                    <Typography.Text type="secondary">匹配度</Typography.Text>
                    <Tag color={getMatchColor(analysis.matchLevel)} style={{ width: 'fit-content', margin: 0 }}>
                      {analysis.matchLevel}
                    </Tag>
                  </Space>
                </Col>
              )}
              {analysis.priority && (
                <Col xs={24} sm={12} lg={6}>
                  <Space direction="vertical" size={8}>
                    <Typography.Text type="secondary">优先级</Typography.Text>
                    <Tag color={getPriorityColor(analysis.priority)} style={{ width: 'fit-content', margin: 0 }}>
                      {analysis.priority}
                    </Tag>
                  </Space>
                </Col>
              )}
              {analysis.suitable !== undefined && (
                <Col xs={24} sm={12} lg={6}>
                  <Space direction="vertical" size={8}>
                    <Typography.Text type="secondary">适配结论</Typography.Text>
                    <Tag
                      color={analysis.suitable ? 'success' : 'error'}
                      icon={analysis.suitable ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
                      style={{ width: 'fit-content', margin: 0 }}
                    >
                      {analysis.suitable ? '适合投标' : '谨慎参与'}
                    </Tag>
                  </Space>
                </Col>
              )}
            </Row>
            {modelLabel && (
              <div>
                <Typography.Text type="secondary">使用模型</Typography.Text>
                <div style={{ marginTop: 6 }}>
                  <Tag color="blue" style={{ margin: 0 }}>
                    {modelLabel}
                  </Tag>
                </div>
              </div>
            )}
            {analysis.recommendation && (
              <Alert
                type={analysis.suitable === false ? 'warning' : 'success'}
                showIcon
                message="投标建议"
                description={analysis.recommendation}
              />
            )}
          </Space>
        </Card>
      )}

      {analysis.dimensionScores.length > 0 && (
        <Card size="small" title="维度评分">
          <Row gutter={[12, 12]}>
            {analysis.dimensionScores.map((item) => (
              <Col xs={24} md={12} key={item.label}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
                  <Typography.Text>{item.label}</Typography.Text>
                  <Typography.Text strong>{item.score}</Typography.Text>
                </div>
                <Progress percent={item.score} showInfo={false} strokeColor="#1677ff" size="small" />
              </Col>
            ))}
          </Row>
        </Card>
      )}

      {(analysis.advantages.length > 0 || analysis.reasons.length > 0 || analysis.risks.length > 0 || thinkingText) && (
        <Row gutter={[12, 12]}>
          {analysis.advantages.length > 0 && (
            <Col xs={24} lg={8}>
              <BulletCard title="项目优势" items={analysis.advantages} />
            </Col>
          )}
          {analysis.reasons.length > 0 && (
            <Col xs={24} lg={8}>
              <BulletCard title="匹配理由" items={analysis.reasons} />
            </Col>
          )}
          {analysis.risks.length > 0 && (
            <Col xs={24} lg={8}>
              <BulletCard title="潜在风险" items={analysis.risks} />
            </Col>
          )}
          {thinkingText && (
            <Col xs={24} lg={8}>
              <Card
                size="small"
                title="思考过程"
                hoverable
                style={{ cursor: 'pointer', height: '100%' }}
                onClick={() => setThinkingOpen(true)}
              >
                <Typography.Paragraph
                  ellipsis={{ rows: 3 }}
                  style={{ marginBottom: 0, fontSize: 13, color: '#8c8c8c' }}
                >
                  {thinkingSnippet}
                </Typography.Paragraph>
              </Card>
            </Col>
          )}
        </Row>
      )}

      {analysis.extras.length > 0 && (
        <Card size="small" title="补充信息">
          <Descriptions column={1} size="small">
            {analysis.extras.map((item) => (
              <Descriptions.Item key={item.label} label={item.label}>
                {item.value}
              </Descriptions.Item>
            ))}
          </Descriptions>
        </Card>
      )}

      {analysis.rawText && (
        <Card size="small" title="原始分析文本">
          <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
            {analysis.rawText}
          </Typography.Paragraph>
        </Card>
      )}

      {thinkingText && (
        <Modal
          title="思考过程"
          open={thinkingOpen}
          onCancel={() => setThinkingOpen(false)}
          footer={null}
          width={700}
        >
          <Typography.Paragraph style={{ whiteSpace: 'pre-wrap', lineHeight: 1.75, maxHeight: '60vh', overflow: 'auto' }}>
            {thinkingText}
          </Typography.Paragraph>
        </Modal>
      )}
    </Space>
  );
}
