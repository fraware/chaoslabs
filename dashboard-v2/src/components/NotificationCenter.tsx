import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { BellIcon } from '@heroicons/react/24/outline';
import { clsx } from 'clsx';

interface Notification {
  id: string;
  type: 'info' | 'warning' | 'error' | 'success';
  title: string;
  message: string;
  timestamp: string;
  read: boolean;
}

// Mock notifications
const mockNotifications: Notification[] = [
  {
    id: '1',
    type: 'success',
    title: 'Experiment Completed',
    message: 'Network latency experiment on web-server completed successfully',
    timestamp: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
    read: false,
  },
  {
    id: '2',
    type: 'warning',
    title: 'High Latency Detected',
    message: 'API response times are higher than normal',
    timestamp: new Date(Date.now() - 15 * 60 * 1000).toISOString(),
    read: false,
  },
  {
    id: '3',
    type: 'error',
    title: 'Experiment Failed',
    message: 'CPU stress test failed due to insufficient permissions',
    timestamp: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
    read: true,
  },
];

async function fetchNotifications(): Promise<Notification[]> {
  // Simulate API call
  await new Promise(resolve => setTimeout(resolve, 200));
  return mockNotifications;
}

export function NotificationCenter() {
  const [isOpen, setIsOpen] = useState(false);

  const { data: notifications = [] } = useQuery({
    queryKey: ['notifications'],
    queryFn: fetchNotifications,
    staleTime: 30 * 1000,
    refetchInterval: 60 * 1000,
  });

  const unreadCount = notifications.filter(n => !n.read).length;

  const getNotificationIcon = (type: Notification['type']) => {
    switch (type) {
      case 'success':
        return '✅';
      case 'warning':
        return '⚠️';
      case 'error':
        return '❌';
      default:
        return 'ℹ️';
    }
  };

  const timeAgo = (timestamp: string) => {
    const diff = Date.now() - new Date(timestamp).getTime();
    const minutes = Math.floor(diff / (1000 * 60));
    const hours = Math.floor(diff / (1000 * 60 * 60));
    
    if (minutes < 1) return 'Just now';
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    return new Date(timestamp).toLocaleDateString();
  };

  return (
    <div className="relative">
      <button
        type="button"
        className="relative p-2 text-gray-400 hover:text-gray-500 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-chaos-500"
        onClick={() => setIsOpen(!isOpen)}
      >
        <BellIcon className="h-6 w-6" />
        {unreadCount > 0 && (
          <span className="absolute top-0 right-0 inline-flex items-center justify-center px-2 py-1 text-xs font-bold leading-none text-red-100 transform translate-x-1/2 -translate-y-1/2 bg-red-600 rounded-full">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {isOpen && (
        <>
          <div
            className="fixed inset-0 z-10"
            onClick={() => setIsOpen(false)}
          />
          <div className="absolute right-0 z-20 mt-2 w-80 origin-top-right rounded-md bg-white py-1 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none">
            <div className="px-4 py-2 border-b border-gray-200">
              <h3 className="text-sm font-medium text-gray-900">Notifications</h3>
            </div>
            
            <div className="max-h-96 overflow-y-auto">
              {notifications.length === 0 ? (
                <div className="px-4 py-8 text-center text-sm text-gray-500">
                  No notifications
                </div>
              ) : (
                notifications.map((notification) => (
                  <div
                    key={notification.id}
                    className={clsx(
                      'px-4 py-3 hover:bg-gray-50 border-b border-gray-100 last:border-b-0',
                      !notification.read && 'bg-blue-50'
                    )}
                  >
                    <div className="flex items-start">
                      <span className="text-lg mr-3">
                        {getNotificationIcon(notification.type)}
                      </span>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-gray-900">
                          {notification.title}
                        </p>
                        <p className="text-sm text-gray-500 mt-1">
                          {notification.message}
                        </p>
                        <p className="text-xs text-gray-400 mt-1">
                          {timeAgo(notification.timestamp)}
                        </p>
                      </div>
                      {!notification.read && (
                        <div className="w-2 h-2 bg-blue-600 rounded-full ml-2 mt-1"></div>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
            
            {notifications.length > 0 && (
              <div className="px-4 py-2 border-t border-gray-200">
                <button className="text-sm text-chaos-600 hover:text-chaos-700 font-medium">
                  Mark all as read
                </button>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}