import { BellOutlined, CloudServerOutlined, DeploymentUnitOutlined, EyeOutlined, FieldTimeOutlined, LoginOutlined, SafetyCertificateOutlined, UnorderedListOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import { Descriptions, Drawer, Empty, Space, Tabs, Tag, Typography } from 'antd';
import { useMemo, useState } from 'react';
import { listResource } from '../services/api';

const statusMap = {
  success: { color: 'green', text: '成功' },
  failed: { color: 'red', text: '失败' },
};

const certificateActionMap = {
  apply: '申请',
  submit: '提交',
  verify: '验证',
  revoke: '吊销',
  revoke_verify: '吊销验证',
  revoke_check: '验证检查',
  revoke_status: '吊销状态',
  detail: '详情',
  cleanup: '清理',
};

const domainActionMap = {
  test: '检测',
  domains: '域名列表',
  records: '解析记录',
  record_lines: '解析线路',
  create_record: '新增记录',
  update_record: '更新记录',
  delete_record: '删除记录',
};

function statusRender(_, row) {
  const item = statusMap[row.status] || { color: 'default', text: row.status || '-' };
  return <Tag color={item.color}>{item.text}</Tag>;
}

function ActionTag({ icon, color = 'blue', children, onClick }) {
  return (
    <Tag className="qdl-action-tag" color={color} icon={icon} onClick={onClick}>
      {children}
    </Tag>
  );
}

function TextBlock({ value }) {
  const displayValue = formatLogText(value);
  return (
    <Typography.Paragraph copyable={!!displayValue} className="qdl-log-detail-text">
      {displayValue || '-'}
    </Typography.Paragraph>
  );
}

function formatLogText(value) {
  if (!value) return '';
  const text = String(value);
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
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
  const [certificateDetail, setCertificateDetail] = useState(null);
  const [deployDetail, setDeployDetail] = useState(null);
  const [domainDetail, setDomainDetail] = useState(null);

  const certificateColumns = useMemo(() => [
    { title: '证书域名', dataIndex: 'commonName', width: 220 },
    { title: '动作', dataIndex: 'action', width: 100, render: (_, row) => certificateActionMap[row.action] || row.action || '-' },
    { title: '服务商', dataIndex: 'provider', width: 140 },
    { title: '腾讯云证书ID', dataIndex: 'providerCertificateId', width: 180, render: (_, row) => row.providerCertificateId || '-' },
    { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
    { title: '说明', dataIndex: 'message', ellipsis: true },
    { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 90,
      render: (_, row) => (
        <ActionTag icon={<EyeOutlined />} color="default" onClick={() => setCertificateDetail(row)}>
          详情
        </ActionTag>
      ),
    },
  ], []);

  const deployColumns = useMemo(() => [
    { title: '证书域名', dataIndex: 'commonName', width: 220 },
    { title: '部署服务', dataIndex: 'provider', width: 140 },
    { title: '目标站点', dataIndex: 'targetName', width: 200, render: (_, row) => row.targetName || '-' },
    { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
    { title: '说明', dataIndex: 'message', ellipsis: true },
    { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 90,
      render: (_, row) => (
        <ActionTag icon={<EyeOutlined />} color="default" onClick={() => setDeployDetail(row)}>
          详情
        </ActionTag>
      ),
    },
  ], []);

  const domainColumns = useMemo(() => [
    { title: '目标', dataIndex: 'targetName', width: 220, render: (_, row) => row.targetName || '-' },
    { title: '动作', dataIndex: 'action', width: 110, render: (_, row) => domainActionMap[row.action] || row.action || '-' },
    { title: 'DNS服务商', dataIndex: 'provider', width: 130 },
    { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
    { title: '说明', dataIndex: 'message', ellipsis: true },
    { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 90,
      render: (_, row) => (
        <ActionTag icon={<EyeOutlined />} color="default" onClick={() => setDomainDetail(row)}>
          详情
        </ActionTag>
      ),
    },
  ], []);

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
            key: 'domain',
            label: <TabLabel icon={<CloudServerOutlined />}>域名账号</TabLabel>,
            children: <LogTable resource="domain-account-logs" columns={domainColumns} />,
          },
          {
            key: 'deploy',
            label: <TabLabel icon={<DeploymentUnitOutlined />}>部署账号</TabLabel>,
            children: <LogTable resource="certificate-deploy-logs" columns={deployColumns} />,
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
      <Drawer
        title="证书日志详情"
        open={!!certificateDetail}
        width={760}
        onClose={() => setCertificateDetail(null)}
      >
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="证书域名">{certificateDetail?.commonName || '-'}</Descriptions.Item>
          <Descriptions.Item label="动作">{certificateActionMap[certificateDetail?.action] || certificateDetail?.action || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">{certificateDetail ? statusRender(null, certificateDetail) : '-'}</Descriptions.Item>
          <Descriptions.Item label="说明">{certificateDetail?.message || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求URL"><TextBlock value={certificateDetail?.requestUrl} /></Descriptions.Item>
          <Descriptions.Item label="请求方法">{certificateDetail?.requestMethod || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求头"><TextBlock value={certificateDetail?.requestHeaders} /></Descriptions.Item>
          <Descriptions.Item label="请求体"><TextBlock value={certificateDetail?.requestBody} /></Descriptions.Item>
          <Descriptions.Item label="响应体"><TextBlock value={certificateDetail?.responseBody} /></Descriptions.Item>
        </Descriptions>
      </Drawer>
      <Drawer
        title="部署账号日志详情"
        open={!!deployDetail}
        width={760}
        onClose={() => setDeployDetail(null)}
      >
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="证书域名">{deployDetail?.commonName || '-'}</Descriptions.Item>
          <Descriptions.Item label="部署服务">{deployDetail?.provider || '-'}</Descriptions.Item>
          <Descriptions.Item label="目标站点">{deployDetail?.targetName || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">{deployDetail ? statusRender(null, deployDetail) : '-'}</Descriptions.Item>
          <Descriptions.Item label="说明">{deployDetail?.message || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求URL"><TextBlock value={deployDetail?.requestUrl} /></Descriptions.Item>
          <Descriptions.Item label="请求方式">{deployDetail?.requestMethod || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求头"><TextBlock value={deployDetail?.requestHeaders} /></Descriptions.Item>
          <Descriptions.Item label="请求体"><TextBlock value={deployDetail?.requestBody} /></Descriptions.Item>
          <Descriptions.Item label="响应体"><TextBlock value={deployDetail?.responseBody} /></Descriptions.Item>
        </Descriptions>
      </Drawer>
      <Drawer
        title="域名账号日志详情"
        open={!!domainDetail}
        width={760}
        onClose={() => setDomainDetail(null)}
      >
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="目标">{domainDetail?.targetName || '-'}</Descriptions.Item>
          <Descriptions.Item label="动作">{domainActionMap[domainDetail?.action] || domainDetail?.action || '-'}</Descriptions.Item>
          <Descriptions.Item label="DNS服务商">{domainDetail?.provider || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">{domainDetail ? statusRender(null, domainDetail) : '-'}</Descriptions.Item>
          <Descriptions.Item label="说明">{domainDetail?.message || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求URL"><TextBlock value={domainDetail?.requestUrl} /></Descriptions.Item>
          <Descriptions.Item label="请求方式">{domainDetail?.requestMethod || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求头"><TextBlock value={domainDetail?.requestHeaders} /></Descriptions.Item>
          <Descriptions.Item label="请求体"><TextBlock value={domainDetail?.requestBody} /></Descriptions.Item>
          <Descriptions.Item label="响应体"><TextBlock value={domainDetail?.responseBody} /></Descriptions.Item>
        </Descriptions>
      </Drawer>
    </div>
  );
}
