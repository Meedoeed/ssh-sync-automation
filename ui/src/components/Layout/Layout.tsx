import { Outlet } from 'react-router-dom';
import Navbar from './Navbar';

const Layout = () => {
  return (
    <div className="min-h-screen bg-[#FDFDFD] text-slate-900 antialiased">
      <Navbar />
      <main className="container mx-auto pt-32 pb-20 px-6">
        <Outlet />
      </main>
      
      <div className="fixed inset-0 z-[-1] opacity-[0.03] pointer-events-none" 
           style={{ backgroundImage: 'radial-gradient(#000 1px, transparent 1px)', backgroundSize: '40px 40px' }}>
      </div>
    </div>
  );
};

export default Layout;