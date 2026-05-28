import React, { useEffect, useMemo, useState } from 'react';
import ReactDOM from 'react-dom/client';
import {
  BarsOutlined,
  DashboardOutlined,
  DeploymentUnitOutlined,
  FileProtectOutlined,
  FieldTimeOutlined,
  GlobalOutlined,
  LogoutOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import { PageContainer, ProLayout } from '@ant-design/pro-components';
import { ConfigProvider, Dropdown, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { api, getToken, setToken } from './services/api';
import { Login } from './pages/Login/Login';
import { Dashboard } from './pages/Dashboard/Dashboard';
import { DomainPage, DomainRecordsPage } from './pages/DomainPage';
import { CertificatePage } from './pages/CertificatePage';
import { ResourcePage } from './pages/ResourcePage';
import { SSLAccountPage } from './pages/SSLAccountPage';
import { SettingsBase } from './pages/SettingsBase';
import { LogPage } from './pages/LogPage';
import './styles.css';

const routes = [
  { path: '/dashboard', name: '仪表盘', icon: <DashboardOutlined /> },
  { path: '/domains', name: '域名管理', icon: <GlobalOutlined /> },
  { path: '/certificates', name: 'SSL证书', icon: <SafetyCertificateOutlined /> },
  { path: '/tasks', name: '自动任务', icon: <FieldTimeOutlined /> },
  {
    path: '/accounts',
    name: '账号管理',
    icon: <TeamOutlined />,
    routes: [
      { path: '/accounts/domain', name: '域名账号', icon: <DeploymentUnitOutlined /> },
      { path: '/accounts/ssl', name: 'SSL账号', icon: <FileProtectOutlined /> },
      { path: '/accounts/deploy', name: '部署账号', icon: <DeploymentUnitOutlined /> },
    ],
  },
  { path: '/logs', name: '日志管理', icon: <BarsOutlined /> },
  {
    path: '/settings',
    name: '系统设置',
    icon: <SettingOutlined />,
    routes: [
      { path: '/settings/base', name: '基础设置' },
      { path: '/settings/profile', name: '个人设置' },
      { path: '/settings/notice', name: '通知设置' },
    ],
  },
];

const route = {
  path: '/',
  routes,
};

const pageMap = {
  '/dashboard': <Dashboard />,
  '/domains': <DomainPage />,
  '/certificates': <CertificatePage />,
  '/tasks': <ResourcePage title="自动任务" resource="tasks" columnsPreset="tasks" />,
  '/accounts/domain': <ResourcePage title="域名账号" resource="domain-accounts" columnsPreset="domainAccounts" />,
  '/accounts/ssl': <SSLAccountPage />,
  '/accounts/deploy': <div className="qdl-placeholder-page" />,
  '/logs': <LogPage />,
  '/settings/base': <SettingsBase />,
  '/settings/profile': <ResourcePage title="个人设置" resource="users" columnsPreset="users" />,
  '/settings/notice': <ResourcePage title="通知设置" resource="notification-settings" columnsPreset="notices" />,
};

const routeMeta = routes.reduce((acc, item) => {
  acc[item.path] = { ...item };
  item.routes?.forEach((child) => {
    acc[child.path] = { ...child, parent: item };
  });
  return acc;
}, {});

function resolveRoute(pathname) {
  const domainRecordMatch = pathname.match(/^\/domains\/(\d+)$/);
  if (domainRecordMatch) {
    return {
      current: { path: pathname, name: 'DNS记录', parent: routeMeta['/domains'] },
      page: <DomainRecordsPage domainId={domainRecordMatch[1]} />,
      menuPathname: '/domains',
    };
  }
  return {
    current: routeMeta[pathname] || routeMeta['/dashboard'],
    page: pageMap[pathname] || <Dashboard />,
    menuPathname: pathname,
  };
}

function App() {
  const [pathname, setPathname] = useState(window.location.pathname === '/' ? '/dashboard' : window.location.pathname);
  const [user, setUser] = useState(null);
  const [ready, setReady] = useState(false);
  const [openKeys, setOpenKeys] = useState(() => {
    const initialPath = window.location.pathname === '/' ? '/dashboard' : window.location.pathname;
    const initial = routeMeta[initialPath];
    return initial?.parent ? [initial.parent.path] : [];
  });

  useEffect(() => {
    window.history.replaceState(null, '', pathname);
  }, [pathname]);

  useEffect(() => {
    const onPopState = () => setPathname(window.location.pathname === '/' ? '/dashboard' : window.location.pathname);
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  useEffect(() => {
    const meta = routeMeta[pathname];
    setOpenKeys(meta?.parent ? [meta.parent.path] : []);
  }, [pathname]);

  useEffect(() => {
    if (!getToken()) {
      setReady(true);
      return;
    }
    api('/auth/me')
      .then((res) => setUser(res.data))
      .catch(() => setToken(''))
      .finally(() => setReady(true));
  }, []);

  const menuItems = useMemo(() => [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: () => {
        setToken('');
        setUser(null);
      },
    },
  ], []);

  const resolved = resolveRoute(pathname);
  const current = resolved.current;
  const navigate = (path) => {
    setPathname(path);
    window.history.pushState(null, '', path);
  };
  const breadcrumbItems = [
    ...(current.parent ? [{ title: current.parent.name }] : []),
    { title: current.name },
  ];
  if (!ready) return null;

  if (!user) {
    return <Login onSuccess={setUser} />;
  }

  return (
    <ProLayout
      className="qdl-pro-layout"
      title={false}
      logo={<img className="qdl-brand-logo" src="/logo.svg" alt="DaSSLm" />}
      layout="mix"
      navTheme="light"
      fixSiderbar
      fixedHeader
      route={route}
      location={{ pathname: resolved.menuPathname }}
      menu={{ defaultOpenAll: false }}
      menuProps={{
        openKeys,
        onOpenChange: setOpenKeys,
      }}
      menuItemRender={(item, dom) => (
        <a
          onClick={() => {
            if (!item.path) return;
            setPathname(item.path);
            window.history.pushState(null, '', item.path);
          }}
        >
          {dom}
        </a>
      )}
      subMenuItemRender={(item, dom) => dom}
      avatarProps={{
        title: user.nickname || user.username,
        render: (_, dom) => <Dropdown menu={{ items: menuItems }}>{dom}</Dropdown>,
      }}
      token={{
        header: {
          colorBgHeader: '#ffffff',
          colorHeaderTitle: '#111827',
          colorTextMenu: '#4b5563',
          colorTextMenuSelected: '#1677ff',
          heightLayoutHeader: 54,
        },
        sider: {
          colorMenuBackground: '#fafafa',
          colorBgMenuItemSelected: '#f0f0f0',
          colorTextMenu: '#333333',
          colorTextMenuSelected: '#111111',
          colorTextMenuItemHover: '#111111',
        },
        pageContainer: {
          paddingInlinePageContainerContent: 30,
          paddingBlockPageContainerContent: 24,
        },
      }}
    >
      <PageContainer
        className="qdl-page-container"
        title={current.name}
        breadcrumb={{ items: breadcrumbItems }}
      >
        {React.cloneElement(resolved.page, { key: pathname, navigate })}
      </PageContainer>
    </ProLayout>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(
  <ConfigProvider locale={zhCN} theme={{ algorithm: theme.defaultAlgorithm }}>
    <App />
  </ConfigProvider>,
);
