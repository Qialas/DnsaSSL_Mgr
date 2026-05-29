import { DeleteOutlined, EditOutlined, ExperimentOutlined, PlusOutlined, UnorderedListOutlined } from '@ant-design/icons';
import {
  DrawerForm,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Card, Drawer, Popconfirm, Space, Tag, message } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { CloudProviderIcon, providerNames } from '../components/CloudProviderIcon';
import { ProxyConfigFields } from '../components/ProxyConfigFields';
import { createResource, deleteResource, listDeployAccountSites, listResource, testDeployAccount, updateResource } from '../services/api';

export const deployProviders = [
  { value: 'btpanel', label: '宝塔面板', desc: 'BT Panel Website SSL', iconProvider: 'btpanel' },
];

export const deployProviderNames = deployProviders.reduce((acc, item) => {
  acc[item.value] = item.label;
  return acc;
}, {});

const statusMap = {
  enabled: { color: 'green', text: '启用' },
  disabled: { color: 'default', text: '停用' },
};

function ActionTag({ icon, color = 'blue', children, onClick }) {
  return (
    <Tag className="qdl-action-tag" color={color} icon={icon} onClick={onClick}>
      {children}
    </Tag>
  );
}

function statusRender(_, row) {
  const item = statusMap[row.status] || { color: 'default', text: row.status || '-' };
  return <Tag color={item.color}>{item.text}</Tag>;
}

function proxyRender(_, row) {
  if (!row.useProxy) return <Tag>直连</Tag>;
  return <Tag color="processing">{row.proxyMode === 'auto' ? '自动轮询' : '指定代理'}</Tag>;
}

function providerRender(_, row) {
  return (
    <Space size={8}>
      <span className={`qdl-ssl-provider-icon qdl-ssl-provider-${row.provider}`}>
        <CloudProviderIcon provider={row.provider} />
      </span>
      <span>{providerNames[row.provider] || row.provider || '-'}</span>
    </Space>
  );
}

function DeployProviderCardSelect() {
  return (
    <ProFormRadio.Group
      name="provider"
      label="部署服务"
      rules={[{ required: true, message: '请选择部署服务' }]}
      options={deployProviders.map((provider) => ({
        value: provider.value,
        label: (
          <Card size="small" className="qdl-provider-card qdl-ssl-provider-card" bodyStyle={{ padding: 14 }}>
            <Space align="start">
              <span className={`qdl-ssl-provider-icon qdl-ssl-provider-${provider.value}`}>
                <CloudProviderIcon provider={provider.iconProvider} />
              </span>
              <span>
                <span className="qdl-provider-name">{provider.label}</span>
                <span className="qdl-provider-desc">{provider.desc}</span>
              </span>
            </Space>
          </Card>
        ),
      }))}
      fieldProps={{
        className: 'qdl-provider-radio qdl-ssl-provider-radio',
        optionType: 'button',
      }}
    />
  );
}

function normalizeDeployAccount(values) {
  return {
    ...values,
    endpoint: String(values.endpoint || '').replace(/\/+$/, ''),
  };
}

export function DeployAccountPage() {
  const actionRef = useRef();
  const siteActionRef = useRef();
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState(null);
  const [siteOpen, setSiteOpen] = useState(false);
  const [siteAccount, setSiteAccount] = useState(null);

  const openSites = (row) => {
    setSiteAccount(row);
    setSiteOpen(true);
  };

  useEffect(() => {
    if (siteOpen && siteAccount?.id) {
      siteActionRef.current?.reloadAndRest?.();
    }
  }, [siteOpen, siteAccount?.id]);

  const columns = useMemo(() => [
    { title: '账号名称', dataIndex: 'name', width: 180 },
    { title: '部署服务', dataIndex: 'provider', width: 150, render: providerRender },
    { title: '面板地址', dataIndex: 'endpoint', ellipsis: true },
    { title: '代理', dataIndex: 'useProxy', width: 100, render: proxyRender },
    { title: '状态', dataIndex: 'status', width: 90, render: statusRender },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
    {
      title: '操作',
      valueType: 'option',
      width: 250,
      fixed: 'right',
      render: (_, row) => (
        <Space>
          {['btpanel', 'baota'].includes(row.provider) && (
            <ActionTag
              icon={<ExperimentOutlined />}
              color="processing"
              onClick={async () => {
                try {
                  const res = await testDeployAccount(row.id);
                  message.success(res.data.message || '检测通过');
                } catch (error) {
                  message.error(error.message || '检测失败');
                }
              }}
            >
              检测
            </ActionTag>
          )}
          {['btpanel', 'baota'].includes(row.provider) && (
            <ActionTag icon={<UnorderedListOutlined />} color="processing" onClick={() => openSites(row)}>
              站点
            </ActionTag>
          )}
          <ActionTag icon={<EditOutlined />} onClick={() => { setCurrent(row); setOpen(true); }}>
            编辑
          </ActionTag>
          <Popconfirm title="确认删除？" onConfirm={async () => { await deleteResource('deploy-accounts', row.id); message.success('已删除'); actionRef.current?.reload(); }}>
            <ActionTag icon={<DeleteOutlined />} color="error">
              删除
            </ActionTag>
          </Popconfirm>
        </Space>
      ),
    },
  ], []);

  const siteColumns = useMemo(() => [
    { title: '站点名称', dataIndex: 'name', width: 220 },
    { title: '根目录', dataIndex: 'path', ellipsis: true, render: (_, row) => row.path || '-' },
    { title: '状态', dataIndex: 'status', width: 100, render: (_, row) => row.status || '-' },
    { title: '备注', dataIndex: 'ps', ellipsis: true, render: (_, row) => row.ps || '-' },
    { title: '到期时间', dataIndex: 'expire', width: 140, render: (_, row) => row.expire || '-' },
    { title: '创建时间', dataIndex: 'addTime', width: 170, render: (_, row) => row.addTime || '-' },
  ], []);

  return (
    <>
      <ProTable
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        request={async (params) => {
          const res = await listResource('deploy-accounts', { page: params.current, pageSize: params.pageSize });
          return { data: res.data.items, total: res.data.total, success: true };
        }}
        toolBarRender={() => [
          <Button key="new" type="primary" icon={<PlusOutlined />} onClick={() => { setCurrent(null); setOpen(true); }}>
            新建
          </Button>,
        ]}
        scroll={{ x: 'max-content' }}
        pagination={{ defaultPageSize: 10 }}
      />
      <DrawerForm
        title={`${current ? '编辑' : '新建'}部署账号`}
        open={open}
        drawerProps={{ destroyOnClose: true, onClose: () => setOpen(false), width: 620 }}
        initialValues={current || { status: 'enabled', provider: 'btpanel', useProxy: false, proxyMode: 'manual' }}
        onFinish={async (values) => {
          const payload = normalizeDeployAccount(values);
          if (current?.id) await updateResource('deploy-accounts', current.id, { ...current, ...payload });
          else await createResource('deploy-accounts', payload);
          message.success('保存成功');
          setOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="name" label="账号名称" rules={[{ required: true, message: '请输入账号名称' }]} />
        <DeployProviderCardSelect />
        <ProFormText
          name="endpoint"
          label="面板地址"
          placeholder="https://panel.example.com:8888"
          rules={[{ required: true, message: '请输入宝塔面板地址' }]}
        />
        <ProFormText.Password
          name="accessKey"
          label="API密钥"
          rules={[{ required: true, message: '请输入宝塔面板API密钥' }]}
        />
        <ProFormSelect
          name="status"
          label="状态"
          valueEnum={{ enabled: '启用', disabled: '停用' }}
          rules={[{ required: true, message: '请选择状态' }]}
        />
        <ProxyConfigFields />
        <ProFormTextArea name="remark" label="备注" />
      </DrawerForm>
      <Drawer
        title={`${siteAccount?.name || ''} 站点列表`}
        open={siteOpen}
        width={920}
        destroyOnClose
        onClose={() => {
          setSiteOpen(false);
          setSiteAccount(null);
        }}
      >
        <ProTable
          key={siteAccount?.id || 'btpanel-sites'}
          rowKey={(row) => row.id || row.name}
          actionRef={siteActionRef}
          columns={siteColumns}
          search={false}
          options={false}
          request={async (params) => {
            if (!siteAccount?.id) return { data: [], total: 0, success: true };
            const res = await listDeployAccountSites(siteAccount.id, {
              page: params.current,
              pageSize: params.pageSize,
            });
            return { data: res.data.items, total: res.data.total, success: true };
          }}
          scroll={{ x: 'max-content' }}
          pagination={{ defaultPageSize: 10 }}
        />
      </Drawer>
    </>
  );
}
