import { useState, useEffect, useCallback } from 'react';

interface ConnectionStatus {
  isOnline: boolean;
  latency: number;
  lastCheck: Date;
}

export function useConnectionStatus() {
  const [status, setStatus] = useState<ConnectionStatus>({
    isOnline: navigator.onLine,
    latency: 0,
    lastCheck: new Date(),
  });

  const checkConnection = useCallback(async () => {
    const start = performance.now();
    
    try {
      // Ping the health endpoint to check connectivity and latency
      const response = await fetch('/api/healthz', {
        method: 'HEAD',
        cache: 'no-cache',
      });
      
      const end = performance.now();
      const latency = Math.round(end - start);
      
      setStatus({
        isOnline: response.ok,
        latency,
        lastCheck: new Date(),
      });
    } catch (error) {
      setStatus({
        isOnline: false,
        latency: 0,
        lastCheck: new Date(),
      });
    }
  }, []);

  useEffect(() => {
    // Initial check
    checkConnection();

    // Set up periodic checks
    const interval = setInterval(checkConnection, 30000); // Check every 30 seconds

    // Listen for online/offline events
    const handleOnline = () => {
      checkConnection();
    };

    const handleOffline = () => {
      setStatus(prev => ({
        ...prev,
        isOnline: false,
        lastCheck: new Date(),
      }));
    };

    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    return () => {
      clearInterval(interval);
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
    };
  }, [checkConnection]);

  return status;
}
