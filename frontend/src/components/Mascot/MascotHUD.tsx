import React, { useEffect, useState } from 'react';
import { Skeleton } from 'antd';
import { client } from '@/api/client';

interface MascotHUDProps {
  bidId: string;
}

/**
 * 小招情报速递 — 详情页顶部悬浮 HUD
 * 首次打开调用 /api/bids/insight 触发 AI 生成并缓存
 * 后续访问直接读取数据库缓存
 */
export default function MascotHUD({ bidId }: MascotHUDProps) {
  const [insight, setInsight] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    client.get<{ data: { insight: string; cached: string } }>(`/bids/insight?id=${encodeURIComponent(bidId)}`)
      .then(res => {
        if (!cancelled && res?.data?.insight) {
          setInsight(res.data.insight);
        }
      })
      .catch(() => {
        if (!cancelled) setInsight('小招暂时无法连接智脑核心 ✦');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, [bidId]);

  if (loading) {
    return (
      <div style={{
        background: 'rgba(20, 20, 35, 0.85)',
        backdropFilter: 'blur(12px)',
        borderRadius: 10,
        border: '1px solid rgba(189, 147, 249, 0.25)',
        padding: '10px 16px',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
      }}>
        <span style={{ fontSize: 22 }}>🤖</span>
        <Skeleton.Input active size="small" style={{ width: 200 }} />
      </div>
    );
  }

  if (!insight) return null;

  return (
    <div style={{
      background: 'rgba(20, 20, 35, 0.85)',
      backdropFilter: 'blur(12px)',
      borderRadius: 10,
      border: '1px solid rgba(189, 147, 249, 0.25)',
      padding: '10px 16px',
      display: 'flex',
      alignItems: 'flex-start',
      gap: 10,
      animation: 'fade-in-up 0.4s ease-out',
    }}>
      <span style={{ fontSize: 22, flexShrink: 0, lineHeight: 1.4 }}>🤖</span>
      <div>
        <div style={{
          fontSize: 11,
          color: 'var(--anime-purple)',
          opacity: 0.7,
          marginBottom: 2,
          letterSpacing: 1,
        }}>
          ✦ 小招情报速递
        </div>
        <div style={{
          fontSize: 13,
          color: '#ddd',
          lineHeight: 1.6,
        }}>
          {insight}
        </div>
      </div>
    </div>
  );
}
