import { useState, useEffect } from 'react';
import { getRandomQuote, getTimeBasedGreeting, getBeijingDate } from '@/components/Mascot/quotes';

export interface DailyGreeting {
  text: string;
  quote: string;
}

export function useDailyGreeting() {
  const [greeting, setGreeting] = useState<DailyGreeting | null>(null);
  const [showGreeting, setShowGreeting] = useState(false);

  /* ── 每日问候（先本地，再异步拉 AI 版替换） ── */
  useEffect(() => {
    let cancelled = false;
    // 基于 UTC+8 (北京时间) 生成问候
    const bj = getBeijingDate();
    const wk = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'][bj.weekday];
    const localText = `${bj.iso} ${bj.year}年${bj.month}月${bj.day}日 ${wk}，${getTimeBasedGreeting()}`;
    const localQuote = getRandomQuote().text;
    setGreeting({ text: localText, quote: localQuote });
    setShowGreeting(true);
    setTimeout(() => { if (!cancelled) setShowGreeting(false); }, 10500);

    // 异步拉取 AI 语录（只取语录，不用服务端的日期/问候，避免时间不同步）
    fetch('/api/mascot/daily')
      .then(r => r.ok ? r.json() : Promise.reject())
      .then((data: { quote?: string }) => {
        if (cancelled) return;
        if (data.quote) {
          setGreeting(prev => prev ? { ...prev, quote: data.quote! } : prev);
        }
      })
      .catch(() => {});

    return () => { cancelled = true; };
  }, []);

  return { greeting, showGreeting };
}
