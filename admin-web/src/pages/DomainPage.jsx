import { ArrowLeftOutlined, DeleteOutlined, EditOutlined, EyeOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea, ProTable } from '@ant-design/pro-components';
import { Button, Drawer, Form, Input, Popconfirm, Select, Space, Tag, Tooltip, message } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { CloudProviderIcon, providerNames } from '../components/CloudProviderIcon';
import {
  createDomainRecord,
  createResource,
  deleteDomainRecord,
  deleteResource,
  getResource,
  listDomainRecordLines,
  listDomainRecords,
  listProviderDomains,
  listResource,
  refreshDomainExpires,
  updateDomainRecord,
} from '../services/api';

function ActionTag({ icon, color = 'blue', children, onClick }) {
  return (
    <Tag className="qdl-action-tag" color={color} icon={icon} onClick={onClick}>
      {children}
    </Tag>
  );
}

function statusRender(_, row) {
  const text = row.status === 'disabled' ? '停用' : '启用';
  return <Tag color={row.status === 'disabled' ? 'default' : 'green'}>{text}</Tag>;
}

function recordStatusRender(_, row) {
  const text = row.status === 'disabled' ? '暂停' : '启用';
  return <Tag color={row.status === 'disabled' ? 'default' : 'green'}>{text}</Tag>;
}

function formatDateTime(value) {
  if (!value) return '未查询';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '未查询';
  return date.toLocaleString('zh-CN', { hour12: false });
}

function goPath(navigate, path) {
  if (navigate) {
    navigate(path);
    return;
  }
  window.history.pushState(null, '', path);
}

export function DomainPage({ navigate }) {
  const actionRef = useRef();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [accounts, setAccounts] = useState([]);
  const [providerDomains, setProviderDomains] = useState([]);
  const [selectedAccount, setSelectedAccount] = useState();
  const [form] = Form.useForm();

  const accountOptions = useMemo(() => accounts.map((item) => ({
    label: (
      <Space size={8}>
        <span className={`qdl-provider-icon qdl-provider-icon-inline qdl-provider-${item.provider}`}>
          <CloudProviderIcon provider={item.provider} />
        </span>
        <span>{item.name}（{providerNames[item.provider] || item.provider}）</span>
      </Space>
    ),
    value: item.id,
  })), [accounts]);

  const openRecordsPage = (row) => goPath(navigate, `/domains/${row.id}`);

  const loadAccounts = async () => {
    const res = await listResource('domain-accounts', { page: 1, pageSize: 100 });
    setAccounts(res.data.items || []);
    return res.data.items || [];
  };

  const loadProviderDomains = async (accountId) => {
    setSelectedAccount(accountId);
    form.setFieldsValue({ domainNames: [] });
    if (!accountId) {
      setProviderDomains([]);
      return;
    }
    const res = await listProviderDomains(accountId);
    setProviderDomains((res.data.items || []).map((item) => ({
      ...item,
      disabled: Boolean(res.data.selected?.[item.name]),
    })));
  };

  const openCreateDrawer = async () => {
    form.resetFields();
    setProviderDomains([]);
    setSelectedAccount(undefined);
    await loadAccounts();
    setDrawerOpen(true);
  };

  const domainColumns = [
    {
      title: '域名',
      dataIndex: 'name',
      width: 220,
      render: (_, row) => (
        <Space size={8}>
          <span className={`qdl-provider-icon-plain qdl-provider-plain-${row.dnsProvider}`}>
            <CloudProviderIcon provider={row.dnsProvider} />
          </span>
          <Button type="link" className="qdl-domain-link" onClick={() => openRecordsPage(row)}>
            {row.name}
          </Button>
        </Space>
      ),
    },
    { title: '域名账号', dataIndex: 'domainAccountId', width: 110 },
    { title: '记录数', dataIndex: 'recordCount', width: 90 },
    {
      title: '到期时间',
      dataIndex: 'expiresAt',
      width: 230,
      render: (_, row) => (
        <Space size={8}>
          <span>{formatDateTime(row.expiresAt)}</span>
          <Tooltip title="刷新到期时间">
            <Button
              type="text"
              size="small"
              className="qdl-icon-button"
              icon={<ReloadOutlined />}
              onClick={async () => {
                await refreshDomainExpires(row.id);
                message.success('到期时间已刷新');
                actionRef.current?.reload();
              }}
            />
          </Tooltip>
        </Space>
      ),
    },
    { title: '状态', dataIndex: 'status', width: 90, render: statusRender },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
    {
      title: '操作',
      valueType: 'option',
      width: 170,
      fixed: 'right',
      render: (_, row) => (
        <Space>
          <ActionTag icon={<EyeOutlined />} color="processing" onClick={() => openRecordsPage(row)}>
            记录
          </ActionTag>
          <Popconfirm title="确认删除？" onConfirm={async () => { await deleteResource('domains', row.id); message.success('已删除'); actionRef.current?.reload(); }}>
            <ActionTag icon={<DeleteOutlined />} color="error">
              删除
            </ActionTag>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      <ProTable
        rowKey="id"
        actionRef={actionRef}
        columns={domainColumns}
        search={false}
        request={async (params) => {
          const res = await listResource('domains', { page: params.current, pageSize: params.pageSize });
          return { data: res.data.items, total: res.data.total, success: true };
        }}
        toolBarRender={() => [
          <Button key="new" type="primary" icon={<PlusOutlined />} onClick={openCreateDrawer}>
            新建
          </Button>,
        ]}
        scroll={{ x: 'max-content' }}
        pagination={{ defaultPageSize: 10 }}
      />

      <Drawer
        title="新建域名"
        open={drawerOpen}
        width={560}
        destroyOnClose
        onClose={() => setDrawerOpen(false)}
        extra={<Button type="primary" onClick={() => form.submit()}>保存</Button>}
      >
        <Form
          layout="vertical"
          form={form}
          initialValues={{ status: 'enabled' }}
          onFinish={async (values) => {
            await createResource('domains', values);
            message.success('保存成功');
            setDrawerOpen(false);
            actionRef.current?.reload();
          }}
        >
          <Form.Item name="domainAccountId" label="域名账号" rules={[{ required: true, message: '请选择域名账号' }]}>
            <Select options={accountOptions} placeholder="请选择域名账号" onChange={loadProviderDomains} />
          </Form.Item>
          <Form.Item name="domainNames" label="账号下域名" rules={[{ required: true, message: '请选择域名' }]}>
            <Select
              mode="multiple"
              placeholder={selectedAccount ? '请选择账号下的域名' : '请先选择域名账号'}
              options={providerDomains.map((item) => ({
                label: `${item.name}${item.disabled ? '（已添加）' : ''}`,
                value: item.name,
                disabled: item.disabled,
              }))}
              optionFilterProp="label"
            />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true, message: '请选择状态' }]}>
            <Select options={[{ label: '启用', value: 'enabled' }, { label: '停用', value: 'disabled' }]} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={4} />
          </Form.Item>
        </Form>
      </Drawer>
    </>
  );
}

export function DomainRecordsPage({ domainId, navigate }) {
  const recordActionRef = useRef();
  const [domain, setDomain] = useState(null);
  const [recordLines, setRecordLines] = useState([{ label: '默认', value: '默认' }]);
  const [recordFormOpen, setRecordFormOpen] = useState(false);
  const [currentRecord, setCurrentRecord] = useState(null);
  const isCloudflareDomain = domain?.dnsProvider === 'cloudflare';
  const isTencentDomain = ['tencentcloud', 'dnspod'].includes(domain?.dnsProvider);

  useEffect(() => {
    getResource('domains', domainId).then((res) => setDomain(res.data));
  }, [domainId]);

  useEffect(() => {
    if (!domain?.id || !isTencentDomain) {
      setRecordLines([{ label: '默认', value: '默认' }]);
      return;
    }
    listDomainRecordLines(domain.id)
      .then((res) => {
        const options = (res.data.items || []).map((item) => ({
          label: item.lineId ? `${item.name}（${item.lineId}）` : item.name,
          value: item.name,
        }));
        setRecordLines(options.length ? options : [{ label: '默认', value: '默认' }]);
      })
      .catch((err) => {
        message.warning(err.message || '解析线路获取失败，已使用默认线路');
        setRecordLines([{ label: '默认', value: '默认' }]);
      });
  }, [domain?.id, isTencentDomain]);

  const recordInitialValues = {
    rr: '@',
    type: 'A',
    line: isCloudflareDomain ? undefined : (isTencentDomain ? '默认' : 'default'),
    ttl: 600,
    status: 'enabled',
    ...(currentRecord || {}),
  };
  if (isTencentDomain && !recordInitialValues.line) {
    recordInitialValues.line = '默认';
  }
  const recordTypeValueEnum = isTencentDomain
    ? { A: 'A', AAAA: 'AAAA', CNAME: 'CNAME', MX: 'MX', TXT: 'TXT', CAA: 'CAA' }
    : { A: 'A', AAAA: 'AAAA', CNAME: 'CNAME', MX: 'MX', TXT: 'TXT', NS: 'NS', CAA: 'CAA' };

  const recordColumns = [
    { title: '主机记录', dataIndex: 'rr', width: 120 },
    { title: '类型', dataIndex: 'type', width: 90 },
    { title: '记录值', dataIndex: 'value', ellipsis: true },
    { title: '线路', dataIndex: 'line', width: 110, render: (_, row) => row.line || '-' },
    { title: 'TTL', dataIndex: 'ttl', width: 90 },
    { title: '优先级', dataIndex: 'priority', width: 90 },
    { title: '状态', dataIndex: 'status', width: 90, render: recordStatusRender },
    {
      title: '操作',
      valueType: 'option',
      width: 140,
      fixed: 'right',
      render: (_, row) => (
        <Space>
          <ActionTag icon={<EditOutlined />} onClick={() => { setCurrentRecord(row); setRecordFormOpen(true); }}>
            编辑
          </ActionTag>
          <Popconfirm title="确认删除？" onConfirm={async () => { await deleteDomainRecord(domainId, row.id); message.success('已删除'); recordActionRef.current?.reload(); }}>
            <ActionTag icon={<DeleteOutlined />} color="error">
              删除
            </ActionTag>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      <div className="qdl-subpage-toolbar">
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => goPath(navigate, '/domains')}>返回</Button>
          <span className={`qdl-provider-icon-plain qdl-provider-plain-${domain?.dnsProvider || ''}`}>
            <CloudProviderIcon provider={domain?.dnsProvider} />
          </span>
          <span className="qdl-subpage-title">{domain?.name || 'DNS记录'}</span>
        </Space>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => recordActionRef.current?.reload()}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCurrentRecord(null); setRecordFormOpen(true); }}>新建记录</Button>
        </Space>
      </div>

      <ProTable
        rowKey="id"
        actionRef={recordActionRef}
        columns={recordColumns}
        search={false}
        options={false}
        request={async () => {
          const res = await listDomainRecords(domainId);
          return { data: res.data.items, total: res.data.total, success: true };
        }}
        scroll={{ x: 'max-content' }}
        pagination={{ defaultPageSize: 10 }}
      />

      <ModalForm
        title={`${currentRecord ? '编辑' : '新建'}DNS记录`}
        open={recordFormOpen}
        modalProps={{ destroyOnClose: true, onCancel: () => setRecordFormOpen(false) }}
        initialValues={recordInitialValues}
        onFinish={async (values) => {
          const payload = { ...values };
          if (isCloudflareDomain) delete payload.line;
          if (currentRecord?.id) await updateDomainRecord(domainId, currentRecord.id, payload);
          else await createDomainRecord(domainId, payload);
          message.success('保存成功');
          setRecordFormOpen(false);
          recordActionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="rr" label="主机记录" rules={[{ required: true, message: '请输入主机记录' }]} />
        <ProFormSelect name="type" label="记录类型" rules={[{ required: true, message: '请选择记录类型' }]} valueEnum={recordTypeValueEnum} />
        <ProFormText name="value" label="记录值" rules={[{ required: true, message: '请输入记录值' }]} />
        {isTencentDomain && (
          <ProFormSelect
            name="line"
            label="解析线路"
            options={recordLines}
            rules={[{ required: true, message: '请选择解析线路' }]}
          />
        )}
        {!isCloudflareDomain && !isTencentDomain && <ProFormText name="line" label="解析线路" />}
        <ProFormDigit name="ttl" label="TTL" min={1} fieldProps={{ precision: 0 }} />
        <ProFormDigit name="priority" label="优先级" min={0} fieldProps={{ precision: 0 }} />
        <ProFormSelect name="status" label="状态" valueEnum={{ enabled: '启用', disabled: '暂停' }} />
        <ProFormTextArea name="remark" label="备注" />
      </ModalForm>
    </>
  );
}
