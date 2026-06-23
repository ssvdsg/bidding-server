// 中转 AI 的 key_model → 实际后端模型友好名映射
// 数据来源：中转 AI 服务 /docs 接口
const RELAY_MODEL_PRETTY_NAME: Record<string, string> = {
  TEXT_DEEPSEEK_V4: 'DeepSeek-V4',
  TEXT_A14: 'GLM-5',
  TEXT_A22: 'Qwen3.5-Plus',
  TEXT_A13: 'Qwen3:30B',
  TEXT_A8: 'DeepSeek-V3.2',
  lingxi: 'WPS Lingxi',
};

// 把后端记录的形如 "Relay AI (TEXT_DEEPSEEK_V4)" / "Relay AI File (TEXT_A8)" 转成 "Relay AI · DeepSeek-V4"
export function prettyModelLabel(model?: string | null): string {
  if (!model) return '';
  const raw = String(model).trim();
  if (!raw) return '';

  // 命中括号内的 key_model
  const match = raw.match(/\(\s*([A-Za-z0-9_\-:.]+)\s*\)/);
  if (match) {
    const key = match[1];
    const pretty = RELAY_MODEL_PRETTY_NAME[key];
    if (pretty) {
      // 保留前缀（Relay AI / Relay AI File 等链路标签），用 · 串接友好名
      const prefix = raw.slice(0, match.index).replace(/\s+$/, '').trim();
      return prefix ? `${prefix} · ${pretty}` : pretty;
    }
  }

  // 整串就是裸的 key_model
  if (RELAY_MODEL_PRETTY_NAME[raw]) return RELAY_MODEL_PRETTY_NAME[raw];

  // 不在表里就回退到去掉括号
  return raw.replace(/\s*\([^)]*\)\s*/g, '').trim();
}
