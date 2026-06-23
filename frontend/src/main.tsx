import React, { useEffect, useState } from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import 'dayjs/locale/zh-cn';
import App from './App';
import './index.css';

// 监听系统主题变化
const useSystemTheme = () => {
  const [isDark, setIsDark] = useState(
    window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
  );

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = (e: MediaQueryListEvent) => setIsDark(e.matches);
    mediaQuery.addEventListener('change', handler);
    return () => mediaQuery.removeEventListener('change', handler);
  }, []);

  return isDark;
};

const Root = () => {
  const isDark = useSystemTheme();

  return (
    <React.StrictMode>
      <ConfigProvider
        locale={zhCN}
        theme={{
          algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
          token: {
            // 商务二次元风格 - 深蓝商务 + 暖金点缀 + 柔和粉彩
            colorPrimary: isDark ? '#818cf8' : '#1a1a2e',
            colorInfo: isDark ? '#818cf8' : '#16213e',
            colorSuccess: '#10b981',
            colorWarning: '#f59e0b',
            colorError: '#ef4444',
            colorLink: isDark ? '#a78bfa' : '#6d28d9',
            borderRadius: 8,
            borderRadiusLG: 12,
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Hiragino Sans", "Noto Sans SC", "Helvetica Neue", Arial, sans-serif',
            colorBgContainer: isDark ? '#0f0f1a' : '#ffffff',
            colorBgElevated: isDark ? '#1a1a2e' : '#ffffff',
            colorBorder: isDark ? '#2a2a3e' : '#e4dcd0',
            colorBorderSecondary: isDark ? '#2a2a3e' : '#ede4d8',
            boxShadow: '0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04)',
            boxShadowSecondary: '0 4px 12px rgba(0,0,0,0.08)',
          },
          components: {
            Layout: {
              bodyBg: isDark ? '#0a0a14' : '#f5f0eb',
              headerBg: isDark ? '#0f0f1a' : '#ffffff',
              siderBg: isDark ? '#0f0f1a' : '#ffffff',
            },
            Menu: {
              itemBg: 'transparent',
              itemActiveBg: isDark ? '#1f1f3a' : '#f0e8de',
              itemSelectedBg: isDark ? '#1f1f3a' : '#f0e8de',
              itemSelectedColor: isDark ? '#a78bfa' : '#1a1a2e',
              itemHoverBg: isDark ? '#1a1a30' : '#f5efe8',
              itemColor: isDark ? '#a0a0b8' : '#555',
            },
            Card: {
              paddingLG: 20,
              borderRadiusLG: 12,
            },
            Table: {
              headerBg: isDark ? '#14142a' : '#faf5f0',
              borderColor: isDark ? '#2a2a3e' : '#ede4d8',
            },
            Button: {
              borderRadius: 6,
              primaryShadow: '0 2px 6px rgba(106, 13, 173, 0.2)',
            },
            Input: {
              borderRadius: 6,
            },
            Progress: {
              borderRadius: 4,
            },
          }
        }}
      >
        <App />
      </ConfigProvider>
    </React.StrictMode>
  );
};

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(<Root />);
