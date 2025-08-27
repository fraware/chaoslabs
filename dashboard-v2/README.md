# ChaosLabs Dashboard v2

A modern, high-performance React dashboard for chaos engineering with state-of-the-art performance optimizations.

## ✨ Features

### Performance Optimizations (P10)

- **React Query**: Request deduplication, intelligent caching, and background updates
- **Virtualized Lists**: Handle 50,000+ rows smoothly with `@tanstack/react-virtual`
- **Code Splitting**: Lazy-loaded routes and chunked bundles for optimal loading
- **SSE/WebSocket Streaming**: Real-time experiment updates
- **Offline Audit Pack Viewer**: PWA with offline capabilities
- **Bundle Optimization**: Manual chunk splitting for vendor libraries

### Key Technologies

- **React 18** with concurrent features
- **TypeScript** for type safety
- **Vite** for fast development and optimized builds
- **Tailwind CSS** for utility-first styling
- **React Query** for server state management
- **React Virtual** for performance with large datasets
- **PWA** support with Workbox

## 🚀 Performance Goals

- **Time-to-Interactive**: ↓ ≥30% compared to v1
- **Large Dataset Handling**: Smooth browsing of 50,000+ rows
- **Bundle Size**: Optimized chunks with lazy loading
- **Offline Support**: Full audit pack viewing without internet

## 📦 Installation

```bash
cd dashboard-v2
npm install
```

## 🛠️ Development

```bash
# Start development server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Type checking
npm run type-check

# Linting
npm run lint

# Bundle analysis
npm run analyze
```

## 📊 Performance Features

### Virtualized Table Component

```tsx
import { VirtualizedTable } from '@/components/VirtualizedTable';

<VirtualizedTable
  data={experiments}
  columns={columns}
  rowHeight={56}
  overscan={10}
  isLoading={isLoading}
/>
```

### React Query Integration

```tsx
import { useQuery } from '@tanstack/react-query';

const { data, isLoading } = useQuery({
  queryKey: ['experiments'],
  queryFn: fetchExperiments,
  staleTime: 30 * 1000, // 30 seconds
  refetchInterval: 60 * 1000, // 1 minute
});
```

### Real-time Updates

```tsx
import { useExperimentUpdates } from '@/hooks/useWebSocket';

function ExperimentsList() {
  // Automatically invalidates React Query cache on updates
  useExperimentUpdates();
  
  // ... component logic
}
```

## 🏗️ Architecture

### Component Structure

```
src/
├── components/          # Reusable UI components
│   ├── VirtualizedTable.tsx
│   ├── ErrorBoundary.tsx
│   └── Layout.tsx
├── hooks/              # Custom React hooks
│   ├── useWebSocket.ts
│   └── useConnectionStatus.ts
├── pages/              # Route components (lazy-loaded)
│   ├── Dashboard.tsx
│   ├── ExperimentsList.tsx
│   └── AuditPack.tsx
└── main.tsx           # Application entry point
```

### Performance Optimizations

1. **Code Splitting**: Each route is lazy-loaded
2. **Bundle Chunking**: Vendor libraries separated
3. **Resource Hints**: DNS prefetch, preconnect
4. **Critical CSS**: Inlined to prevent FOUC
5. **Web Vitals**: Monitored and optimized

### PWA Features

- **Service Worker**: Caches assets and API responses
- **Offline Mode**: View audit packs without internet
- **App Manifest**: Installable as desktop/mobile app
- **Background Sync**: Queue actions when offline

## 🔧 Configuration

### Vite Configuration

- **Manual Chunks**: Optimized bundle splitting
- **Proxy Setup**: API routes proxied to controller
- **PWA Plugin**: Service worker generation
- **Build Optimization**: ESBuild minification

### Tailwind Configuration

- **Custom Colors**: Chaos brand colors
- **Performance**: Purged unused styles
- **Animations**: Optimized for performance
- **Dark Mode**: System preference support

## 📱 Responsive Design

- **Mobile-First**: Optimized for all screen sizes
- **Touch-Friendly**: Proper touch targets
- **Accessibility**: WCAG 2.1 compliance
- **Performance**: Optimized for mobile networks

## 🧪 Testing

```bash
# Unit tests
npm run test

# E2E tests
npm run test:e2e

# Performance tests
npm run test:perf
```

## 🚀 Deployment

```bash
# Build production bundle
npm run build

# Serve with any static server
npx serve dist

# Docker deployment
docker build -t chaoslabs-dashboard .
docker run -p 3000:3000 chaoslabs-dashboard
```

## 📈 Performance Monitoring

The dashboard includes built-in performance monitoring:

- **Web Vitals**: LCP, FID, CLS tracking
- **Bundle Analysis**: Size and loading metrics
- **Real User Monitoring**: Performance in production
- **Error Tracking**: Boundary and network errors

## 🔒 Security

- **Content Security Policy**: XSS protection
- **HTTPS Only**: Secure communication
- **Input Validation**: Client-side validation
- **CORS**: Proper cross-origin handling

## 🌐 Browser Support

- **Chrome**: 90+
- **Firefox**: 88+
- **Safari**: 14+
- **Edge**: 90+

## 📚 Documentation

- [Component API](./docs/components.md)
- [Performance Guide](./docs/performance.md)
- [Deployment Guide](./docs/deployment.md)
- [Contributing](./docs/contributing.md)