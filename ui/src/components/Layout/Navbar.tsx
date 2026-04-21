import { Link, useLocation } from 'react-router-dom';

const Navbar = () => {
  const location = useLocation();
  
  const links = [
    { path: '/', label: 'Dashboard', icon: '📊' },
    { path: '/servers', label: 'Servers', icon: '🖥️' },
    { path: '/tasks', label: 'Tasks', icon: '📋' },
  ];
  
  return (
    <nav className="bg-gray-800 text-white p-4">
      <div className="container mx-auto flex gap-6">
        <h1 className="text-xl font-bold mr-8">SSH Sync Automation</h1>
        {links.map(link => (
          <Link
            key={link.path}
            to={link.path}
            className={`hover:text-gray-300 ${location.pathname === link.path ? 'text-blue-400' : ''}`}
          >
            <span className="mr-1">{link.icon}</span>
            {link.label}
          </Link>
        ))}
      </div>
    </nav>
  );
};

export default Navbar;