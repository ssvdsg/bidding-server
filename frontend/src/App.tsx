import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Spin } from 'antd';
import LayoutComponent from './components/layout/Layout';

// ── Code-split page-level components ─────────────────────────
const Dashboard = React.lazy(() => import('./pages/Dashboard/Dashboard'));
const Bids = React.lazy(() => import('./pages/Bids/Bids'));
const BidDetail = React.lazy(() => import('./pages/Bids/BidDetail'));
const ExcludedBids = React.lazy(() => import('./pages/Bids/ExcludedBids'));
const Tracked = React.lazy(() => import('./pages/Tracked/Tracked'));
const Companies = React.lazy(() => import('./pages/Companies/Companies'));
const CompanyRecords = React.lazy(() => import('./pages/Companies/CompanyRecords'));
const CompanyAwardDetail = React.lazy(() => import('./pages/Companies/CompanyAwardDetail'));
const Settings = React.lazy(() => import('./pages/Settings/Settings'));
const AITasks = React.lazy(() => import('./pages/Settings/AITasks'));
const ChatPage = React.lazy(() => import('./pages/Chat'));

/** Centered full-height loading spinner used as Suspense fallback */
function PageLoading() {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: '60vh',
        width: '100%',
      }}
    >
      <Spin size="large" />
    </div>
  );
}

function App() {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL.replace(/\/$/, '') || '/'}>
      <Routes>
        <Route path="/" element={<LayoutComponent />}>
          <Route
            index
            element={
              <React.Suspense fallback={<PageLoading />}>
                <Dashboard />
              </React.Suspense>
            }
          />
          <Route
            path="bids"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <Bids />
              </React.Suspense>
            }
          />
          <Route
            path="bids/:id"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <BidDetail />
              </React.Suspense>
            }
          />
          <Route
            path="bids/excluded"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <ExcludedBids />
              </React.Suspense>
            }
          />
          <Route
            path="tracked"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <Tracked />
              </React.Suspense>
            }
          />
          <Route
            path="companies"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <Companies />
              </React.Suspense>
            }
          />
          <Route
            path="companies/record/:id"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <CompanyAwardDetail />
              </React.Suspense>
            }
          />
          <Route
            path="companies/:company"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <CompanyRecords />
              </React.Suspense>
            }
          />
          <Route
            path="tasks"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <AITasks />
              </React.Suspense>
            }
          />
          <Route
            path="chat"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <ChatPage />
              </React.Suspense>
            }
          />
          <Route
            path="settings"
            element={
              <React.Suspense fallback={<PageLoading />}>
                <Settings />
              </React.Suspense>
            }
          />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
