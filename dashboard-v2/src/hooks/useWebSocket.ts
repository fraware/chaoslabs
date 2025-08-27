import { useEffect, useRef, useState, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';

interface WebSocketOptions {
  url: string;
  protocols?: string | string[];
  reconnectAttempts?: number;
  reconnectInterval?: number;
  onMessage?: (event: MessageEvent) => void;
  onError?: (event: Event) => void;
  onOpen?: (event: Event) => void;
  onClose?: (event: CloseEvent) => void;
}

interface WebSocketState {
  socket: WebSocket | null;
  isConnected: boolean;
  lastMessage: any;
  connectionState: 'connecting' | 'connected' | 'disconnected' | 'error';
}

export function useWebSocket(options: WebSocketOptions) {
  const {
    url,
    protocols,
    reconnectAttempts = 5,
    reconnectInterval = 3000,
    onMessage,
    onError,
    onOpen,
    onClose,
  } = options;

  const [state, setState] = useState<WebSocketState>({
    socket: null,
    isConnected: false,
    lastMessage: null,
    connectionState: 'disconnected',
  });

  const queryClient = useQueryClient();
  const reconnectCount = useRef(0);
  const reconnectTimer = useRef<NodeJS.Timeout>();

  const connect = useCallback(() => {
    setState(prev => ({ ...prev, connectionState: 'connecting' }));
    
    try {
      const socket = new WebSocket(url, protocols);
      
      socket.onopen = (event) => {
        setState(prev => ({
          ...prev,
          socket,
          isConnected: true,
          connectionState: 'connected',
        }));
        reconnectCount.current = 0;
        onOpen?.(event);
      };

      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          setState(prev => ({ ...prev, lastMessage: data }));
          
          // Handle real-time updates for React Query
          if (data.type === 'experiment_update') {
            queryClient.invalidateQueries({ queryKey: ['experiments'] });
            queryClient.invalidateQueries({ queryKey: ['experiment', data.experiment_id] });
          }
          
          if (data.type === 'notification') {
            queryClient.invalidateQueries({ queryKey: ['notifications'] });
          }
          
          onMessage?.(event);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      socket.onerror = (event) => {
        setState(prev => ({ ...prev, connectionState: 'error' }));
        onError?.(event);
      };

      socket.onclose = (event) => {
        setState(prev => ({
          ...prev,
          socket: null,
          isConnected: false,
          connectionState: 'disconnected',
        }));
        onClose?.(event);
        
        // Attempt to reconnect if not manually closed
        if (!event.wasClean && reconnectCount.current < reconnectAttempts) {
          reconnectCount.current++;
          reconnectTimer.current = setTimeout(() => {
            connect();
          }, reconnectInterval * Math.pow(2, reconnectCount.current - 1)); // Exponential backoff
        }
      };

    } catch (error) {
      setState(prev => ({ ...prev, connectionState: 'error' }));
      console.error('Failed to create WebSocket connection:', error);
    }
  }, [url, protocols, reconnectAttempts, reconnectInterval, onMessage, onError, onOpen, onClose, queryClient]);

  const disconnect = useCallback(() => {
    if (reconnectTimer.current) {
      clearTimeout(reconnectTimer.current);
    }
    
    if (state.socket) {
      state.socket.close(1000, 'User disconnected');
    }
  }, [state.socket]);

  const sendMessage = useCallback((data: any) => {
    if (state.socket && state.isConnected) {
      state.socket.send(JSON.stringify(data));
      return true;
    }
    return false;
  }, [state.socket, state.isConnected]);

  useEffect(() => {
    connect();
    
    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
      }
    };
  }, []);

  return {
    ...state,
    connect,
    disconnect,
    sendMessage,
  };
}

// Custom hook for experiment updates
export function useExperimentUpdates() {
  const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`;
  
  return useWebSocket({
    url: wsUrl,
    onMessage: (event) => {
      const data = JSON.parse(event.data);
      if (data.type === 'welcome') {
        console.log('Connected to experiment updates');
      }
    },
    onError: (error) => {
      console.error('WebSocket error:', error);
    },
  });
}