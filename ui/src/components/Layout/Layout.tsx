import { Outlet } from 'react-router-dom';
import Navbar from './Navbar';

const Layout = () => {
  return (
    <div className="min-h-screen bg-[#FDFDFD] flex">
      <Navbar />

      <div className="flex-1 ml-24 min-h-screen flex flex-col">
        <header className="h-16 flex items-center justify-between px-12 bg-white/50 backdrop-blur-sm sticky top-0 z-40">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></div>
            <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Система активна</span>
          </div>
          
          <div className="flex items-center gap-3">
            <span className="text-xs font-bold text-slate-900">Admin</span>
            <div className="w-8 h-8 rounded-full bg-slate-900 flex items-center justify-center text-[10px] text-white font-bold">
              AD
            </div>
          </div>
        </header>

        <main className="p-12 animate-in fade-in slide-in-from-bottom-2 duration-700">
          <Outlet />
        </main>
        
        <footer className="mt-auto p-12 pt-0">
          <div className="pt-8 border-t border-slate-100 flex justify-between items-center">
            <p className="text-[10px] text-slate-300 font-medium uppercase tracking-tighter">
              &copy; 2026 Script Manager v2.0.4
            </p>
          </div>
        </footer>
      </div>
    </div>
  );
};

export default Layout;