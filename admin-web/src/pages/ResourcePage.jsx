import { DeleteOutlined, EditOutlined, ExperimentOutlined, PlusOutlined } from '@ant-design/icons';
import { DrawerForm, ModalForm, ProFormDependency, ProFormRadio, ProFormSelect, ProFormSwitch, ProFormText, ProFormTextArea, ProTable } from '@ant-design/pro-components';
import { Button, Card, Col, Popconfirm, Row, Space, Tag, message } from 'antd';
import { useMemo, useRef, useState } from 'react';
import { CloudProviderIcon, cloudProviders, providerNames } from '../components/CloudProviderIcon';
import { createResource, deleteResource, listResource, testDomainAccount, updateResource } from '../services/api';

const statusMap = {
  enabled: { color: 'green', text: '启用' },
  disabled: { color: 'default', text: '停用' },
  pending: { color: 'gold', text: '待处理' },
  issued: { color: 'blue', text: '已签发' },
  failed: { color: 'red', text: '失败' },
  expired: { color: 'red', text: '已过期' },
};

function statusRender(_, row) {
  const item = statusMap[row.status] || { color: 'default', text: row.status || '-' };
  return <Tag color={item.color}>{item.text}</Tag>;
}

const presets = {
  domains: [
    { title: '域名', dataIndex: 'name', formItemProps: { rules: [{ required: true }] } },
    { title: 'DNS服务商', dataIndex: 'dnsProvider' },
    { title: '记录数', dataIndex: 'recordCount', valueType: 'digit' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { enabled: '启用', disabled: '停用' }, render: statusRender },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
  ],
  certificates: [
    { title: '主域名', dataIndex: 'commonName', formItemProps: { rules: [{ required: true }] } },
    { title: '签发机构', dataIndex: 'issuer' },
    { title: '过期时间', dataIndex: 'expiresAt', valueType: 'dateTime' },
    { title: '提前续期天数', dataIndex: 'renewBeforeDay', valueType: 'digit' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { pending: '待申请', issued: '已签发', failed: '失败', expired: '已过期' }, render: statusRender },
  ],
  tasks: [
    { title: '任务名称', dataIndex: 'name', formItemProps: { rules: [{ required: true }] } },
    { title: '任务类型', dataIndex: 'taskType', valueType: 'select', valueEnum: { dns_sync: '域名同步', ssl_issue: '证书申请', ssl_deploy: '证书部署' } },
    { title: 'Cron表达式', dataIndex: 'cronExpr' },
    { title: '最近状态', dataIndex: 'lastRunStatus' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { enabled: '启用', disabled: '停用' }, render: statusRender },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
  ],
  domainAccounts: [
    { title: '账号名称', dataIndex: 'name', formItemProps: { rules: [{ required: true }] } },
    {
      title: 'DNS服务商',
      dataIndex: 'provider',
      valueType: 'select',
      valueEnum: { aliyun: '阿里云', dnspod: 'DNSPod', tencentcloud: '腾讯云', cloudflare: 'Cloudflare' },
      render: (_, row) => (
        <Space size={8}>
          <span className={`qdl-provider-icon qdl-provider-icon-inline qdl-provider-${row.provider}`}>
            <CloudProviderIcon provider={row.provider} />
          </span>
          <span>{providerNames[row.provider] || row.provider || '-'}</span>
        </Space>
      ),
    },
    { title: 'AccessKey', dataIndex: 'accessKey', ellipsis: true },
    { title: '最近检测', dataIndex: 'lastTestAt', valueType: 'dateTime' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { enabled: '启用', disabled: '停用' }, render: statusRender },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
  ],
  sslAccounts: [
    { title: '账号名称', dataIndex: 'name', formItemProps: { rules: [{ required: true }] } },
    { title: '证书服务', dataIndex: 'provider', valueType: 'select', valueEnum: { letsencrypt: "Let's Encrypt", zerossl: 'ZeroSSL', other: '其他' } },
    { title: '邮箱', dataIndex: 'email' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { enabled: '启用', disabled: '停用' }, render: statusRender },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
  ],
  logs: [
    { title: '用户', dataIndex: 'username' },
    { title: '动作', dataIndex: 'action' },
    { title: '资源', dataIndex: 'resource' },
    { title: '路径', dataIndex: 'path', ellipsis: true },
    { title: 'IP', dataIndex: 'ip' },
    { title: '状态', dataIndex: 'status' },
    { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime' },
  ],
  settings: [
    { title: '键名', dataIndex: 'settingKey', formItemProps: { rules: [{ required: true }] } },
    { title: '值', dataIndex: 'settingValue', ellipsis: true },
    { title: '类型', dataIndex: 'valueType', valueType: 'select', valueEnum: { string: '文本', number: '数字', boolean: '布尔', json: 'JSON' } },
    { title: '说明', dataIndex: 'description', ellipsis: true },
  ],
  users: [
    { title: '用户名', dataIndex: 'username', formItemProps: { rules: [{ required: true }] } },
    { title: '密码', dataIndex: 'password', hideInTable: true },
    { title: '昵称', dataIndex: 'nickname' },
    { title: '邮箱', dataIndex: 'email' },
    { title: '角色', dataIndex: 'role', valueType: 'select', valueEnum: { admin: '管理员', operator: '操作员' } },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { enabled: '启用', disabled: '停用' }, render: statusRender },
  ],
  notices: [
    { title: '名称', dataIndex: 'name', formItemProps: { rules: [{ required: true }] } },
    { title: '渠道', dataIndex: 'channel', valueType: 'select', valueEnum: { email: '邮件', webhook: 'Webhook', wechat: '企业微信' } },
    { title: '事件', dataIndex: 'events', ellipsis: true },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { enabled: '启用', disabled: '停用' }, render: statusRender },
  ],
};

function ActionTag({ icon, color = 'blue', children, onClick }) {
  return (
    <Tag className="qdl-action-tag" color={color} icon={icon} onClick={onClick}>
      {children}
    </Tag>
  );
}

function ProviderCardSelect() {
  return (
    <ProFormRadio.Group
      name="provider"
      label="DNS渠道商"
      rules={[{ required: true, message: '请选择DNS渠道商' }]}
      options={cloudProviders.map((provider) => ({
        value: provider.value,
        label: (
          <Card size="small" className="qdl-provider-card" bodyStyle={{ padding: 14 }}>
            <Space align="start">
              <span className={`qdl-provider-icon qdl-provider-${provider.value}`}>
                <CloudProviderIcon provider={provider.value} fallback={provider.fallback} />
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
        className: 'qdl-provider-radio',
        optionType: 'button',
      }}
    />
  );
}

function Field({ column }) {
  if (column.valueType === 'select') {
    return <ProFormSelect name={column.dataIndex} label={column.title} valueEnum={column.valueEnum} rules={column.formItemProps?.rules} />;
  }
  if (column.valueType === 'switch') {
    return <ProFormSwitch name={column.dataIndex} label={column.title} />;
  }
  if (column.dataIndex?.toLowerCase().includes('remark') || column.dataIndex === 'settingValue' || column.dataIndex === 'events') {
    return <ProFormTextArea name={column.dataIndex} label={column.title} />;
  }
  return <ProFormText name={column.dataIndex} label={column.title} rules={column.formItemProps?.rules} />;
}

function DomainAccountFormFields() {
  return (
    <>
      <ProFormText name="name" label="账号名称" rules={[{ required: true, message: '请输入账号名称' }]} />
      <ProviderCardSelect />
      <ProFormDependency name={['provider']}>
        {({ provider }) => (
          <>
            <ProFormText
              name="accessKey"
              label={provider === 'cloudflare' ? '账号标识（可选）' : 'AccessKey / SecretId'}
              rules={provider === 'cloudflare' ? [] : [{ required: true, message: '请输入AccessKey或SecretId' }]}
            />
            <ProFormText.Password
              name="secretKey"
              label={provider === 'cloudflare' ? 'API Token' : 'SecretKey / SecretAccessKey'}
              rules={[{ required: true, message: '请输入授权密钥' }]}
            />
          </>
        )}
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

export function ResourcePage({ title, resource, columnsPreset, readOnly = false }) {
  const actionRef = useRef();
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState(null);

  const columns = useMemo(() => {
    const base = presets[columnsPreset] || [];
    if (readOnly) return base;
    return [
      ...base,
      {
        title: '操作',
        valueType: 'option',
        width: resource === 'domain-accounts' ? 190 : 130,
        fixed: 'right',
        render: (_, row) => (
          <Space>
            {resource === 'domain-accounts' && (
              <ActionTag
                icon={<ExperimentOutlined />}
                color="processing"
                onClick={async () => {
                  const res = await testDomainAccount(row.id);
                  message.success(res.data?.message || '检测通过');
                  actionRef.current?.reload();
                }}
              >
                测试
              </ActionTag>
            )}
            <ActionTag icon={<EditOutlined />} onClick={() => { setCurrent(row); setOpen(true); }}>
              编辑
            </ActionTag>
            <Popconfirm title="确认删除？" onConfirm={async () => { await deleteResource(resource, row.id); message.success('已删除'); actionRef.current?.reload(); }}>
              <ActionTag icon={<DeleteOutlined />} color="error">
                删除
              </ActionTag>
            </Popconfirm>
          </Space>
        ),
      },
    ];
  }, [columnsPreset, readOnly, resource]);

  return (
    <>
      <ProTable
        key={resource}
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        request={async (params) => {
          const res = await listResource(resource, { page: params.current, pageSize: params.pageSize });
          return { data: res.data.items, total: res.data.total, success: true };
        }}
        toolBarRender={() => readOnly ? [] : [
          <Button key="new" type="primary" icon={<PlusOutlined />} onClick={() => { setCurrent(null); setOpen(true); }}>
            新建
          </Button>,
        ]}
        scroll={{ x: 'max-content' }}
        pagination={{ defaultPageSize: 10 }}
      />
      {resource === 'domain-accounts' ? (
        <DrawerForm
          title={`${current ? '编辑' : '新建'}${title}`}
          open={open}
          drawerProps={{ destroyOnClose: true, onClose: () => setOpen(false), width: 560 }}
          initialValues={current || { status: 'enabled', provider: 'aliyun' }}
          onFinish={async (values) => {
            if (current?.id) await updateResource(resource, current.id, { ...current, ...values });
            else await createResource(resource, values);
            message.success('保存成功');
            setOpen(false);
            actionRef.current?.reload();
            return true;
          }}
        >
          <DomainAccountFormFields />
        </DrawerForm>
      ) : (
        <ModalForm
          title={`${current ? '编辑' : '新建'}${title}`}
          open={open}
          modalProps={{ destroyOnClose: true, onCancel: () => setOpen(false) }}
          initialValues={current || { status: 'enabled' }}
          onFinish={async (values) => {
            if (current?.id) await updateResource(resource, current.id, { ...current, ...values });
            else await createResource(resource, values);
            message.success('保存成功');
            setOpen(false);
            actionRef.current?.reload();
            return true;
          }}
        >
          {(presets[columnsPreset] || []).filter((column) => column.valueType !== 'option').map((column) => (
            <Field key={column.dataIndex} column={column} />
          ))}
        </ModalForm>
      )}
    </>
  );
}
