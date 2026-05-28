import {
  ApiOutlined,
  DeleteOutlined,
  EditOutlined,
  FileProtectOutlined,
  SaveOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import {
  DrawerForm,
  ProFormDependency,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Card, Col, Drawer, Popconfirm, Row, Space, Tag, message } from 'antd';
import { useMemo, useRef, useState } from 'react';
import { CloudProviderIcon } from '../components/CloudProviderIcon';
import {
  createResource,
  deleteResource,
  importSSLAccountCertificates,
  listResource,
  listSSLAccountCertificates,
  updateResource,
} from '../services/api';

export const sslProviders = [
  {
    value: 'letsencrypt',
    label: "Let's Encrypt",
    desc: 'ACME 免费证书',
    directoryUrl: 'https://acme-v02.api.letsencrypt.org/directory',
    icon: <SafetyCertificateOutlined />,
  },
  {
    value: 'zerossl',
    label: 'ZeroSSL',
    desc: 'ACME / EAB',
    directoryUrl: 'https://acme.zerossl.com/v2/DV90',
    icon: <SafetyCertificateOutlined />,
  },
  {
    value: 'custom_acme',
    label: '自定义 ACME',
    desc: '兼容 ACME 目录',
    icon: <ApiOutlined />,
  },
  {
    value: 'tencent_free',
    label: '腾讯云免费证书',
    desc: 'Tencent Cloud SSL',
    iconProvider: 'tencentcloud',
  },
  {
    value: 'aliyun_free',
    label: '阿里云免费证书',
    desc: 'Alibaba Cloud SSL',
    iconProvider: 'aliyun',
  },
];

export const sslProviderNames = sslProviders.reduce((acc, item) => {
  acc[item.value] = item.label;
  return acc;
}, {});

const acmeDirectoryMap = sslProviders.reduce((acc, item) => {
  if (item.directoryUrl) acc[item.value] = item.directoryUrl;
  return acc;
}, {});

const statusMap = {
  enabled: { color: 'green', text: '启用' },
  disabled: { color: 'default', text: '停用' },
};

const certificateStatusMap = {
  0: { color: 'processing', text: '审核中' },
  1: { color: 'blue', text: '已通过' },
  2: { color: 'red', text: '审核失败' },
  3: { color: 'red', text: '已过期' },
  4: { color: 'cyan', text: '已加DNS' },
  6: { color: 'default', text: '取消中' },
  7: { color: 'default', text: '已取消' },
  8: { color: 'geekblue', text: '待确认' },
  9: { color: 'orange', text: '吊销中' },
  10: { color: 'default', text: '已吊销' },
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

function certificateStatusRender(_, row) {
  const code = row.status == null ? '' : String(row.status);
  const item = certificateStatusMap[code] || { color: 'default', text: row.statusName || row.statusMsg || code || '-' };
  return <Tag color={item.color}>{row.statusName || item.text}</Tag>;
}

function SSLProviderIcon({ provider }) {
  if (provider?.iconProvider) {
    return <CloudProviderIcon provider={provider.iconProvider} />;
  }
  return provider?.icon || <SafetyCertificateOutlined />;
}

function providerRender(_, row) {
  const provider = sslProviders.find((item) => item.value === row.provider);
  return (
    <Space size={8}>
      <span className={`qdl-ssl-provider-icon qdl-ssl-provider-${row.provider}`}>
        <SSLProviderIcon provider={provider} />
      </span>
      <span>{sslProviderNames[row.provider] || row.provider || '-'}</span>
    </Space>
  );
}

function SSLProviderCardSelect() {
  return (
    <ProFormRadio.Group
      name="provider"
      label="证书服务"
      rules={[{ required: true, message: '请选择证书服务' }]}
      options={sslProviders.map((provider) => ({
        value: provider.value,
        label: (
          <Card size="small" className="qdl-provider-card qdl-ssl-provider-card" bodyStyle={{ padding: 14 }}>
            <Space align="start">
              <span className={`qdl-ssl-provider-icon qdl-ssl-provider-${provider.value}`}>
                <SSLProviderIcon provider={provider} />
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

function SSLAccountFormFields() {
  return (
    <>
      <ProFormText name="name" label="账号名称" rules={[{ required: true, message: '请输入账号名称' }]} />
      <SSLProviderCardSelect />
      <ProFormDependency name={['provider']}>
        {({ provider }) => {
          const isCloudProvider = provider === 'tencent_free' || provider === 'aliyun_free';
          const isACMEProvider = ['letsencrypt', 'zerossl', 'custom_acme'].includes(provider);
          const needsContactEmail = isACMEProvider || provider === 'tencent_free';
          return (
            <>
              {needsContactEmail && (
                <ProFormText
                  name="email"
                  label={provider === 'tencent_free' ? '联系人邮箱' : '注册邮箱'}
                  rules={[{ required: true, message: provider === 'tencent_free' ? '请输入腾讯云证书联系人邮箱' : '请输入ACME注册邮箱' }]}
                />
              )}
              {provider === 'custom_acme' && (
                <ProFormText
                  name="directoryUrl"
                  label="ACME目录地址"
                  rules={[{ required: true, message: '请输入ACME目录地址' }]}
                  placeholder="https://acme.example.com/directory"
                />
              )}
              {(provider === 'zerossl' || provider === 'custom_acme') && (
                <Row gutter={12}>
                  <Col span={12}>
                    <ProFormText name="eabKid" label="EAB Key ID" />
                  </Col>
                  <Col span={12}>
                    <ProFormText.Password name="eabHmacKey" label="EAB HMAC Key" />
                  </Col>
                </Row>
              )}
              {isCloudProvider && (
                <>
                  <ProFormText
                    name="accessKey"
                    label={provider === 'tencent_free' ? 'SecretId' : 'AccessKey ID'}
                    rules={[{ required: true, message: '请输入云厂商AccessKey或SecretId' }]}
                  />
                  <ProFormText.Password
                    name="secretKey"
                    label={provider === 'tencent_free' ? 'SecretKey' : 'AccessKey Secret'}
                    rules={[{ required: true, message: '请输入云厂商SecretKey' }]}
                  />
                </>
              )}
            </>
          );
        }}
      </ProFormDependency>
      <Row gutter={12}>
        <Col span={12}>
          <ProFormSelect
            name="status"
            label="状态"
            valueEnum={{ enabled: '启用', disabled: '停用' }}
            rules={[{ required: true, message: '请选择状态' }]}
          />
        </Col>
      </Row>
      <ProFormTextArea name="remark" label="备注" />
    </>
  );
}

function normalizeSSLAccountPayload(values) {
  const payload = { ...values };
  if (payload.provider === 'letsencrypt' || payload.provider === 'zerossl') {
    payload.directoryUrl = acmeDirectoryMap[payload.provider];
  }
  if (payload.provider !== 'custom_acme') {
    payload.directoryUrl = acmeDirectoryMap[payload.provider] || '';
  }
  if (!['zerossl', 'custom_acme'].includes(payload.provider)) {
    payload.eabKid = '';
    payload.eabHmacKey = '';
  }
  if (!['letsencrypt', 'zerossl', 'custom_acme', 'tencent_free'].includes(payload.provider)) {
    payload.email = '';
  }
  if (!['tencent_free', 'aliyun_free'].includes(payload.provider)) {
    payload.accessKey = '';
    payload.secretKey = '';
  }
  return payload;
}

export function SSLAccountPage() {
  const actionRef = useRef();
  const certificateActionRef = useRef();
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState(null);
  const [certificateOpen, setCertificateOpen] = useState(false);
  const [certificateAccount, setCertificateAccount] = useState(null);

  const openCertificates = (row) => {
    setCertificateAccount(row);
    setCertificateOpen(true);
  };

  const columns = useMemo(() => [
    { title: '账号名称', dataIndex: 'name', width: 180 },
    { title: '证书服务', dataIndex: 'provider', width: 180, render: providerRender },
    { title: '邮箱', dataIndex: 'email', width: 200, render: (_, row) => row.email || '-' },
    { title: 'ACME目录', dataIndex: 'directoryUrl', ellipsis: true, render: (_, row) => row.directoryUrl || '-' },
    { title: '状态', dataIndex: 'status', width: 90, render: statusRender },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
    {
      title: '操作',
      valueType: 'option',
      width: 190,
      fixed: 'right',
      render: (_, row) => (
        <Space>
          {row.provider === 'tencent_free' && (
            <ActionTag icon={<FileProtectOutlined />} color="processing" onClick={() => openCertificates(row)}>
              证书
            </ActionTag>
          )}
          <ActionTag icon={<EditOutlined />} onClick={() => { setCurrent(row); setOpen(true); }}>
            编辑
          </ActionTag>
          <Popconfirm title="确认删除？" onConfirm={async () => { await deleteResource('ssl-accounts', row.id); message.success('已删除'); actionRef.current?.reload(); }}>
            <ActionTag icon={<DeleteOutlined />} color="error">
              删除
            </ActionTag>
          </Popconfirm>
        </Space>
      ),
    },
  ], []);

  const certificateColumns = useMemo(() => [
    { title: '域名', dataIndex: 'domain', width: 220 },
    { title: '证书ID', dataIndex: 'certificateId', width: 180, ellipsis: true },
    { title: '颁发者', dataIndex: 'productZhName', width: 160, render: (_, row) => row.productZhName || '-' },
    { title: '状态', dataIndex: 'status', width: 100, render: certificateStatusRender },
    { title: '验证方式', dataIndex: 'verifyType', width: 100, render: (_, row) => row.verifyType || '-' },
    { title: '过期时间', dataIndex: 'certEndTime', width: 170, render: (_, row) => row.certEndTime || '-' },
    { title: '备注', dataIndex: 'alias', ellipsis: true, render: (_, row) => row.alias || '-' },
  ], []);

  return (
    <>
      <ProTable
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        request={async (params) => {
          const res = await listResource('ssl-accounts', { page: params.current, pageSize: params.pageSize });
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
        title={`${current ? '编辑' : '新建'}SSL账号`}
        open={open}
        drawerProps={{ destroyOnClose: true, onClose: () => setOpen(false), width: 620 }}
        initialValues={current || { status: 'enabled', provider: 'letsencrypt' }}
        onFinish={async (values) => {
          const payload = normalizeSSLAccountPayload(values);
          if (current?.id) await updateResource('ssl-accounts', current.id, { ...current, ...payload });
          else await createResource('ssl-accounts', payload);
          message.success('保存成功');
          setOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <SSLAccountFormFields />
      </DrawerForm>
      <Drawer
        title={`${certificateAccount?.name || ''} 证书资源`}
        open={certificateOpen}
        width={920}
        onClose={() => setCertificateOpen(false)}
        extra={(
          <Button
            type="primary"
            icon={<SaveOutlined />}
            disabled={!certificateAccount?.id}
            onClick={async () => {
              const res = await importSSLAccountCertificates(certificateAccount.id);
              message.success(`已保存 ${res.data.saved || 0} 张证书`);
              certificateActionRef.current?.reload();
            }}
          >
            一键保存到本地
          </Button>
        )}
      >
        <ProTable
          rowKey="certificateId"
          actionRef={certificateActionRef}
          columns={certificateColumns}
          search={false}
          options={false}
          request={async (params) => {
            if (!certificateAccount?.id) return { data: [], total: 0, success: true };
            const res = await listSSLAccountCertificates(certificateAccount.id, { page: params.current, pageSize: params.pageSize });
            return { data: res.data.items, total: res.data.total, success: true };
          }}
          scroll={{ x: 'max-content' }}
          pagination={{ defaultPageSize: 10 }}
        />
      </Drawer>
    </>
  );
}
