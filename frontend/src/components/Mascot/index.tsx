import React, { useState, useRef, useEffect, useCallback } from 'react';
import { getRandomQuote, getMascotReaction, MascotStats } from './quotes';
import MascotChat from './chat';
import { client, unwrapData } from '@/api/client';

const POS_KEY = 'mascot_pos';
const DEFAULT_POS = { x: 24, y: 120 };

function loadPos() {
  try { return JSON.parse(localStorage.getItem(POS_KEY) || ''); }
  catch { return DEFAULT_POS; }
}
function savePos(pos: { x: number; y: number }) {
  try { localStorage.setItem(POS_KEY, JSON.stringify(pos)); } catch {}
}

export default function Mascot({ stats: externalStats }: { stats?: MascotStats }) {
  const [fetchedStats, setFetchedStats] = useState<MascotStats | undefined>();
  const stats = externalStats || fetchedStats;

  /* ── 自拉取 Dashboard 统计（供业务感知语录使用） ── */
  useEffect(() => {
    if (externalStats) return; // 外部已提供，不自拉
    let cancelled = false;
    const fetchStats = async () => {
      try {
        const res = await client.get<{ data: any }>('/statistics');
        const d = unwrapData(res);
        if (!cancelled && d) {
          setFetchedStats({
            total: d.total,
            today: d.today,
            suitable: d.suitable,
            highPriority: d.highPriority,
            pendingAI: d.pendingAI ?? d.pending_ai,
          });
        }
      } catch { /* silent */ }
    };
    fetchStats(); // 首次
    const timer = setInterval(fetchStats, 30000); // 每 30s 刷新
    return () => { cancelled = true; clearInterval(timer); };
  }, [externalStats]);

  const [pos, setPos] = useState(loadPos);
  const posRef = useRef(pos);
  const [bubble, setBubble] = useState<{ text: string; visible: boolean }>({ text: '', visible: false });
  const [chatOpen, setChatOpen] = useState(false);
  const [dragging, setDragging] = useState(false);
  const aiQuoteRef = useRef<string | null>(null); // AI 异步加载的每日语录

  const draggingRef = useRef(false);
  const movedRef = useRef(false);
  const dragStartRef = useRef({ x: 0, y: 0, posX: 0, posY: 0 });
  const rafRef = useRef<number | null>(null);
  const bubbleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => { posRef.current = pos; }, [pos]);

  /* ── 异步拉取 AI 每日语录（供气泡使用） ── */
  useEffect(() => {
    let cancelled = false;
    fetch('/api/mascot/daily')
      .then(r => r.ok ? r.json() : Promise.reject())
      .then((data: { quote?: string }) => {
        if (cancelled) return;
        if (data.quote) {
          aiQuoteRef.current = data.quote;
        }
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, []);

  /* ── 气泡（预设语录 + AI 异步补充） ── */
  const showBubble = useCallback((text: string) => {
    setBubble({ text, visible: true });
    if (bubbleTimerRef.current) clearTimeout(bubbleTimerRef.current);
    bubbleTimerRef.current = setTimeout(() => setBubble(prev => ({ ...prev, visible: false })), 4000);
  }, []);

  useEffect(() => {
    const welcome = setTimeout(() => showBubble('你好呀！我是小招AI ✦ 有需要随时点我～'), 2000);
    const q1 = getRandomQuote().text;
    const q2 = getRandomQuote().text;
    const q3 = getRandomQuote().text;
    const t1 = setTimeout(() => showBubble(q1), 25000);
    const t2 = setTimeout(() => showBubble(q2), 50000);
    const t3 = setTimeout(() => showBubble(q3), 75000);
    // 业务感知语录：看板娘灵动化，数据驱动台词
    const tBiz = setTimeout(() => {
      if (stats) {
        const reaction = getMascotReaction(stats);
        if (reaction) showBubble(reaction.text);
      }
    }, 60000);
    // AI 语录作为第 4 条惊喜（100s 后，此时 AI 已加载完成）
    const t4 = setTimeout(() => {
      if (aiQuoteRef.current) showBubble(aiQuoteRef.current);
    }, 100000);
    return () => { clearTimeout(welcome); clearTimeout(t1); clearTimeout(t2); clearTimeout(t3); clearTimeout(tBiz); clearTimeout(t4); if (bubbleTimerRef.current) clearTimeout(bubbleTimerRef.current); };
  }, [showBubble]);

  /* ── 拖拽 ── */
  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!draggingRef.current) return;
    const dx = e.clientX - dragStartRef.current.x;
    const dy = e.clientY - dragStartRef.current.y;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) movedRef.current = true;
    if (rafRef.current) cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(() => {
      setPos({
        x: Math.max(0, Math.min(window.innerWidth - 60, dragStartRef.current.posX + dx)),
        y: Math.max(0, Math.min(window.innerHeight - 60, dragStartRef.current.posY + dy)),
      });
    });
  }, []);

  const handleMouseUp = useCallback(() => {
    draggingRef.current = false;
    setDragging(false);
    if (rafRef.current) cancelAnimationFrame(rafRef.current);
    savePos(posRef.current);
    document.removeEventListener('mousemove', handleMouseMove);
    document.removeEventListener('mouseup', handleMouseUp);
  }, [handleMouseMove]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    draggingRef.current = true;
    setDragging(true);
    movedRef.current = false;
    dragStartRef.current = { x: e.clientX, y: e.clientY, posX: posRef.current.x, posY: posRef.current.y };
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  }, [handleMouseMove, handleMouseUp]);

  // ── Mobile touch handlers ──
  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (!draggingRef.current) return;
    e.preventDefault();
    const touch = e.touches[0];
    const dx = touch.clientX - dragStartRef.current.x;
    const dy = touch.clientY - dragStartRef.current.y;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) movedRef.current = true;
    if (rafRef.current) cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(() => {
      setPos({
        x: Math.max(0, Math.min(window.innerWidth - 60, dragStartRef.current.posX + dx)),
        y: Math.max(0, Math.min(window.innerHeight - 60, dragStartRef.current.posY + dy)),
      });
    });
  }, []);

  const handleTouchEnd = useCallback(() => {
    draggingRef.current = false;
    setDragging(false);
    if (rafRef.current) cancelAnimationFrame(rafRef.current);
    savePos(posRef.current);
    document.removeEventListener('touchmove', handleTouchMove);
    document.removeEventListener('touchend', handleTouchEnd);
  }, [handleTouchMove]);

  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    e.preventDefault();
    draggingRef.current = true;
    setDragging(true);
    movedRef.current = false;
    const touch = e.touches[0];
    dragStartRef.current = { x: touch.clientX, y: touch.clientY, posX: posRef.current.x, posY: posRef.current.y };
    document.addEventListener('touchmove', handleTouchMove);
    document.addEventListener('touchend', handleTouchEnd);
  }, [handleTouchMove, handleTouchEnd]);

  useEffect(() => () => {
    document.removeEventListener('mousemove', handleMouseMove);
    document.removeEventListener('mouseup', handleMouseUp);
    document.removeEventListener('touchmove', handleTouchMove);
    document.removeEventListener('touchend', handleTouchEnd);
    if (rafRef.current) cancelAnimationFrame(rafRef.current);
  }, [handleMouseMove, handleMouseUp, handleTouchMove, handleTouchEnd]);

  return (
    <>
      {/* ── 悬浮头像 ── */}
      <div
        className="mascot-float"
        style={{
          position: 'fixed', left: pos.x, top: pos.y, zIndex: 10000,
          cursor: dragging ? 'grabbing' : 'pointer', userSelect: 'none', touchAction: 'none',
        }}
        onMouseDown={handleMouseDown}
        onTouchStart={handleTouchStart}
        onClick={() => { if (!movedRef.current) setChatOpen(true); }}
      >
        <div className={`mascot-bubble ${bubble.visible ? 'visible' : ''}`}>
          <span style={{ fontSize: 12, lineHeight: 1.5, display: 'block' }}>{bubble.text}</span>
        </div>
        <div className="mascot-float-avatar">
          <img
            src={import.meta.env.BASE_URL + 'assets/xiaozhao.jpg'}
            alt="小招AI"
            width={40}
            height={40}
            style={{ borderRadius: '50%', objectFit: 'cover' }}
            onError={(e) => {
              // 加载失败时显示内联 SVG 兜底
              (e.target as HTMLImageElement).style.display = 'none';
              const p = (e.target as HTMLImageElement).parentElement;
              if (p) {
                p.innerHTML = '<svg width="40" height="40" viewBox="0 0 40 40"><circle cx="20" cy="20" r="19" fill="#fbbf24"/><circle cx="14" cy="16" r="3.5" fill="#333"/><circle cx="26" cy="16" r="3.5" fill="#333"/><circle cx="14" cy="16" r="1.2" fill="#fff"/><circle cx="26" cy="16" r="1.2" fill="#fff"/><path d="M13 26 Q20 32 27 26" stroke="#e84d4d" stroke-width="2" fill="none" stroke-linecap="round"/><circle cx="20" cy="8" r="4" fill="#ff6b9d" opacity="0.6"/></svg>';
              }
            }}
          />
        </div>
        <div className="mascot-float-glow" />
        <div className="mascot-float-tag">小招 AI</div>
      </div>

      <MascotChat open={chatOpen} onClose={() => setChatOpen(false)} />
    </>
  );
}
