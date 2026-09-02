import { StrictMode, Suspense, useEffect, type CSSProperties, type ReactElement, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Navigate, Route, Routes, matchPath, useLocation, useParams } from 'react-router-dom';
import '@fontsource/chakra-petch/latin-400.css';
import '@fontsource/chakra-petch/latin-500.css';
import '@fontsource/chakra-petch/latin-600.css';
import '@fontsource/chakra-petch/latin-700.css';
import '@fontsource/share-tech-mono/latin-400.css';
import '@/app/globals.css';
import { Toaster } from '@/components/ui/sonner';
import { ServiceWorkerCleanup } from '@/components/shell/service-worker-cleanup';
import { ShellCanvas } from '@/components/shell/shell-canvas';
import { StudioAmbient } from '@/components/shell/studio-ambient';
import { WindowActivityPolicy } from '@/components/shell/window-activity-policy';
import { DesktopPerformance } from '@/components/shell/desktop-performance';
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar';
import { AppSidebar } from '@/components/shell/app-sidebar';
import { CommandStrip } from '@/components/shell/command-strip';
import { ShellActivityMonitor } from '@/components/shell/shell-activity-monitor';
import { TelemetryNotice } from '@/components/shell/telemetry-notice';
import { SIDEBAR_COOKIE_NAME } from '@/components/shell/shell-cookies';
import { TacticalWorkspace } from '@/components/tactical/tactical-workspace';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';
import { EditorWorkspace } from '@/components/editor/editor-workspace';
import OnboardingPage from '@/app/(app)/onboarding/page';
import MatchesPage from '@/app/(app)/matches/page';
import FindHighlightsPage from '@/app/(app)/matches/[id]/page';
import UploadPage from '@/app/upload/page';
import FullDemoIndexPage from '@/app/(app)/full-demo/page';
import FullDemoJobPage from '@/app/(app)/full-demo/[id]/page';
import TacticalIndexPage from '@/app/(app)/tactical/page';
import CheatersPage from '@/app/(app)/cheaters/page';
import PlayersPage from '@/app/(app)/players/page';
import StreamsPage from '@/app/(app)/streams/page';
import EditorPage from '@/app/(app)/editor/page';
import VideosPage from '@/app/(app)/videos/page';
import FeedPage from '@/app/(app)/feed/page';
import SettingsPage from '@/app/(app)/settings/page';
import SeriesPage from '@/app/(app)/series/[id]/page';
import BootstrapPage from '@/src/pages/bootstrap-page';
import NotFoundPage from '@/src/pages/not-found-page';

const SHELL_VARS: CSSProperties & { '--sidebar-width': string } = { '--sidebar-width': '240px' };
const ROUTE_TITLES = [
  ['/onboarding', 'Inicio'], ['/matches/:id', 'Partida'], ['/matches', 'Partidas'],
  ['/upload', 'Subir demo'], ['/full-demo/:id', 'Demo completa'], ['/full-demo', 'Demo completa'],
  ['/tactical/:jobId', 'Táctica'], ['/tactical', 'Táctica'], ['/cheaters', 'CheaterDetect'],
  ['/players', 'Jugadores'], ['/streams', 'Clips de streams'], ['/editor/:id', 'Editor'],
  ['/editor', 'Editor'], ['/videos', 'Biblioteca'], ['/feed', 'Feed'], ['/settings', 'Ajustes'],
  ['/series/:id', 'Serie'], ['/bootstrap', 'Iniciando sesión'],
] as const;

function DocumentTitle(): null {
  const { pathname } = useLocation();
  useEffect(() => {
    const route = ROUTE_TITLES.find(([pattern]) => matchPath({ path: pattern, end: true }, pathname));
    document.title = `${route?.[1] ?? 'Página no encontrada'} · ClipHub`;
  }, [pathname]);
  return null;
}

function cookieValue(name: string): string | undefined {
  for (const entry of document.cookie.split(';')) {
    const [key, ...parts] = entry.trim().split('=');
    if (key === name) return parts.join('=');
  }
  return undefined;
}

function AppShell({ children }: { children: ReactNode }): ReactElement {
  const sidebarOpen = cookieValue(SIDEBAR_COOKIE_NAME) !== 'false';
  return (
    <SidebarProvider defaultOpen={sidebarOpen} style={SHELL_VARS}>
      <ShellActivityMonitor />
      <TelemetryNotice />
      <AppSidebar />
      <SidebarInset>
        <CommandStrip />
        <main className="@container/content mr-auto w-full max-w-[1440px] flex-1 px-(--shell-gutter) py-10">
          {children}
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}

function TacticalPage(): ReactElement {
  const { jobId = '' } = useParams();
  return <Suspense fallback={<TacticalWorkspaceSkeleton />}><TacticalWorkspace jobId={jobId} /></Suspense>;
}

function EditorProjectPage(): ReactElement {
  const { id = '' } = useParams();
  return <EditorWorkspace projectId={id} />;
}

function ShellRoute({ children }: { children: ReactNode }): ReactElement {
  return <AppShell>{children}</AppShell>;
}

function Application(): ReactElement {
  return (
    <>
      <StudioAmbient />
      <ShellCanvas />
      <ServiceWorkerCleanup />
      <WindowActivityPolicy />
      <DesktopPerformance />
      <DocumentTitle />
      <Routes>
        <Route path="/" element={<Navigate to="/onboarding" replace />} />
        <Route path="/bootstrap" element={<BootstrapPage />} />
        <Route path="/upload" element={<UploadPage />} />
        <Route path="/onboarding" element={<ShellRoute><OnboardingPage /></ShellRoute>} />
        <Route path="/matches" element={<ShellRoute><MatchesPage /></ShellRoute>} />
        <Route path="/matches/:id" element={<ShellRoute><FindHighlightsPage /></ShellRoute>} />
        <Route path="/series/:id" element={<ShellRoute><SeriesPage /></ShellRoute>} />
        <Route path="/full-demo" element={<ShellRoute><FullDemoIndexPage /></ShellRoute>} />
        <Route path="/full-demo/:id" element={<ShellRoute><FullDemoJobPage /></ShellRoute>} />
        <Route path="/tactical" element={<ShellRoute><TacticalIndexPage /></ShellRoute>} />
        <Route path="/tactical/:jobId" element={<ShellRoute><TacticalPage /></ShellRoute>} />
        <Route path="/cheaters" element={<ShellRoute><CheatersPage /></ShellRoute>} />
        <Route path="/players" element={<ShellRoute><PlayersPage /></ShellRoute>} />
        <Route path="/streams" element={<ShellRoute><StreamsPage /></ShellRoute>} />
        <Route path="/editor" element={<ShellRoute><EditorPage /></ShellRoute>} />
        <Route path="/editor/:id" element={<ShellRoute><EditorProjectPage /></ShellRoute>} />
        <Route path="/videos" element={<ShellRoute><VideosPage /></ShellRoute>} />
        <Route path="/feed" element={<ShellRoute><FeedPage /></ShellRoute>} />
        <Route path="/settings" element={<ShellRoute><SettingsPage /></ShellRoute>} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
      <Toaster />
    </>
  );
}

const root = document.getElementById('root');
if (root === null) throw new Error('missing application root');
createRoot(root).render(<StrictMode><BrowserRouter><Application /></BrowserRouter></StrictMode>);
