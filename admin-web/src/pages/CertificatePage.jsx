import {
  CheckCircleOutlined,
  CopyOutlined,
  DeleteOutlined,
  DeploymentUnitOutlined,
  DownloadOutlined,
  EyeOutlined,
  FileSearchOutlined,
  HistoryOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  SendOutlined,
  StopOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import {
  DrawerForm,
  ProFormDependency,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Card, Descriptions, Divider, Drawer, Popconfirm, Space, Tag, Typography, message } from 'antd';
import JSZip from 'jszip';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { CloudProviderIcon } from '../components/CloudProviderIcon';
import {
  createResource,
  deleteResource,
  deployCertificate,
  getCertificateDetail,
  listResource,
  revokeCertificate,
  submitCertificate,
} from '../services/api';
import { deployProviderNames, deployProviders } from './DeployAccountPage';
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

const terminalCertificateStatuses = new Set(['issued', 'failed', 'expired', 'canceled', 'revoked']);

function ActionTag({ icon, color = 'blue', children, onClick, disabled = false }) {
  return (
    <Tag
      className={`qdl-action-tag${disabled ? ' qdl-action-tag-disabled' : ''}`}
      color={disabled ? 'default' : color}
      icon={icon}
      onClick={disabled ? undefined : onClick}
    >
      {children}
    </Tag>
  );
}

function statusRender(_, row) {
  const item = certificateStatusMap[row.status] || { color: 'default', text: row.status || '-' };
  return <Tag color={item.color}>{item.text}</Tag>;
}

function downloadBlob(filename, blob) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

async function copyText(value, title) {
  if (!value) return;
  await navigator.clipboard.writeText(value);
  message.success(`${title}已复制`);
}

async function downloadCertificateZip(baseName, certPEM, keyPEM) {
  const zip = new JSZip();
  if (certPEM) zip.file(`${baseName}.pem`, certPEM);
  if (keyPEM) zip.file(`${baseName}.key`, keyPEM);
  const blob = await zip.generateAsync({ type: 'blob' });
  downloadBlob(`${baseName}.zip`, blob);
}

function safeFilename(value) {
  return String(value || 'certificate').replace(/[\\/:*?"<>|\s]+/g, '_');
}

function formatDateTime(value) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString('zh-CN', { hour12: false });
}

function remainingDaysRender(_, row) {
  if (!row.expiresAt) return '-';
  const expiresAt = new Date(row.expiresAt);
  if (Number.isNaN(expiresAt.getTime())) return '-';
  const days = Math.ceil((expiresAt.getTime() - Date.now()) / 86400000);
  if (days < 0) {
    return <Tag color="red">已过期 {Math.abs(days)} 天</Tag>;
  }
  if (days === 0) {
    return <Tag color="red">今天到期</Tag>;
  }
  if (days <= 30) {
    const opacity = Math.max(0.12, Math.min(0.9, (31 - days) / 30));
    return (
      <Tag
        color="red"
        style={{
          backgroundColor: `rgba(255, 77, 79, ${opacity})`,
          borderColor: `rgba(255, 77, 79, ${Math.min(1, opacity + 0.18)})`,
          color: opacity > 0.45 ? '#fff' : '#cf1322',
        }}
      >
        剩余 {days} 天
      </Tag>
    );
  }
  return <Tag color="green">剩余 {days} 天</Tag>;
}

function serverCertificatePEM(detail) {
  return [detail?.certPem, detail?.chainPem].map((item) => String(item || '').trim()).filter(Boolean).join('\n');
}

function certificateDetailLabels(provider) {
  if (provider === 'tencent_free') {
    return {
      certificateId: '腾讯云证书ID',
      orderId: '腾讯云订单ID',
      status: '腾讯云状态',
    };
  }
  if (['letsencrypt', 'zerossl', 'custom_acme'].includes(provider)) {
    return {
      certificateId: '证书URL',
      orderId: 'ACME订单URL',
      status: 'ACME状态',
    };
  }
  return {
    certificateId: '服务商证书ID',
    orderId: '服务商订单ID',
    status: '服务商状态',
  };
}

function PemBlock({ title, value }) {
  if (!value) return null;
  return (
    <div className="qdl-certificate-pem-block">
      <div className="qdl-certificate-pem-title">
        <span>{title}</span>
        <Button
          size="small"
          icon={<CopyOutlined />}
          onClick={() => copyText(value, title)}
        >
          复制
        </Button>
      </div>
      <Typography.Paragraph copyable className="qdl-certificate-pem-text">
        {value}
      </Typography.Paragraph>
    </div>
  );
}

function LogTextBlock({ value }) {
  return (
    <Typography.Paragraph copyable={!!value} className="qdl-log-detail-text">
      {value || '-'}
    </Typography.Paragraph>
  );
}

function certFlowAction(row, verifying) {
  if (verifying) {
    return { text: '验证中', icon: <SyncOutlined spin />, color: 'processing', disabled: true };
  }
  if (row.status === 'issued') {
    return { text: '已签发', icon: <CheckCircleOutlined />, color: 'success', disabled: true };
  }
  if (row.status === 'submitted') {
    return { text: '验证', icon: <SyncOutlined />, color: 'processing', mode: 'verify' };
  }
  if (['dns_added', 'pending', 'applying'].includes(row.status)) {
    return { text: '验证', icon: <SyncOutlined />, color: 'processing', mode: 'submit' };
  }
  if (terminalCertificateStatuses.has(row.status)) {
    return { text: '提交', icon: <SendOutlined />, color: 'default', disabled: true };
  }
  return { text: '提交', icon: <SendOutlined />, color: 'processing', mode: 'submit' };
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

function DeployProviderCardSelect({ onChange }) {
  return (
    <ProFormRadio.Group
      name="deployProvider"
      label="部署服务"
      rules={[{ required: true, message: '请选择部署服务' }]}
      options={deployProviders.map((provider) => ({
        value: provider.value,
        label: (
          <Card size="small" className="qdl-provider-card qdl-ssl-provider-card" bodyStyle={{ padding: 14 }}>
            <Space align="start">
              <span className={`qdl-ssl-provider-icon qdl-ssl-provider-${provider.value}`}>
                <CloudProviderIcon provider={provider.iconProvider || provider.value} />
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
        onChange,
      }}
    />
  );
}

function sameDeployProvider(selectedProvider, accountProvider) {
  if (!selectedProvider || !accountProvider) return false;
  if (selectedProvider === accountProvider) return true;
  return selectedProvider === 'btpanel' && accountProvider === 'baota';
}

function deployProviderLabel(provider) {
  if (provider === 'baota') return deployProviderNames.btpanel || '宝塔面板';
  return deployProviderNames[provider] || provider;
}

function DeployAccountCardSelect({ accounts }) {
  if (!accounts.length) {
    return (
      <Card size="small" className="qdl-empty-account-card">
        当前部署服务下暂无启用账号
      </Card>
    );
  }
  return (
    <ProFormRadio.Group
      name="deployAccountId"
      label="部署账号"
      rules={[{ required: true, message: '请选择部署账号' }]}
      options={accounts.map((item) => ({
        value: item.id,
        label: (
          <Card size="small" className="qdl-provider-card qdl-deploy-account-card" bodyStyle={{ padding: 14 }}>
            <Space align="start">
              <span className={`qdl-ssl-provider-icon qdl-ssl-provider-${item.provider}`}>
                <CloudProviderIcon provider={item.provider} />
              </span>
              <span>
                <span className="qdl-provider-name">{item.name}</span>
                <span className="qdl-provider-desc">{deployProviderLabel(item.provider)}</span>
                <span className="qdl-provider-desc">{item.endpoint || '-'}</span>
              </span>
            </Space>
          </Card>
        ),
      }))}
      fieldProps={{
        className: 'qdl-provider-radio qdl-ssl-provider-radio qdl-deploy-account-radio',
        optionType: 'button',
      }}
    />
  );
}

export function CertificatePage() {
  const actionRef = useRef();
  const formRef = useRef();
  const deployFormRef = useRef();
  const pollTimerRef = useRef(null);
  const [open, setOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState(null);
  const [logOpen, setLogOpen] = useState(false);
  const [deployOpen, setDeployOpen] = useState(false);
  const [deployTarget, setDeployTarget] = useState(null);
  const [deployAccounts, setDeployAccounts] = useState([]);
  const [deployProvider, setDeployProvider] = useState();
  const [logCertificate, setLogCertificate] = useState(null);
  const [logDetail, setLogDetail] = useState(null);
  const [verifyingId, setVerifyingId] = useState(null);
  const [detailUpdating, setDetailUpdating] = useState(false);
  const [sslAccounts, setSSLAccounts] = useState([]);
  const [domains, setDomains] = useState([]);

  const stopVerifyPolling = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current);
      pollTimerRef.current = null;
    }
    setVerifyingId(null);
  }, []);

  useEffect(() => () => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current);
    }
  }, []);

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
  }, []);

  const updateDetail = useCallback(async () => {
    if (!detail?.id) return;
    setDetailUpdating(true);
    try {
      const res = await getCertificateDetail(detail.id, { refresh: 1 });
      setDetail(res.data);
      actionRef.current?.reload();
      message.success('证书信息已更新');
    } finally {
      setDetailUpdating(false);
    }
  }, [detail?.id]);

  const verifyCertificate = useCallback(async (row, { silent = false } = {}) => {
    const res = await getCertificateDetail(row.id, { refresh: 1 });
    const next = res.data;
    if (detailOpen && detail?.id === row.id) {
      setDetail(next);
    }
    actionRef.current?.reload();
    if (next.status === 'issued') {
      stopVerifyPolling();
      setDetail(next);
      setDetailOpen(true);
      message.success('证书已签发，证书内容已下载到本地');
      return next;
    }
    if (terminalCertificateStatuses.has(next.status)) {
      stopVerifyPolling();
      if (!silent) message.warning(next.providerStatusMsg || '证书已进入终止状态');
      return next;
    }
    if (!silent) message.info(next.providerStatusMsg || '暂未签发，已开始自动验证');
    return next;
  }, [detail?.id, detailOpen, stopVerifyPolling]);

  const startVerifyPolling = useCallback(async (row) => {
    stopVerifyPolling();
    setVerifyingId(row.id);
    const checked = await verifyCertificate(row);
    if (terminalCertificateStatuses.has(checked.status)) {
      return;
    }
    pollTimerRef.current = setInterval(() => {
      verifyCertificate(row, { silent: true }).catch((err) => {
        stopVerifyPolling();
        message.error(err.message || '验证失败');
      });
    }, 5000);
  }, [stopVerifyPolling, verifyCertificate]);

  const submitOrVerify = useCallback(async (row) => {
    if (row.status === 'submitted') {
      await startVerifyPolling(row);
      return;
    }
    await submitCertificate(row.id);
    message.success('已提交，开始自动验证');
    actionRef.current?.reload();
    await startVerifyPolling({ ...row, status: 'submitted' });
  }, [startVerifyPolling]);

  const openLogs = useCallback((row) => {
    setLogCertificate(row);
    setLogOpen(true);
  }, []);

  const openDeploy = useCallback(async (row) => {
    const res = await listResource('deploy-accounts', { page: 1, pageSize: 100 });
    setDeployAccounts((res.data.items || []).filter((item) => item.status !== 'disabled'));
    setDeployProvider(undefined);
    setDeployTarget(row);
    setDeployOpen(true);
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
    { title: '剩余时间', dataIndex: 'expiresAt', width: 130, render: remainingDaysRender },
    { title: '提前续期天数', dataIndex: 'renewBeforeDay', width: 120 },
    { title: '状态', dataIndex: 'status', width: 100, render: statusRender },
    {
      title: '操作',
      valueType: 'option',
      width: 360,
      fixed: 'right',
      render: (_, row) => {
        const flow = certFlowAction(row, verifyingId === row.id);
        return (
          <Space>
            <ActionTag
              icon={flow.icon}
              color={flow.color}
              disabled={flow.disabled}
              onClick={() => submitOrVerify(row)}
            >
              {flow.text}
            </ActionTag>
            <Popconfirm title="确认吊销？" onConfirm={async () => { await revokeCertificate(row.id); message.success('已提交吊销，后台将自动验证并检查吊销状态'); actionRef.current?.reload(); }}>
              <ActionTag icon={<StopOutlined />} color="warning">
                吊销
              </ActionTag>
            </Popconfirm>
            <Popconfirm title="确认删除？" onConfirm={async () => { await deleteResource('certificates', row.id); message.success('已删除'); actionRef.current?.reload(); }}>
              <ActionTag icon={<DeleteOutlined />} color="error">
                删除
              </ActionTag>
            </Popconfirm>
            {row.status === 'issued' && (
              <ActionTag icon={<DeploymentUnitOutlined />} color="processing" onClick={() => openDeploy(row)}>
                部署
              </ActionTag>
            )}
            <ActionTag icon={<HistoryOutlined />} color="default" onClick={() => openLogs(row)}>
              日志
            </ActionTag>
            <ActionTag icon={<EyeOutlined />} color="default" onClick={() => showDetail(row)}>
              详情
            </ActionTag>
          </Space>
        );
      },
    },
  ], [domainMap, openDeploy, openLogs, showDetail, sslAccountMap, submitOrVerify, verifyingId]);

  const logColumns = useMemo(() => [
    { title: '动作', dataIndex: 'action', width: 90, render: (_, row) => certificateActionMap[row.action] || row.action || '-' },
    { title: '状态', dataIndex: 'status', width: 90, render: (_, row) => <Tag color={row.status === 'success' ? 'green' : 'red'}>{row.status === 'success' ? '成功' : '失败'}</Tag> },
    { title: '说明', dataIndex: 'message', ellipsis: true },
    { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
    {
      title: '操作',
      valueType: 'option',
      fixed: 'right',
      width: 90,
      render: (_, row) => (
        <ActionTag icon={<FileSearchOutlined />} color="default" onClick={() => setLogDetail(row)}>
          详情
        </ActionTag>
      ),
    },
  ], []);

  const initialValues = { status: 'pending', renewBeforeDay: 30 };
  const detailBaseName = safeFilename(detail?.commonName);
  const detailServerCert = serverCertificatePEM(detail);
  const detailAccount = sslAccountMap[detail?.sslAccountId];
  const detailLabels = certificateDetailLabels(detailAccount?.provider);

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
      <DrawerForm
        formRef={deployFormRef}
        title="部署SSL证书"
        open={deployOpen}
        drawerProps={{
          destroyOnClose: true,
          onClose: () => {
            setDeployOpen(false);
            setDeployProvider(undefined);
          },
          width: 640,
        }}
        initialValues={{
          siteName: deployTarget?.commonName,
        }}
        onFinish={async (values) => {
          if (!values.deployAccountId) {
            message.warning('请选择部署账号');
            return false;
          }
          await deployCertificate(deployTarget.id, {
            deployAccountId: Number(values.deployAccountId),
            siteName: values.siteName,
          });
          message.success('部署成功');
          setDeployOpen(false);
          return true;
        }}
      >
        <DeployProviderCardSelect
          onChange={(event) => {
            const next = event.target.value;
            setDeployProvider(next);
            deployFormRef.current?.setFieldsValue({ deployAccountId: undefined });
          }}
        />
        <ProFormDependency name={['deployProvider']}>
          {({ deployProvider: selectedProvider }) => {
            const provider = selectedProvider || deployProvider;
            if (!provider) return null;
            const accounts = deployAccounts.filter((item) => sameDeployProvider(provider, item.provider));
            return <DeployAccountCardSelect accounts={accounts} />;
          }}
        </ProFormDependency>
        <ProFormText
          name="siteName"
          label="站点名称"
          placeholder={deployTarget?.commonName}
          rules={[{ required: true, message: '请输入宝塔站点名称' }]}
        />
      </DrawerForm>
      <Drawer
        title="SSL证书详情"
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        width={760}
        extra={(
          <Button icon={<SyncOutlined />} loading={detailUpdating} onClick={updateDetail}>
            更新
          </Button>
        )}
      >
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="主域名">{detail?.commonName || '-'}</Descriptions.Item>
          <Descriptions.Item label="备用域名">{formatSANs(detail?.sans) || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">{detail ? statusRender(null, detail) : '-'}</Descriptions.Item>
          <Descriptions.Item label={detailLabels.certificateId}>{detail?.providerCertificateId || '-'}</Descriptions.Item>
          <Descriptions.Item label={detailLabels.orderId}>{detail?.providerOrderId || '-'}</Descriptions.Item>
          <Descriptions.Item label={detailLabels.status}>{detail?.providerStatusMsg || detail?.providerStatus || '-'}</Descriptions.Item>
          <Descriptions.Item label="验证方式">{detail?.verifyType || '-'}</Descriptions.Item>
          <Descriptions.Item label="验证记录">{detail?.authRecordName || '-'}</Descriptions.Item>
          <Descriptions.Item label="验证值">
            <Typography.Text copyable={!!detail?.authRecordValue}>{detail?.authRecordValue || '-'}</Typography.Text>
          </Descriptions.Item>
          <Descriptions.Item label="签发机构">{detail?.issuer || '-'}</Descriptions.Item>
          <Descriptions.Item label="过期时间">{formatDateTime(detail?.expiresAt)}</Descriptions.Item>
        </Descriptions>
        {(detailServerCert || detail?.keyPem) ? (
          <>
            <Divider orientation="left">服务器配置证书</Divider>
            <div className="qdl-certificate-deploy-grid">
              <PemBlock title="证书文件" value={detailServerCert} />
              <PemBlock title="私钥文件" value={detail?.keyPem} />
            </div>
            <div className="qdl-certificate-download-footer">
              <Button
                icon={<DownloadOutlined />}
                onClick={() => downloadCertificateZip(detailBaseName, detailServerCert, detail?.keyPem)}
              >
                打包下载
              </Button>
            </div>
          </>
        ) : null}
      </Drawer>
      <Drawer
        title={`${logCertificate?.commonName || ''} 证书日志`}
        open={logOpen}
        onClose={() => setLogOpen(false)}
        width={860}
      >
        <ProTable
          key={logCertificate?.id || 'certificate-logs'}
          rowKey="id"
          columns={logColumns}
          search={false}
          options={false}
          request={async (params) => {
            if (!logCertificate?.id) return { data: [], total: 0, success: true };
            const res = await listResource('certificate-logs', {
              page: params.current,
              pageSize: params.pageSize,
              certificateId: logCertificate.id,
            });
            return { data: res.data.items, total: res.data.total, success: true };
          }}
          scroll={{ x: 'max-content' }}
          pagination={{ defaultPageSize: 10 }}
        />
      </Drawer>
      <Drawer
        title="证书日志详情"
        open={!!logDetail}
        onClose={() => setLogDetail(null)}
        width={760}
      >
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="证书域名">{logDetail?.commonName || '-'}</Descriptions.Item>
          <Descriptions.Item label="动作">{certificateActionMap[logDetail?.action] || logDetail?.action || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">
            {logDetail ? <Tag color={logDetail.status === 'success' ? 'green' : 'red'}>{logDetail.status === 'success' ? '成功' : '失败'}</Tag> : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="说明">{logDetail?.message || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求URL"><LogTextBlock value={logDetail?.requestUrl} /></Descriptions.Item>
          <Descriptions.Item label="请求方法">{logDetail?.requestMethod || '-'}</Descriptions.Item>
          <Descriptions.Item label="请求头"><LogTextBlock value={logDetail?.requestHeaders} /></Descriptions.Item>
          <Descriptions.Item label="请求体"><LogTextBlock value={logDetail?.requestBody} /></Descriptions.Item>
          <Descriptions.Item label="响应体"><LogTextBlock value={logDetail?.responseBody} /></Descriptions.Item>
        </Descriptions>
      </Drawer>
    </>
  );
}
