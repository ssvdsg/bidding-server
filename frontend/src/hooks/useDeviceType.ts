import { useMemo } from 'react';
import { useResponsive } from 'ahooks';

export type DeviceType = 'mobile' | 'desktop';

export interface DeviceInfo {
  /** True when viewport < 992px (below Ant Design's `lg` breakpoint) */
  isMobile: boolean;
  /** True when viewport >= 992px */
  isDesktop: boolean;
  /** Convenience discriminator: `'mobile'` | `'desktop'` */
  deviceType: DeviceType;
}

/**
 * Reactive device-type hook powered by `ahooks`'s `useResponsive`.
 *
 * Uses Ant Design's standard breakpoints internally:
 *  - `xs`  < 576px
 *  - `sm`  ≥ 576px
 *  - `md`  ≥ 768px
 *  - `lg`  ≥ 992px   ← **desktop boundary** (all breakpoints ≥ this are desktop)
 *  - `xl`  ≥ 1200px
 *  - `xxl` ≥ 1600px
 *
 * @example
 * ```tsx
 * function Dashboard() {
 *   const { isMobile, deviceType } = useDeviceType();
 *
 *   if (isMobile) {
 *     return <DashboardMobileSummary />;      // lightweight list + download link
 *   }
 *
 *   return (
 *     <>
 *       <DashboardChart />                    // <-- uses Recharts (Desktop only)
 *       <PDFPreview isMobile={false} ... />   // <-- uses pdfjs-dist (Desktop only)
 *     </>
 *   );
 * }
 * ```
 *
 * @example
 * ```tsx
 * // Conditional rendering at the route level
 * function PDFViewerPage() {
 *   const { isMobile } = useDeviceType();
 *   return isMobile
 *     ? <NativeDownloadLink />   // just a download button, no pdfjs-dist import
 *     : <PDFPreview ... />;      // full pdfjs-dist canvas renderer
 * }
 * ```
 */
export function useDeviceType(): DeviceInfo {
  const responsive = useResponsive();

  return useMemo(() => {
    // `useResponsive` returns boolean flags keyed by breakpoint name.
    // We treat anything below `lg` as mobile — matches Ant Design's
    // `Grid.useBreakpoint()` convention used in Layout.tsx.
    const isMobile = !responsive.lg;

    return {
      isMobile,
      isDesktop: responsive.lg,
      deviceType: isMobile ? 'mobile' : 'desktop',
    };
  }, [responsive]);
}
