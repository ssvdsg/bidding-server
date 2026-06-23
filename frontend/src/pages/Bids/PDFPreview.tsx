import React, { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Modal, Space, Spin, Tabs, Typography } from 'antd';
import { DownloadOutlined, FullscreenOutlined } from '@ant-design/icons';
import { getDocument, GlobalWorkerOptions } from 'pdfjs-dist';
// 用 vite 打包本地 worker，避免依赖 unpkg CDN（国内网络/CSP/跨域常导致空白或加载失败）
import pdfWorkerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url';

GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

function ExternalLinkCard({ url, label }: { url: string; label?: string }) {
  return (
    <div
      style={{
        background: '#f8f8f8',
        borderRadius: 8,
        padding: 24,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 16,
        textAlign: 'center',
      }}
    >
      <Typography.Title level={5} style={{ margin: 0 }}>
        该文件托管在 {label || '外部平台'}
      </Typography.Title>
      <Typography.Text type="secondary">
        外部分享链接无法在本站内嵌渲染，请点击下方按钮跳转查看。
      </Typography.Text>
      <Button type="primary" href={url} target="_blank" rel="noreferrer">
        在 {label || '外部平台'} 打开
      </Button>
      <Typography.Link href={url} target="_blank" rel="noreferrer" copyable style={{ wordBreak: 'break-all', fontSize: 12 }}>
        {url}
      </Typography.Link>
    </div>
  );
}

type PDFLink = {
  key: string;
  label: string;
  isPdf?: boolean;
  /** 外部分享链接（如 WPS 灵析），存在时直接打开外链，不走本地代理也不渲染 */
  externalURL?: string;
  externalLabel?: string;
};

type PDFPreviewProps = {
  bidId: string;
  links: PDFLink[];
  htmlContent?: string;
  height?: number;
  isMobile?: boolean;
};

function hasText(value?: string | null): value is string {
  return typeof value === 'string' && value.trim() !== '';
}

function buildProxyURL(bidId: string, fileKey: string) {
  return `/api/pdfProxy?id=${encodeURIComponent(bidId)}&file=${encodeURIComponent(fileKey)}`;
}

function HTMLFrame({ content }: { content: string; height?: number }) {
  return (
    <iframe
      srcDoc={content}
      title="公告在线预览"
      style={{ width: '100%', minHeight: 600, border: 0, borderRadius: 8, background: '#fff' }}
      sandbox="allow-same-origin"
    />
  );
}

function NonPDFFallback({ url }: { url: string; height?: number }) {
  // 非 PDF 文件（多数情况下是 HTML/招标公告原始网页），优先用 iframe 在线渲染
  // 浏览器拿不到 PDF/HTML 之外的格式（doc/zip）时会自己弹下载，依然有出口
  return (
    <div style={{ background: '#f8f8f8', borderRadius: 8, padding: 8, display: 'flex', flexDirection: 'column', gap: 8 }}>
      <iframe
        title="文件在线预览"
        src={url}
        style={{
          width: '100%',
          minHeight: 600,
          border: 0,
          borderRadius: 8,
          background: '#fff',
        }}
        sandbox="allow-same-origin allow-popups allow-forms"
      />
      <div style={{ textAlign: 'right' }}>
        <Button size="small" type="link" icon={<DownloadOutlined />} href={url} target="_blank">
          无法预览？点击下载
        </Button>
      </div>
    </div>
  );
}

function PDFCanvasPreview({ bidId, fileKey }: { bidId: string; fileKey: string; height?: number }) {
  const wrapRef = useRef<HTMLDivElement | null>(null);   // 外层滚动容器（用来量宽度）
  const canvasHostRef = useRef<HTMLDivElement | null>(null); // 真正放 canvas 的内层
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>('');
  const [empty, setEmpty] = useState(false);
  const [containerWidth, setContainerWidth] = useState(0);

  useEffect(() => {
    if (!wrapRef.current) return;
    const node = wrapRef.current;
    // 首次测量
    const inner = node.clientWidth - 16;
    if (inner > 0) setContainerWidth(inner);
    // ResizeObserver 仅当宽度变化超过 20px 才更新，避免 PDF 渲染过程中
    // 因 canvas 插入导致容器微小幅宽变化触发无限重加载循环
    const observer = new ResizeObserver(() => {
      const newInner = node.clientWidth - 16;
      if (newInner <= 0) return;
      setContainerWidth(prev => Math.abs(prev - newInner) > 20 ? newInner : prev);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!containerWidth) return;
    let cancelled = false;
    let loadingTask: ReturnType<typeof getDocument> | null = null;

    const renderPDF = async () => {
      setLoading(true);
      setError('');
      setEmpty(false);
      if (canvasHostRef.current) {
        canvasHostRef.current.innerHTML = '';
      }

      const url = buildProxyURL(bidId, fileKey);
      try {
        loadingTask = getDocument({ url, withCredentials: false });
        const pdf = await loadingTask.promise;

        if (pdf.numPages === 0) {
          if (!cancelled) setEmpty(true);
          return;
        }

        const dpr = Math.min(window.devicePixelRatio || 1, 2);

        for (let pageNumber = 1; pageNumber <= pdf.numPages; pageNumber += 1) {
          if (cancelled) return;
          const page = await pdf.getPage(pageNumber);
          const baseViewport = page.getViewport({ scale: 1 });
          const cssScale = containerWidth / baseViewport.width;
          const renderScale = cssScale * dpr;
          const renderViewport = page.getViewport({ scale: renderScale });

          const canvas = document.createElement('canvas');
          const context = canvas.getContext('2d');
          if (!context) continue;

          canvas.width = renderViewport.width;
          canvas.height = renderViewport.height;
          canvas.style.width = `${containerWidth}px`;
          canvas.style.height = `${baseViewport.height * cssScale}px`;
          canvas.style.maxWidth = '100%';
          canvas.style.display = 'block';
          canvas.style.marginBottom = '12px';
          canvas.style.borderRadius = '8px';
          canvas.style.background = '#fff';
          canvas.style.boxShadow = '0 1px 4px rgba(0,0,0,0.06)';

          await page.render({
            canvas,
            canvasContext: context,
            viewport: renderViewport,
          }).promise;

          if (!cancelled && canvasHostRef.current) {
            canvasHostRef.current.appendChild(canvas);
          }
        }
      } catch (err) {
        if (cancelled) return;
        const raw = err instanceof Error ? err.message : String(err);
        const friendly = /InvalidPDF|Invalid PDF/i.test(raw)
          ? '文件不是有效的 PDF，可能是 Word/扫描件或附件已失效'
          : raw;
        setError(friendly);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    renderPDF();

    return () => {
      cancelled = true;
      try { loadingTask?.destroy(); } catch { /* ignore */ }
    };
  }, [bidId, fileKey, containerWidth]);

  const downloadURL = buildProxyURL(bidId, fileKey);

  return (
    <div
      ref={wrapRef}
      style={{
        width: '100%',
        overflowX: 'hidden',
        background: '#f8f8f8',
        padding: 8,
        borderRadius: 8,
        boxSizing: 'border-box',
      }}
    >
      {loading && (
        <div style={{ display: 'flex', justifyContent: 'center', padding: '48px 0' }}>
          <Spin tip="正在加载 PDF..." />
        </div>
      )}
      {!loading && error && (
        <Alert
          type="warning"
          message="PDF 在线预览失败"
          description={
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <span>{error}</span>
              <span>
                <Button size="small" type="link" icon={<DownloadOutlined />} href={downloadURL} target="_blank" style={{ paddingLeft: 0 }}>
                  直接下载查看
                </Button>
              </span>
            </div>
          }
          showIcon
        />
      )}
      {!loading && !error && empty && (
        <Typography.Text type="secondary">该 PDF 没有可显示的页面</Typography.Text>
      )}
      <div ref={canvasHostRef} />
    </div>
  );
}

export default function PDFPreview({ bidId, links, htmlContent, height = 720, isMobile }: PDFPreviewProps) {
  const [modalOpen, setModalOpen] = useState(false);
  const [darkMode, setDarkMode] = useState(() => {
    try { return localStorage.getItem('pdf_dark_mode') === '1'; } catch { return false; }
  });
  const toggleDarkMode = () => {
    const next = !darkMode;
    setDarkMode(next);
    try { localStorage.setItem('pdf_dark_mode', next ? '1' : '0'); } catch {}
  };
  const hasHTMLPreview = hasText(htmlContent);
  if (!links.length && !hasHTMLPreview) return null;

  const hasAnyPDF = links.some((item) => item.isPdf !== false && !item.externalURL);

  const fileTabs = links.map((item) => {
    let children: React.ReactNode;
    if (item.externalURL) {
      children = <ExternalLinkCard url={item.externalURL} label={item.externalLabel} />;
    } else if (item.isPdf === false) {
      children = <NonPDFFallback url={buildProxyURL(bidId, item.key)} />;
    } else {
      children = <PDFCanvasPreview bidId={bidId} fileKey={item.key} />;
    }
    return {
      key: item.key,
      label: item.externalURL ? `${item.label}（${item.externalLabel || '外链'}）` : item.label,
      children,
    };
  });

  const htmlTab = hasHTMLPreview
    ? {
        key: 'html-preview',
        label: '公告在线预览',
        children: <HTMLFrame content={htmlContent!} />,
      }
    : null;

  const previewItems = htmlTab && !hasAnyPDF
    ? [htmlTab, ...fileTabs]
    : [...fileTabs, ...(htmlTab ? [htmlTab] : [])];

  // 直接展示完整内容（无高度限制），点击可打开全屏弹窗
  const modalWidth = isMobile ? '100vw' : '90vw';
  const modalHeight = isMobile ? '100dvh' : '90vh';

  const renderPreview = (items: typeof previewItems) => items.length === 1 ? items[0].children : (
    <Tabs defaultActiveKey={items[0]?.key} items={items} />
  );

  return (
    <>
      {/* 直接渲染完整内容，点击容器打开全屏弹窗 */}
      <div onClick={() => setModalOpen(true)} style={{ cursor: 'pointer' }}>
        <Card size="small" type="inner" title={
          <Space>
            <span>在线预览</span>
            <Button size="small" type="link" icon={<FullscreenOutlined />} onClick={(e) => { e.stopPropagation(); setModalOpen(true); }}>
              全屏
            </Button>
          </Space>
        }
        extra={
          <Button
            size="small"
            type={darkMode ? 'primary' : 'default'}
            onClick={(e) => { e.stopPropagation(); toggleDarkMode(); }}
            style={{ fontSize: 11 }}
          >
            {darkMode ? '🌙 全息' : '☀️ 正常'}
          </Button>
        }
        bodyStyle={{ padding: 8, ...(darkMode ? { filter: 'invert(0.9) hue-rotate(180deg)', background: '#111' } : {}) }}>
          {renderPreview(previewItems)}
        </Card>
      </div>

      <Modal
        title="文件预览"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={null}
        width={modalWidth}
        styles={{ body: { maxHeight: modalHeight, overflow: 'auto', padding: 12 } }}
        destroyOnClose
      >
        {renderPreview(previewItems)}
      </Modal>
    </>
  );
}
