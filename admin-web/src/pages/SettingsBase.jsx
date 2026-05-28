import { Tabs } from 'antd';

const settingTabs = [
  { key: 'site', label: '站点设置' },
  { key: 'service', label: '服务配置' },
  { key: 'security', label: '安全设置' },
  { key: 'database', label: '数据库设置' },
];

function EmptyPanel() {
  return <div className="qdl-settings-empty" />;
}

export function SettingsBase() {
  return (
    <div className="qdl-settings-tabs">
      <Tabs
        tabPosition="left"
        defaultActiveKey="site"
        items={settingTabs.map((item) => ({
          ...item,
          children: <EmptyPanel />,
        }))}
      />
    </div>
  );
}
