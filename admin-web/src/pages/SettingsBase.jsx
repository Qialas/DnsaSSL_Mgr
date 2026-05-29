import {
  ApiOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  ExperimentOutlined,
  GlobalOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import {
  DrawerForm,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Col, Popconfirm, Row, Space, Tabs, Tag, message } from 'antd';
import { useMemo, useRef, useState } from 'react';
import { createResource, deleteResource, listResource, testProxySetting, updateResource } from '../services/api';

const settingTabs = [
  { key: 'site', label: '站点设置', icon: <GlobalOutlined /> },
  { key: 'proxy', label: '代理设置', icon: <ApiOutlined /> },
  { key: 'task', label: '任务设置', icon: <ClockCircleOutlined /> },
];

function EmptyPanel() {
  return <div className="qdl-settings-empty" />;
}

const protocolMeta = {
  http: { label: 'HTTP', color: 'blue' },
  https: { label: 'HTTPS', color: 'cyan' },
  sock4: { label: 'SOCK4', color: 'purple' },
  sock5: { label: 'SOCK5', color: 'geekblue' },
  sock5h: { label: 'SOCK5H', color: 'magenta' },
};

const statusMeta = {
  enabled: { label: '启用', color: 'green' },
  disabled: { label: '停用', color: 'default' },
};

function ActionTag({ icon, color = 'blue', children, onClick }) {
  return (
    <Tag className="qdl-action-tag" color={color} icon={icon} onClick={onClick}>
      {children}
    </Tag>
  );
}

function ProxyPoolPanel() {
  const actionRef = useRef();
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState(null);
  const [rows, setRows] = useState([]);

  const stats = useMemo(() => {
    const enabled = rows.filter((item) => item.status === 'enabled').length;
    const protocolCount = rows.reduce((acc, item) => {
      acc[item.protocol] = (acc[item.protocol] || 0) + 1;
      return acc;
    }, {});
    return [
      { label: '池内代理', value: rows.length, icon: <ApiOutlined /> },
      { label: '启用节点', value: enabled, icon: <CheckCircleOutlined /> },
      {
        label: '协议覆盖',
        value: ['http', 'https', 'sock4', 'sock5', 'sock5h'].filter((item) => protocolCount[item]).length,
        suffix: '/5',
        icon: <GlobalOutlined />,
      },
    ];
  }, [rows]);

  const columns = [
    {
      title: '代理名称',
      dataIndex: 'name',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <span className="qdl-proxy-name">{row.name}</span>
          <span className="qdl-proxy-address">{row.host}:{row.port}</span>
        </Space>
      ),
    },
    {
      title: '协议',
      dataIndex: 'protocol',
      width: 110,
      render: (_, row) => {
        const meta = protocolMeta[row.protocol] || { label: row.protocol || '-', color: 'default' };
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: '账号',
      dataIndex: 'username',
      width: 140,
      render: (_, row) => row.username || <span className="qdl-muted-text">无认证</span>,
    },
    {
      title: '权重',
      dataIndex: 'weight',
      width: 86,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 96,
      render: (_, row) => {
        const meta = statusMeta[row.status] || { label: row.status || '-', color: 'default' };
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: '最近检测',
      dataIndex: 'lastTestAt',
      width: 170,
      render: (_, row) => row.lastTestAt ? new Date(row.lastTestAt).toLocaleString() : <span className="qdl-muted-text">未检测</span>,
    },
    {
      title: '备注',
      dataIndex: 'remark',
      ellipsis: true,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 210,
      fixed: 'right',
      render: (_, row) => (
        <Space>
          <ActionTag
            icon={<ExperimentOutlined />}
            color="processing"
            onClick={async () => {
              const res = await testProxySetting(row.id);
              message.success(res.data?.message || '检测通过');
              actionRef.current?.reload();
            }}
          >
            测试
          </ActionTag>
          <ActionTag icon={<EditOutlined />} onClick={() => { setCurrent(row); setOpen(true); }}>
            编辑
          </ActionTag>
          <Popconfirm
            title="确认删除这个代理？"
            onConfirm={async () => {
              await deleteResource('proxy-settings', row.id);
              message.success('已删除');
              actionRef.current?.reload();
            }}
          >
            <ActionTag icon={<DeleteOutlined />} color="error">
              删除
            </ActionTag>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="qdl-proxy-pool">
      <div className="qdl-proxy-pool-head">
        <div>
          <h3>代理池</h3>
          <p>统一维护 HTTP、HTTPS、SOCK4、SOCK5、SOCK5H 代理节点</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCurrent(null); setOpen(true); }}>
          新增代理
        </Button>
      </div>
      <Row gutter={12} className="qdl-proxy-stats">
        {stats.map((item) => (
          <Col xs={24} md={8} key={item.label}>
            <div className="qdl-proxy-stat">
              <span className="qdl-proxy-stat-icon">{item.icon}</span>
              <span>
                <span className="qdl-proxy-stat-label">{item.label}</span>
                <strong>{item.value}<small>{item.suffix}</small></strong>
              </span>
            </div>
          </Col>
        ))}
      </Row>
      <ProTable
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        options={false}
        request={async (params) => {
          const res = await listResource('proxy-settings', { page: params.current, pageSize: params.pageSize });
          setRows(res.data.items || []);
          return { data: res.data.items, total: res.data.total, success: true };
        }}
        pagination={{ defaultPageSize: 10 }}
        scroll={{ x: 'max-content' }}
      />
      <DrawerForm
        title={`${current ? '编辑' : '新增'}代理`}
        open={open}
        drawerProps={{ destroyOnClose: true, onClose: () => setOpen(false), width: 520 }}
        initialValues={current || { protocol: 'http', status: 'enabled', weight: 1 }}
        onFinish={async (values) => {
          const payload = { ...current, ...values };
          if (current?.id) await updateResource('proxy-settings', current.id, payload);
          else await createResource('proxy-settings', payload);
          message.success('保存成功');
          setOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="name" label="代理名称" rules={[{ required: true, message: '请输入代理名称' }]} />
        <ProFormSelect
          name="protocol"
          label="代理协议"
          valueEnum={{ http: 'HTTP', https: 'HTTPS', sock4: 'SOCK4', sock5: 'SOCK5', sock5h: 'SOCK5H' }}
          rules={[{ required: true, message: '请选择代理协议' }]}
        />
        <Row gutter={12}>
          <Col xs={24} md={16}>
            <ProFormText name="host" label="代理主机" rules={[{ required: true, message: '请输入代理主机' }]} />
          </Col>
          <Col xs={24} md={8}>
            <ProFormDigit name="port" label="端口" min={1} max={65535} rules={[{ required: true, message: '请输入端口' }]} />
          </Col>
        </Row>
        <Row gutter={12}>
          <Col xs={24} md={12}>
            <ProFormText name="username" label="用户名" />
          </Col>
          <Col xs={24} md={12}>
            <ProFormText.Password name="password" label="密码" />
          </Col>
        </Row>
        <Row gutter={12}>
          <Col xs={24} md={12}>
            <ProFormDigit name="weight" label="权重" min={1} max={100} />
          </Col>
          <Col xs={24} md={12}>
            <ProFormSelect name="status" label="状态" valueEnum={{ enabled: '启用', disabled: '停用' }} />
          </Col>
        </Row>
        <ProFormTextArea name="remark" label="备注" />
      </DrawerForm>
    </div>
  );
}

export function SettingsBase() {
  return (
    <div className="qdl-settings-tabs">
      <Tabs
        tabPosition="left"
        defaultActiveKey="site"
        items={settingTabs.map((item) => ({
          key: item.key,
          label: (
            <Space size={6}>
              {item.icon}
              <span>{item.label}</span>
            </Space>
          ),
          children: item.key === 'proxy' ? <ProxyPoolPanel /> : <EmptyPanel />,
        }))}
      />
    </div>
  );
}
