/* ==========================================================
   小招AI 语录库 — 商务二次元风格
   ========================================================== */

export type QuoteCategory = 'greeting' | 'bid_tip' | 'encourage' | 'system' | 'random';

export interface Quote {
  text: string;
  category: QuoteCategory;
}

/** 获取 UTC+8 (北京时间) 当前小时的整数 [0-23] */
export function getBeijingHour(): number {
  const now = new Date();
  const utc = now.getTime() + now.getTimezoneOffset() * 60000;
  return new Date(utc + 8 * 3600000).getHours();
}

/** 获取 UTC+8 (北京时间) 的日期信息 */
export function getBeijingDate(): { year: number; month: number; day: number; weekday: number; iso: string } {
  const now = new Date();
  const utc = now.getTime() + now.getTimezoneOffset() * 60000;
  const bj = new Date(utc + 8 * 3600000);
  return {
    year: bj.getFullYear(),
    month: bj.getMonth() + 1,
    day: bj.getDate(),
    weekday: bj.getDay(),
    iso: `${bj.getFullYear()}-${String(bj.getMonth()+1).padStart(2,'0')}-${String(bj.getDate()).padStart(2,'0')}`,
  };
}

/* 预设语录 */
const PRESET_QUOTES: Quote[] = [
  // ── 问候 ──
  { text: '早上好！今天也要元气满满地投标哦 ✦', category: 'greeting' },
  { text: '午安～ 记得按时吃饭，下午才有精神看标书！', category: 'greeting' },
  { text: '晚上好！今天辛苦了，喝杯茶放松一下吧 🍵', category: 'greeting' },
  { text: '夜深了，早点休息，明天再战！🌙', category: 'greeting' },

  // ── 投标提醒 ──
  { text: '今天有新的招标项目上线，快去看看吧 🔍', category: 'bid_tip' },
  { text: '别忘了检查一下排除库，可能有需要恢复的项目哦', category: 'bid_tip' },
  { text: '中标追踪中还有项目在监控中，一切正常 ✨', category: 'bid_tip' },
  { text: '推荐投标的项目看了吗？别错过好机会！', category: 'bid_tip' },
  { text: 'AI 分析引擎正在努力工作，已扫描今日最新招标 🚀', category: 'bid_tip' },

  // ── 鼓励 ──
  { text: '投标就像种地，今天的耕耘是明天的收获 🌱', category: 'encourage' },
  { text: '每一次投标都是一次成长，加油！💪', category: 'encourage' },
  { text: '机会总是留给有准备的人，而你已经准备好了 ✦', category: 'encourage' },
  { text: '不积跬步无以至千里，每一份标书都很重要 📚', category: 'encourage' },
  { text: '保持专注，保持耐心，成功就在不远处 🎯', category: 'encourage' },
  { text: '小招相信你一定可以的！(｡•̀ᴗ-)✧', category: 'encourage' },
  { text: '命运不会辜负努力的人，今天的你也在闪闪发光 ✨', category: 'encourage' },

  // ── 系统状态 ──
  { text: '小招一直在后台默默守护着你的招标信息哦 💝', category: 'system' },
  { text: '系统运行正常，AI 分析引擎状态良好 ✅', category: 'system' },
  { text: '有任何问题随时点我聊～ 小招随时在线！', category: 'system' },
  { text: '数据已同步，一切尽在掌握 📊', category: 'system' },
  { text: '今天也是努力打工的一天呢 (๑•̀ㅂ•́)و✧', category: 'system' },
  { text: '你专注工作，我专注监控，合作愉快 🤝', category: 'system' },
];

/* 按类别获取随机语录（自动过滤不合时宜的） */
export function getRandomQuote(category?: QuoteCategory): Quote {
  const h = getBeijingHour();
  const isDaytime = h >= 6 && h < 18;
  let pool = category
    ? PRESET_QUOTES.filter(q => q.category === category)
    : PRESET_QUOTES;
  // 白天过滤掉带"夜深""晚安""晚上好"的语录
  if (isDaytime) {
    pool = pool.filter(q => !/夜深|晚安|晚上好|明天再战/i.test(q.text));
  }
  // 如果过滤后为空则回退到原池
  if (pool.length === 0) {
    pool = category
      ? PRESET_QUOTES.filter(q => q.category === category)
      : PRESET_QUOTES;
  }
  return pool[Math.floor(Math.random() * pool.length)];
}

/* ── 业务数据感知语录 — 看板娘灵动化，数据驱动台词 ── */
export interface MascotStats {
  total?: number;
  today?: number;
  suitable?: number;
  highPriority?: number;
  notSuitable?: number;
  pendingAI?: number;
  trackedCompanies?: number;
}

export type MascotEmotion = 'idle' | 'panicked' | 'happy' | 'working' | 'concerned' | 'excited';

export interface MascotReaction {
  emotion: MascotEmotion;
  text: string;
}

export function getMascotReaction(stats: MascotStats): MascotReaction {
  if (!stats || stats.total === undefined) {
    return { emotion: 'idle', text: '正在连接智脑核心... (｡+･`ω･´)' };
  }

  if (stats.pendingAI !== undefined && stats.pendingAI > 15) {
    return {
      emotion: 'panicked',
      text: `主上！AI 待分析队列已经堆积到 ${stats.pendingAI} 个了！机体要过载啦～ ヽ(≧Д≦)ノ`,
    };
  }

  if (stats.today !== undefined && stats.today > 50) {
    return {
      emotion: 'excited',
      text: `🎉 哇噢！今日新增了 ${stats.today} 条高价值标讯！我们离统治行业又近了一步！`,
    };
  }

  if (stats.today === 0) {
    return {
      emotion: 'concerned',
      text: '今天还没有新的标讯输入呢，要不要去喝杯咖啡？☕',
    };
  }

  if (stats.highPriority !== undefined && stats.highPriority > 10) {
    return {
      emotion: 'excited',
      text: `高优先项目 ${stats.highPriority} 个！(๑•̀ㅂ•́)و✧ 今天要大干一场！`,
    };
  }

  if (stats.suitable !== undefined && stats.suitable > 30) {
    return {
      emotion: 'happy',
      text: `推荐投标 ${stats.suitable} 个项目！小招正在火力全开中 🔥`,
    };
  }

  return {
    emotion: 'working',
    text: `正在默默守护主上的企业库。目前有 ${stats.total ?? '?'} 条标讯处于监控中哦 (｀･ω･´)ゞ`,
  };
}

/* 按时间段获取问候语（基于 UTC+8） */
export function getTimeBasedGreeting(): string {
  const h = getBeijingHour();
  if (h < 6) return '夜深了，小招陪你一起熬夜 🌙';
  if (h < 9) return '早安！今天也要加油哦 ☀️';
  if (h < 12) return '上午好，投标的黄金时间到啦 ✦';
  if (h < 14) return '中午好～ 小招提醒你按时吃饭 🍱';
  if (h < 18) return '下午好！打起精神来，冲刺吧 ⚡';
  return '晚上好，今天辛苦啦，小招为你点赞 ✨';
}

/* ==========================================================
   AI 个性化语录（通过 ctyun 通道）
   调用 /api/chat/stream 生成定制化提醒
   ========================================================== */

export async function fetchAIQuote(): Promise<string> {
  try {
    const password = sessionStorage.getItem('access_password') || '';
    const resp = await fetch('/api/chat/stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Access-Password': password,
      },
      body: JSON.stringify({
        prompt: `你现在是一个可爱的招标助理「小招AI」，请用一句话给我一个温暖的鼓励或投标小提醒，
风格要既专业又可爱，可以包含 emoji，不超过 30 个字。不要加引号。`,
        model: 'TEXT_DEEPSEEK_V4',
        web_search: false,
        enable_thinking: false,
      }),
    });
    if (!resp.ok) throw new Error('API error');

    const reader = resp.body?.getReader();
    if (!reader) throw new Error('No reader');

    const decoder = new TextDecoder();
    let buffer = '';
    let result = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || !trimmed.startsWith('data:')) continue;
        const payload = trimmed.slice(5).trim();
        if (payload === '[DONE]') continue;
        try {
          const evt = JSON.parse(payload);
          for (const choice of evt.choices || []) {
            const delta = choice.delta?.content || '';
            result += delta;
          }
        } catch { /* skip parse errors */ }
      }
    }
    return result.trim() || getRandomQuote().text;
  } catch {
    return getRandomQuote().text;
  }
}
