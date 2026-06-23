import React, { useMemo, useState } from 'react';
import { Layout, Menu, Button, Drawer, Grid, theme } from 'antd';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  DashboardOutlined,
  UnorderedListOutlined,
  StopOutlined,
  EyeOutlined,
  BankOutlined,
  SettingOutlined,
  RobotOutlined,
  MenuOutlined,
} from '@ant-design/icons';
import Mascot from '../Mascot';
import { useDailyGreeting } from '@/hooks/useDailyGreeting';

const { Header, Sider, Content } = Layout;
const { useBreakpoint } = Grid;

/* 全局樱花装饰 */
function SakuraPetals() {
  return (
    <>
      <div className="sakura-petal" />
      <div className="sakura-petal" />
      <div className="sakura-petal" />
      <div className="sakura-petal" />
      <div className="sakura-petal" />
      <div className="sakura-petal" />
      <div className="sakura-petal" />
      <div className="sakura-petal" />
    </>
  );
}

export default function LayoutComponent() {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();
  const screens = useBreakpoint();
  const isMobile = !screens.lg;
  const { greeting, showGreeting } = useDailyGreeting();

  const menuItems = useMemo(() => [
    { key: '/', icon: <DashboardOutlined />, label: '数据大盘' },
    { key: '/bids', icon: <UnorderedListOutlined />, label: '项目大厅' },
    { key: '/bids/excluded', icon: <StopOutlined />, label: '排除库' },
    { key: '/tracked', icon: <EyeOutlined />, label: '中标追踪' },
    { key: '/companies', icon: <BankOutlined />, label: '企业库' },
    { key: '/tasks', icon: <RobotOutlined />, label: 'AI 自动任务' },
    { key: '/settings', icon: <SettingOutlined />, label: '系统设置' },
  ], []);

  // 终端风格路径
  const terminalPath = (() => {
    const item = menuItems.find(item => location.pathname.startsWith(item.key) && item.key !== '/');
    const base = item ? item.key : location.pathname;
    return `root@ccs:~# ${base.replace(/^\//, '').replace(/\//g, '/')}`;
  })();

  const menuNode = (
    <Menu
      mode="inline"
      selectedKeys={[location.pathname]}
      items={menuItems}
      onClick={({ key }) => {
        navigate(key);
        setMobileMenuOpen(false);
      }}
      style={{ borderRight: 0, marginTop: isMobile ? 0 : 16 }}
    />
  );

  return (
    <Layout style={{ minHeight: '100vh' }}>
      {/* 全局樱花装饰 */}
      <SakuraPetals />

      {!isMobile && (
        <Sider 
          collapsible 
          collapsed={collapsed} 
          onCollapse={(value) => setCollapsed(value)}
          theme="light"
          style={{ borderRight: `1px solid ${token.colorBorder}` }}
        >
          {/* Logo 区域 — 商务二次元 */}
          <div style={{ 
            height: 64, 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: collapsed ? 'center' : 'flex-start',
            padding: collapsed ? 0 : '0 20px',
            gap: 8,
            fontWeight: 600,
            fontSize: collapsed ? 18 : 20,
            background: 'linear-gradient(135deg, #1a1a2e 0%, #16213e 100%)',
            color: '#fff',
            letterSpacing: '0.5px',
            borderBottom: 'none',
            overflow: 'hidden',
          }}>
            <span style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: collapsed ? 28 : 32,
              height: collapsed ? 28 : 32,
              borderRadius: 8,
              background: 'linear-gradient(135deg, #fbbf24, #f59e0b)',
              color: '#1a1a2e',
              fontSize: collapsed ? 14 : 16,
              fontWeight: 700,
              flexShrink: 0,
            }}>
              B
            </span>
            {!collapsed && (
              <>
                <span style={{ color: '#fff', fontFamily: "'ZCOOL KuaiLe', 'PingFang SC', sans-serif", letterSpacing: '1px' }}>招标AI</span>
                <span style={{ fontSize: 10, opacity: 0.7, color: '#fbbf24', marginLeft: 2 }}>✦</span>
              </>
            )}
          </div>

          {/* 侧边栏底部二次元小标签 */}
          <div style={{
            position: 'absolute',
            bottom: 16,
            left: 0,
            right: 0,
            textAlign: 'center',
            fontSize: 10,
            color: collapsed ? 'transparent' : 'rgba(0,0,0,0.2)',
            transition: 'color 0.2s',
            letterSpacing: 2,
            userSelect: 'none',
          }}>
            {!collapsed && '✦ 小招AI · 为您服务 ✦'}
          </div>

          {menuNode}
        </Sider>
      )}
      <Layout>
        {/* Header 标题栏 — 装饰升级 */}
        <Header style={{ 
          padding: isMobile ? '0 16px' : '0 24px', 
          background: token.colorBgContainer,
          borderBottom: `1px solid ${token.colorBorder}`,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          fontSize: 18,
          fontWeight: 500,
          position: 'relative',
          overflow: 'visible',
          minHeight: 64,
        }}>
          {/* header 底部金色装饰线 */}
          <div style={{
            position: 'absolute',
            bottom: 0,
            left: 0,
            right: 0,
            height: 2,
            background: 'linear-gradient(90deg, transparent, #fbbf24 20%, #ff6b9d 50%, #6d28d9 80%, transparent)',
            opacity: 0.4,
          }} />
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className="deco-star" style={{ fontSize: 14 }}>✦</span>
            <code style={{
              fontFamily: "'JetBrains Mono', 'Fira Code', 'Consolas', monospace",
              fontSize: 13,
              background: 'rgba(0,240,255,0.06)',
              padding: '2px 10px',
              borderRadius: 6,
              color: 'var(--anime-primary)',
              letterSpacing: '0.5px',
            }}>
              {terminalPath}
            </code>
            {greeting && showGreeting && (
              <span className="daily-greeting-card animated-card" style={{ marginLeft: 16 }}>
                <span className="daily-greeting-row">
                  <span className="daily-greeting-star">✦</span>{' '}
                  <span className="daily-greeting-date">{greeting.text}</span>
                </span>
                <span className="daily-greeting-row">
                  <span className="daily-greeting-quote">{greeting.quote}</span>
                </span>
              </span>
            )}
          </span>
          {isMobile && (
            <Button
              type="text"
              icon={<MenuOutlined />}
              onClick={() => setMobileMenuOpen(true)}
            />
          )}
        </Header>
        {/* Content 去掉顶部 padding，由各页面自己控制 */}
        <Content style={{ 
          padding: isMobile ? '0 12px 24px' : '0 24px 24px', 
          overflow: 'auto', 
          background: token.colorBgLayout,
        }}>
          <Outlet />
        </Content>
      </Layout>
      <Drawer
        title={<span><span className="deco-star" style={{ marginRight: 6 }}>✦</span> 菜单</span>}
        placement="left"
        onClose={() => setMobileMenuOpen(false)}
        open={isMobile && mobileMenuOpen}
        styles={{ body: { padding: 0 } }}
        width={260}
      >
        {menuNode}
      </Drawer>

      {/* 全局悬浮桌宠 */}
      <Mascot />
    </Layout>
  );
}
