import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import './index.css';

// Configure React Query with performance optimizations
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000, // 30 seconds
      gcTime: 5 * 60 * 1000, // 5 minutes (renamed from cacheTime)
      retry: (failureCount, error) => {
        // Don't retry on 4xx errors, but retry on network errors
        if (error instanceof Error && 'status' in error && (error as any).status >= 400 && (error as any).status < 500) {
          return false;
        }
        return failureCount < 3;
      },
      refetchOnWindowFocus: false,
      refetchOnReconnect: true,
    },
    mutations: {
      retry: 1,
    },
  },
});

// Enable React DevTools in development
if (import.meta.env.DEV) {
  import('@tanstack/react-query-devtools').then(({ ReactQueryDevtools }) => {
    const devtools = React.createElement(ReactQueryDevtools, { initialIsOpen: false });
    // Add devtools to the app
  });
}

const root = ReactDOM.createRoot(document.getElementById('root')!);

root.render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
);

// Register service worker for offline support
if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js')
      .then((registration) => {
        console.log('SW registered: ', registration);
      })
      .catch((registrationError) => {
        console.log('SW registration failed: ', registrationError);
      });
  });
}