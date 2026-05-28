import {
  DeleteOutlined,
  EyeOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  SendOutlined,
  StopOutlined,
} from '@ant-design/icons';
import {
  DrawerForm,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Descriptions, Drawer, Popconfirm, Space, Tag, Typography, message } from 'antd';
import { useCallback, useMemo, useRef, useState } from 'react';
import {
  createResource,
  deleteResource,
  getCertificateDetail,
  listResource,
  revokeCertificate,
  submitCertificate,
} from '../services/api';
import { sslProviderNames } from './SSLAccountPage';

const certificateStatusMap = {
  pending: { color: 'gold', text: '待申请' },
  applying: { color: 'processing', text: '申请中' },
  dns_added: { color: 'cyan', text: '已加验证' },
  submitted: { color: 'geekblue', text: '已提交' },
  issued: { color: 'blue', text: '已签发' },
  failed: { color: 'red', text: '失败' },
  expired: { color: 'red', text: '已过期' },
  canceled: { color: 'default', text: '已取消' },
  revoked: { color: 'default', text: '已吊销' },
  revoking: { color: 'orange', text: '吊销中' },
};

function ActionTag({ icon, color = 'blue', children, onClick }) {
  return (
    <Tag className="qdl-action-tag" color={color} icon={icon} onClick={onClick}>
      {children}
    </Tag>
  );
}

function statusRender(_, row) {
  const item = certificateStatusMap[row.status] || { color: 'default', text: row.status || '-' };
  return <Tag color={item.color}>{item.text}</Tag>;
}

function formatSANs(value) {
  if (!value) return '';
  if (Array.isArray(value)) return value.join('\n');
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed.join('\n') : String(value);
  } catch {
    return String(value).replaceAll(',', '\n');
  }
}

function normalizeSANs(value) {
  const items = String(value || '')
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
  return JSON.stringify(Array.from(new Set(items)));
}

function normalizeCertificate(values, domainMap) {
  const payload = {
    ...values,
    domainId: Number(values.domainId),
    sslAccountId: Number(values.sslAccountId),
    renewBeforeDay: Number(values.renewBeforeDay || 30),
    sans: normalizeSANs(values.sans),
  };
  if (!payload.commonName) {
    payload.commonName = domainMap[payload.domainId]?.name || '';
  }
  return payload;
}

export function CertificatePage() {
  const actionRef = useRef();
  const formRef = useRef();
  const [open, setOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState(null);
  const [sslAccounts, setSSLAccounts] = useState([]);
  const [domains, setDomains] = useState([]);

  const sslAccountMap = useMemo(() => sslAccounts.reduce((acc, item) => {
    acc[item.id] = item;
    return acc;
  }, {}), [sslAccounts]);

  const domainMap = useMemo(() => domains.reduce((acc, item) => {
    acc[item.id] = item;
    return acc;
  }, {}), [domains]);

  const loadOptions = useCallback(async () => {
    const [sslRes, domainRes] = await Promise.all([
      listResource('ssl-accounts', { page: 1, pageSize: 100 }),
      listResource('domains', { page: 1, pageSize: 100 }),
    ]);
    setSSLAccounts(sslRes.data.items || []);
    setDomains(domainRes.data.items || []);
  }, []);

  const openForm = useCallback(async () => {
    await loadOptions();
    setOpen(true);
  }, [loadOptions]);

  const showDetail = useCallback(async (row) => {
    const res = await getCertificateDetail(row.id);
    setDetail(res.data);
    setDetailOpen(true);
    actionRef.current?.reload();
  }, []);

  const columns = useMemo(() => [
    { title: '主域名', dataIndex: 'commonName', width: 220 },
    {
      title: 'SSL账号',
      dataIndex: 'sslAccountId',
      width: 190,
      render: (_, row) => {
        const account = sslAccountMap[row.sslAccountId];
        return account ? `${account.name} / ${sslProviderNames[account.provider] || account.provider}` : '-';
      },
    },
    {
      title: '关联域名',
      dataIndex: 'domainId',
      width: 180,
      render: (_, row) => domainMap[row.domainId]?.name || row.commonName || '-',
    },
    { title: '签发机构', dataIndex: 'issuer', width: 150, render: (_, row) => row.issuer || '-' },
    { title: '过期时间', dataIndex: 'expiresAt', valueType: 'dateTime', width: 180 },
    { title: '提前续期天数', dataIndex: 'renewBeforeDay', width: 120 },
    { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
    {
      title: '操作',
      valueType: 'option',
      width: 230,
      fixed: 'right',
      render: (_, row) => (
        <Space>
          <ActionTag
            icon={<SendOutlined />}
            color="processing"
            onClick={async () => {
              await submitCertificate(row.id);
              message.success('已提交');
              actionRef.current?.reload();
            }}
          >
            提交
          </ActionTag>
          <Popconfirm title="确认吊销？" onConfirm={async () => { await revokeCertificate(row.id); message.success('已提交吊销'); actionRef.current?.reload(); }}>
            <ActionTag icon={<StopOutlined />} color="warning">
              吊销
            </ActionTag>
          </Popconfirm>
          <Popconfirm title="确认删除？" onConfirm={async () => { await deleteResource('certificates', row.id); message.success('已删除'); actionRef.current?.reload(); }}>
            <ActionTag icon={<DeleteOutlined />} color="error">
              删除
            </ActionTag>
          </Popconfirm>
          <ActionTag icon={<EyeOutlined />} color="default" onClick={() => showDetail(row)}>
            详情
          </ActionTag>
        </Space>
      ),
    },
  ], [domainMap, showDetail, sslAccountMap]);

  const initialValues = { status: 'pending', renewBeforeDay: 30 };

  return (
    <>
      <ProTable
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        request={async (params) => {
          const [res, sslRes, domainRes] = await Promise.all([
            listResource('certificates', { page: params.current, pageSize: params.pageSize }),
            listResource('ssl-accounts', { page: 1, pageSize: 100 }),
            listResource('domains', { page: 1, pageSize: 100 }),
          ]);
          setSSLAccounts(sslRes.data.items || []);
          setDomains(domainRes.data.items || []);
          return { data: res.data.items, total: res.data.total, success: true };
        }}
        toolBarRender={() => [
          <Button key="new" type="primary" icon={<PlusOutlined />} onClick={() => openForm()}>
            新建
          </Button>,
        ]}
        scroll={{ x: 'max-content' }}
        pagination={{ defaultPageSize: 10 }}
      />
      <DrawerForm
        formRef={formRef}
        title="新建SSL证书"
        open={open}
        drawerProps={{ destroyOnClose: true, onClose: () => setOpen(false), width: 620 }}
        initialValues={initialValues}
        onFinish={async (values) => {
          const payload = normalizeCertificate(values, domainMap);
          await createResource('certificates', payload);
          message.success('已进入申请流程');
          setOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormSelect
          name="sslAccountId"
          label="SSL账号"
          rules={[{ required: true, message: '请选择SSL账号' }]}
          options={sslAccounts
            .filter((item) => item.status !== 'disabled')
            .map((item) => ({
              value: item.id,
              label: `${item.name} / ${sslProviderNames[item.provider] || item.provider}`,
            }))}
        />
        <ProFormSelect
          name="domainId"
          label="域名"
          rules={[{ required: true, message: '请选择域名' }]}
          options={domains.map((item) => ({ value: item.id, label: item.name }))}
          fieldProps={{
            onChange: (value) => {
              if (domainMap[value]?.name && !formRef.current?.getFieldValue('commonName')) {
                formRef.current?.setFieldsValue({ commonName: domainMap[value].name });
              }
            },
          }}
        />
        <ProFormText
          name="commonName"
          label="主域名"
          fieldProps={{ prefix: <SafetyCertificateOutlined /> }}
          rules={[{ required: true, message: '请输入主域名' }]}
        />
        <ProFormTextArea name="sans" label="备用域名" fieldProps={{ rows: 4 }} />
        <ProFormDigit
          name="renewBeforeDay"
          label="提前续期天数"
          min={1}
          max={90}
          rules={[{ required: true, message: '请输入提前续期天数' }]}
        />
      </DrawerForm>
      <Drawer
        title="SSL证书详情"
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        width={640}
      >
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="主域名">{detail?.commonName || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">{detail ? statusRender(null, detail) : '-'}</Descriptions.Item>
          <Descriptions.Item label="腾讯云证书ID">{detail?.providerCertificateId || '-'}</Descriptions.Item>
          <Descriptions.Item label="腾讯云订单ID">{detail?.providerOrderId || '-'}</Descriptions.Item>
          <Descriptions.Item label="腾讯云状态">{detail?.providerStatusMsg || detail?.providerStatus || '-'}</Descriptions.Item>
          <Descriptions.Item label="验证方式">{detail?.verifyType || '-'}</Descriptions.Item>
          <Descriptions.Item label="验证记录">{detail?.authRecordName || '-'}</Descriptions.Item>
          <Descriptions.Item label="验证值">
            <Typography.Text copyable={!!detail?.authRecordValue}>{detail?.authRecordValue || '-'}</Typography.Text>
          </Descriptions.Item>
          <Descriptions.Item label="签发机构">{detail?.issuer || '-'}</Descriptions.Item>
          <Descriptions.Item label="过期时间">{detail?.expiresAt || '-'}</Descriptions.Item>
          <Descriptions.Item label="备用域名">{formatSANs(detail?.sans) || '-'}</Descriptions.Item>
        </Descriptions>
      </Drawer>
    </>
  );
}
