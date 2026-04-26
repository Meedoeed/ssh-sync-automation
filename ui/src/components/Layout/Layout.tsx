import React from 'react';
import Navbar from './Navbar';

interface LayoutProps {
  children: React.ReactNode;
}

const Layout: React.FC<LayoutProps> = ({ children }) => {
  return (
    <div className="min-h-screen bg-[#FDFDFD] flex">
      <Navbar />

      <div className="flex-1 ml-24 min-h-screen flex flex-col">
        
        <header className="h-16 flex items-center justify-between px-12 bg-white/50 backdrop-blur-sm sticky top-0 z-40">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></div>
            <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Система активна</span>
          </div>
          
          <div className="flex items-center gap-6">
            <button className="text-slate-400 hover:text-slate-900 transition-colors">
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            </button>
            <div className="h-4 w-[1px] bg-slate-200"></div>
            <div className="flex items-center gap-3">
              <span className="text-xs font-bold text-slate-900">Admin</span>
              <div className="w-8 h-8 rounded-full bg-slate-900 flex items-center justify-center text-[10px] text-white font-bold">
                AD
              </div>
            </div>
          </div>
        </header>

        <main className="p-12 animate-in fade-in slide-in-from-bottom-2 duration-700">
          {children}
        </main>
        
      </div>
    </div>
  );
};

export default Layout;