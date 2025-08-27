import React, { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  HomeIcon,
  BeakerIcon,
  DocumentTextIcon,
  CogIcon,
  Bars3Icon,
  XMarkIcon,
  SignalIcon,
} from '@heroicons/react/24/outline';
import { clsx } from 'clsx';
import { useConnectionStatus } from '../hooks/useConnectionStatus';
import { NotificationCenter } from './NotificationCenter';

interface Props {
  children: React.ReactNode;
}

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: HomeIcon },
  { name: 'Experiments', href: '/experiments', icon: BeakerIcon },
  { name: 'Audit Pack', href: '/audit-pack', icon: DocumentTextIcon },
  { name: 'Settings', href: '/settings', icon: CogIcon },
];

export function Layout({ children }: Props) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const location = useLocation();
  const { isOnline, latency } = useConnectionStatus();

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Mobile sidebar */}
      <div className={clsx(
        'fixed inset-0 z-40 lg:hidden',
        sidebarOpen ? 'block' : 'hidden'
      )}>
        <div 
          className="fixed inset-0 bg-gray-600 bg-opacity-75"
          onClick={() => setSidebarOpen(false)}
        />
        
        <div className="relative flex w-64 flex-col bg-white">
          <div className="absolute top-0 right-0 -mr-12 pt-2">
            <button
              type="button"
              className="ml-1 flex h-10 w-10 items-center justify-center rounded-full focus:outline-none focus:ring-2 focus:ring-inset focus:ring-white"
              onClick={() => setSidebarOpen(false)}
            >
              <XMarkIcon className="h-6 w-6 text-white" />
            </button>
          </div>
          
          <Sidebar />
        </div>
      </div>

      {/* Desktop sidebar */}
      <div className="hidden lg:fixed lg:inset-y-0 lg:flex lg:w-64 lg:flex-col">
        <Sidebar />
      </div>

      {/* Main content */}
      <div className="lg:pl-64">
        {/* Top navigation */}
        <div className="sticky top-0 z-30 bg-white shadow-sm">
          <div className="flex h-16 items-center gap-x-4 border-b border-gray-200 px-4 sm:gap-x-6 sm:px-6 lg:px-8">
            <button
              type="button"
              className="-m-2.5 p-2.5 text-gray-700 lg:hidden"
              onClick={() => setSidebarOpen(true)}
            >
              <Bars3Icon className="h-6 w-6" />
            </button>

            <div className="h-6 w-px bg-gray-200 lg:hidden" />

            <div className="flex flex-1 items-center justify-between">
              <h1 className="text-lg font-semibold text-gray-900">
                {navigation.find(item => item.href === location.pathname)?.name || 'ChaosLabs'}
              </h1>
              
              <div className="flex items-center gap-x-4">
                {/* Connection status */}
                <div className="flex items-center gap-x-2 text-sm">
                  <SignalIcon className={clsx(
                    'h-5 w-5',
                    isOnline ? 'text-green-500' : 'text-red-500'
                  )} />
                  <span className={clsx(
                    'hidden sm:inline',
                    isOnline ? 'text-green-700' : 'text-red-700'
                  )}>
                    {isOnline ? `Online (${latency}ms)` : 'Offline'}
                  </span>
                </div>
                
                <NotificationCenter />
              </div>
            </div>
          </div>
        </div>

        {/* Page content */}
        <main className="py-6">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}

function Sidebar() {
  const location = useLocation();

  return (
    <div className="flex grow flex-col gap-y-5 overflow-y-auto bg-white px-6 pb-4">
      <div className="flex h-16 shrink-0 items-center">
        <div className="text-xl font-bold text-chaos-600">
          ChaosLabs
        </div>
      </div>
      
      <nav className="flex flex-1 flex-col">
        <ul role="list" className="flex flex-1 flex-col gap-y-7">
          <li>
            <ul role="list" className="-mx-2 space-y-1">
              {navigation.map((item) => {
                const isActive = location.pathname === item.href;
                
                return (
                  <li key={item.name}>
                    <Link
                      to={item.href}
                      className={clsx(
                        isActive
                          ? 'bg-chaos-50 text-chaos-700'
                          : 'text-gray-700 hover:bg-gray-50 hover:text-chaos-700',
                        'group flex gap-x-3 rounded-md p-2 text-sm font-semibold leading-6'
                      )}
                    >
                      <item.icon
                        className={clsx(
                          isActive ? 'text-chaos-700' : 'text-gray-400 group-hover:text-chaos-700',
                          'h-6 w-6 shrink-0'
                        )}
                      />
                      {item.name}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </li>
        </ul>
      </nav>
    </div>
  );
}
