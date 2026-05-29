import {
  ApiOutlined,
  BugOutlined,
  DashboardOutlined,
  FieldTimeOutlined,
  GithubOutlined,
  GlobalOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  TagsOutlined,
} from '@ant-design/icons';
import { PageLoading } from '@ant-design/pro-components';
import { Button, Card, Col, Progress, Row, Space, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { api } from '../../services/api';

const { Text } = Typography;

const projectLinks = [
  {
    title: '开源地址',
    detail: 'GitHub Repository',
    url: 'https://github.com/Qialas/DnsaSSL_Mgr',
    icon: <GithubOutlined />,
    accent: '#111827',
  },
  {
    title: 'Issues 提交',
    detail: 'Bug / Feature Request',
    url: 'https://github.com/Qialas/DnsaSSL_Mgr/issues',
    icon: <BugOutlined />,
    accent: '#dc2626',
  },
  {
    title: 'Release 地址',
    detail: '版本发布与下载',
    url: 'https://github.com/Qialas/DnsaSSL_Mgr/releases',
    icon: <TagsOutlined />,
    accent: '#2563eb',
  },
];

function clampPercent(value) {
  return Math.min(Math.max(Number(value || 0), 0), 100);
}

function usageTone(value) {
  const percent = clampPercent(value);
  if (percent >= 85) return { color: '#ef4444', text: '压力较高' };
  if (percent >= 65) return { color: '#f59e0b', text: '需要关注' };
  return { color: '#22c55e', text: '运行平稳' };
}

function formatLoad(value) {
  return Number(value || 0).toFixed(2);
}

function formatTime(value) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleString('zh-CN', { hour12: false });
}

function GaugeItem({ label, percent, primary, secondary }) {
  const value = Math.round(clampPercent(percent));
  const tone = usageTone(value);

  return (
    <div className="qdl-dashboard-gauge-item">
      <Progress
        type="circle"
        percent={value}
        size={116}
        strokeWidth={8}
        strokeColor={tone.color}
        trailColor="#eef1f5"
        format={(num) => <span className="qdl-dashboard-gauge-percent">{num}%</span>}
      />
      <div className="qdl-dashboard-gauge-primary">{primary}</div>
      <div className="qdl-dashboard-gauge-secondary">{secondary}</div>
      <div className="qdl-dashboard-gauge-label">{label}</div>
    </div>
  );
}

function ServerOverview({ server }) {
  const cpu = server.cpu || {};
  const memory = server.memory || {};
  const disk = server.disk || {};
  const loadPercent = server.loadAverage ? Math.min((server.loadAverage / Math.max(cpu.total || 1, 1)) * 100, 100) : 0;

  return (
    <Card className="qdl-dashboard-panel qdl-dashboard-server" bordered={false}>
      <div className="qdl-dashboard-panel-head">
        <div>
          <div className="qdl-dashboard-panel-title">服务器状态</div>
          <Text type="secondary">负载、计算与存储占用</Text>
        </div>
        <span className="qdl-dashboard-live-pill">Live</span>
      </div>
      <Row gutter={[24, 20]} align="middle">
        <Col xs={12} md={6}>
          <GaugeItem label="负载" percent={loadPercent} primary="运行平稳" secondary={`1分钟 ${formatLoad(server.loadAverage)}`} />
        </Col>
        <Col xs={12} md={6}>
          <GaugeItem label="CPU" percent={cpu.percent} primary={`${cpu.total || 0} 核心`} secondary="实时采样" />
        </Col>
        <Col xs={12} md={6}>
          <GaugeItem label="内存" percent={memory.percent} primary={memory.detailText || '暂无数据'} secondary="系统内存" />
        </Col>
        <Col xs={12} md={6}>
          <GaugeItem label="储存" percent={disk.percent} primary={disk.detailText || '暂无数据'} secondary="根分区" />
        </Col>
      </Row>
    </Card>
  );
}

function AssetCard({ icon, title, value, detail, accent }) {
  return (
    <Card className="qdl-dashboard-asset" bordered={false} style={{ '--accent': accent }}>
      <div className="qdl-dashboard-asset-top">
        <span className="qdl-dashboard-asset-icon">{icon}</span>
        <span className="qdl-dashboard-asset-mark" />
      </div>
      <Text type="secondary">{title}</Text>
      <div className="qdl-dashboard-asset-value">{value}</div>
      <Text type="secondary">{detail}</Text>
    </Card>
  );
}

function ProjectLinkItem({ icon, title, detail, url, accent }) {
  return (
    <div
      className="qdl-dashboard-link-item"
      style={{ '--accent': accent }}
      onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}
    >
      <div className="qdl-dashboard-link-main">
        <span className="qdl-dashboard-asset-icon">{icon}</span>
        <div>
          <div className="qdl-dashboard-link-title">{title}</div>
          <Text type="secondary">{detail}</Text>
        </div>
      </div>
      <Typography.Text copyable={{ text: url }} className="qdl-dashboard-link-url">
        {url}
      </Typography.Text>
    </div>
  );
}

function ProjectLinksPanel() {
  return (
    <Card className="qdl-dashboard-panel qdl-dashboard-links" bordered={false}>
      <div className="qdl-dashboard-panel-head">
        <div>
          <div className="qdl-dashboard-panel-title">项目地址</div>
          <Text type="secondary">开源仓库、问题反馈与版本发布</Text>
        </div>
      </div>
      <Row gutter={[14, 14]}>
        {projectLinks.map((item) => (
          <Col key={item.title} xs={24} md={8}>
            <ProjectLinkItem {...item} />
          </Col>
        ))}
      </Row>
    </Card>
  );
}

export function Dashboard() {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const res = await api('/dashboard/overview');
      setData(res.data);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  if (!data) return <PageLoading />;

  return (
    <Space direction="vertical" size={16} className="qdl-dashboard">
      <Card className="qdl-dashboard-summary" bordered={false}>
        <Space align="center" size={12}>
          <span className="qdl-dashboard-summary-icon"><DashboardOutlined /></span>
          <div>
            <div className="qdl-dashboard-title">运行概览</div>
            <Text type="secondary">DNS、SSL 证书与自动化任务状态</Text>
          </div>
        </Space>
        <Space>
          <Text type="secondary">更新于 {formatTime(data.serverUpdatedAt)}</Text>
          <Button type="primary" icon={<ReloadOutlined />} loading={loading} onClick={loadData}>刷新</Button>
        </Space>
      </Card>

      <ServerOverview server={data.server || {}} />

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} xl={6}>
          <AssetCard icon={<GlobalOutlined />} title="域名数量" value={data.domains || 0} detail="已接入管理的域名" accent="#2563eb" />
        </Col>
        <Col xs={24} md={12} xl={6}>
          <AssetCard icon={<SafetyCertificateOutlined />} title="证书数量" value={data.certificates || 0} detail="证书资产总量" accent="#16a34a" />
        </Col>
        <Col xs={24} md={12} xl={6}>
          <AssetCard icon={<FieldTimeOutlined />} title="自动任务数量" value={data.tasks || 0} detail="自动化执行配置" accent="#7c3aed" />
        </Col>
        <Col xs={24} md={12} xl={6}>
          <AssetCard icon={<ApiOutlined />} title="7天内到期证书" value={data.expiringCerts7d || 0} detail="建议优先检查续签" accent="#f59e0b" />
        </Col>
      </Row>

      <ProjectLinksPanel />
    </Space>
  );
}
