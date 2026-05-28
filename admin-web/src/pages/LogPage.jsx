import { BellOutlined, FieldTimeOutlined, LoginOutlined, SafetyCertificateOutlined, UnorderedListOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import { Empty, Space, Tabs, Tag } from 'antd';
import { listResource } from '../services/api';

const statusMap = {
  success: { color: 'green', text: '成功' },
  failed: { color: 'red', text: '失败' },
};

const certificateActionMap = {
  apply: '申请',
  submit: '提交',
  revoke: '吊销',
  detail: '详情',
};

function statusRender(_, row) {
  const item = statusMap[row.status] || { color: 'default', text: row.status || '-' };
  return <Tag color={item.color}>{item.text}</Tag>;
}

function LogTable({ resource, columns }) {
  return (
    <ProTable
      key={resource}
      rowKey="id"
      columns={columns}
      search={false}
      options={false}
      request={async (params) => {
        const res = await listResource(resource, { page: params.current, pageSize: params.pageSize });
        return { data: res.data.items, total: res.data.total, success: true };
      }}
      scroll={{ x: 'max-content' }}
      pagination={{ defaultPageSize: 10 }}
    />
  );
}

const loginColumns = [
  { title: '用户名', dataIndex: 'username', width: 140 },
  { title: '登录IP', dataIndex: 'ip', width: 150 },
  { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
  { title: '说明', dataIndex: 'message', width: 180 },
  { title: 'User-Agent', dataIndex: 'userAgent', ellipsis: true },
  { title: '登录时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
];

const operationColumns = [
  { title: '用户', dataIndex: 'username', width: 120 },
  { title: '动作', dataIndex: 'action', width: 120 },
  { title: '资源', dataIndex: 'resource', width: 150 },
  { title: '方法', dataIndex: 'method', width: 90 },
  { title: '路径', dataIndex: 'path', ellipsis: true },
  { title: 'IP', dataIndex: 'ip', width: 150 },
  { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
  { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
];

const certificateColumns = [
  { title: '证书域名', dataIndex: 'commonName', width: 220 },
  { title: '动作', dataIndex: 'action', width: 100, render: (_, row) => certificateActionMap[row.action] || row.action || '-' },
  { title: '服务商', dataIndex: 'provider', width: 140 },
  { title: '腾讯云证书ID', dataIndex: 'providerCertificateId', width: 180, render: (_, row) => row.providerCertificateId || '-' },
  { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
  { title: '说明', dataIndex: 'message', ellipsis: true },
  { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
];

function PendingPanel() {
  return (
    <div className="qdl-log-empty">
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂未实现" />
    </div>
  );
}

function TabLabel({ icon, children }) {
  return (
    <Space size={6}>
      {icon}
      <span>{children}</span>
    </Space>
  );
}

export function LogPage() {
  return (
    <div className="qdl-log-page">
      <Tabs
        defaultActiveKey="login"
        items={[
          {
            key: 'login',
            label: <TabLabel icon={<LoginOutlined />}>登录日志</TabLabel>,
            children: <LogTable resource="login-logs" columns={loginColumns} />,
          },
          {
            key: 'operation',
            label: <TabLabel icon={<UnorderedListOutlined />}>操作日志</TabLabel>,
            children: <LogTable resource="logs" columns={operationColumns} />,
          },
          {
            key: 'certificate',
            label: <TabLabel icon={<SafetyCertificateOutlined />}>证书日志</TabLabel>,
            children: <LogTable resource="certificate-logs" columns={certificateColumns} />,
          },
          {
            key: 'notification',
            label: <TabLabel icon={<BellOutlined />}>通知日志</TabLabel>,
            children: <PendingPanel />,
          },
          {
            key: 'task',
            label: <TabLabel icon={<FieldTimeOutlined />}>任务日志</TabLabel>,
            children: <PendingPanel />,
          },
        ]}
      />
    </div>
  );
}
