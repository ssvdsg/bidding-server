import React, { useState, useCallback } from 'react';
import { CopyOutlined, CheckOutlined } from '@ant-design/icons';

interface NeonCopyProps {
  value: string;
  /** Optional label shown before the value */
  label?: string;
  /** Child content; if provided, wraps this instead of showing a default text layout */
  children?: React.ReactNode;
  style?: React.CSSProperties;
}

/**
 * Micro-copy HUD — hover 显示复制图标，点击复制 + "✦ 已复制" 反馈
 *
 * @example
 * <NeonCopy value="ZB-250530-001" label="项目编号" />
 * <NeonCopy value={budget.toLocaleString()} label="预算金额" />
 */
export default function NeonCopy({ value, label, children, style }: NeonCopyProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // fallback for older browsers
      const ta = document.createElement('textarea');
      ta.value = value;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }, [value]);

  return (
    <span
      onClick={handleCopy}
      title="点击复制"
      style={{
        cursor: 'pointer',
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        transition: 'color 0.2s',
        userSelect: 'none',
        ...style,
      }}
    >
      {label && <span style={{ color: 'rgba(0,0,0,0.45)', fontSize: 13 }}>{label}: </span>}
      {children || <span style={{ fontWeight: 500 }}>{value}</span>}
      {copied ? (
        <span style={{
          fontSize: 12,
          color: 'var(--anime-primary)',
          animation: 'fade-in-up 0.2s ease-out',
          marginLeft: 4,
        }}>
          <CheckOutlined style={{ marginRight: 2 }} />
          ✦ 已复制
        </span>
      ) : (
        <CopyOutlined
          style={{
            fontSize: 12,
            opacity: 0,
            transition: 'opacity 0.2s',
            marginLeft: 4,
            color: 'var(--anime-purple)',
          }}
          className="neon-copy-icon"
        />
      )}
    </span>
  );
}

/* Inject hover show for the copy icon */
const styleSheet = document.createElement('style');
styleSheet.textContent = `
  .neon-copy-icon { opacity: 0 !important; }
  *:hover > .neon-copy-icon,
  span:hover .neon-copy-icon { opacity: 1 !important; }
`;
document.head.appendChild(styleSheet);
