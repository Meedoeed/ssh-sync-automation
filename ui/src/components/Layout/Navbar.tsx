import { Link, useLocation } from 'react-router-dom';

const Navbar = () => {
  const location = useLocation();
  
  const links = [
    { path: '/', label: 'Дашборд' },
    { path: '/servers', label: 'Серверы' },
    { path: '/tasks', label: 'Задачи' },
  ];
  
  return (
    <nav className="fixed top-0 left-0 right-0 z-50 bg-white/80 backdrop-blur-md border-b border-slate-100">
      <div className="container mx-auto px-6 h-20 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="w-8 h-8 bg-blue-600 rounded-lg shadow-lg shadow-blue-200 flex items-center justify-center">
            <div className="w-1 h-4 bg-white rounded-full rotate-12"></div>
          </div>
          <span className="text-xl font-black tracking-tighter uppercase italic">SSH_SYNC</span>
        </div>
        
        <div className="flex gap-2">
          {links.map(link => {
            const active = location.pathname === link.path;
            return (
              <Link
                key={link.path}
                to={link.path}
                className={`px-5 py-2 rounded-xl text-xs font-black uppercase tracking-widest transition-all ${
                  active 
                    ? 'bg-slate-900 text-white' 
                    : 'text-slate-400 hover:text-slate-900 hover:bg-slate-50'
                }`}
              >
                {link.label}
              </Link>
            );
          })}
        </div>
      </div>
    </nav>
  );
};

export default Navbar;