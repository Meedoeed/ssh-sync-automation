import { Link, useLocation } from 'react-router-dom';

const Navbar = () => {
  const location = useLocation();

  const menuItems = [
    { path: '/', label: 'Дашборд', icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
      </svg>
    )},
    { path: '/servers', label: 'Серверы', icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
      </svg>
    )},
    { path: '/tasks', label: 'Задачи', icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
      </svg>
    )},
  ];

  return (
    <nav className="fixed left-0 top-0 h-screen w-24 bg-white border-r border-slate-100 flex flex-col items-center py-10 z-50">
      <div className="mb-12">
        <div className="w-12 h-12 bg-slate-900 rounded-2xl flex items-center justify-center text-white shadow-xl shadow-slate-200">
          <span className="font-black text-xl italic">S</span>
        </div>
      </div>

      <div className="flex-1 flex flex-col gap-8">
        {menuItems.map((item) => {
          const isActive = location.pathname === item.path;
          return (
            <Link
              key={item.path}
              to={item.path}
              className="relative group"
            >
              <div className={`p-4 rounded-2xl transition-all duration-300 ${
                isActive 
                  ? 'bg-slate-50 text-slate-900 shadow-sm' 
                  : 'text-slate-400 hover:text-slate-600 hover:bg-slate-50/50'
              }`}>
                {item.icon}
              </div>
              
              <span className="absolute left-20 top-1/2 -translate-y-1/2 bg-slate-900 text-white text-[10px] font-bold uppercase tracking-widest px-3 py-1.5 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none whitespace-nowrap z-50">
                {item.label}
              </span>

              {isActive && (
                <div className="absolute -left-6 top-1/2 -translate-y-1/2 w-1 h-8 bg-slate-900 rounded-r-full" />
              )}
            </Link>
          );
        })}
      </div>
    </nav>
  );
};

export default Navbar;