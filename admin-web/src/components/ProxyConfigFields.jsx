import { ProFormDependency, ProFormSelect, ProFormSwitch } from '@ant-design/pro-components';
import { listResource } from '../services/api';

const proxyModeOptions = {
  manual: '指定代理',
  auto: '自动轮询',
};

function proxyLabel(row) {
  const protocol = String(row.protocol || '').toUpperCase();
  return `${row.name} · ${protocol} · ${row.host}:${row.port}`;
}

export function ProxyConfigFields() {
  return (
    <>
      <ProFormSwitch name="useProxy" label="使用代理" />
      <ProFormDependency name={['useProxy', 'proxyMode']}>
        {({ useProxy, proxyMode }) => {
          if (!useProxy) return null;
          return (
            <>
              <ProFormSelect
                name="proxyMode"
                label="代理方式"
                valueEnum={proxyModeOptions}
                rules={[{ required: true, message: '请选择代理方式' }]}
              />
              {proxyMode !== 'auto' && (
                <ProFormSelect
                  name="proxyId"
                  label="代理节点"
                  placeholder="从代理池选择"
                  rules={[{ required: true, message: '请选择代理节点' }]}
                  request={async () => {
                    const res = await listResource('proxy-settings', { page: 1, pageSize: 100 });
                    return (res.data.items || [])
                      .filter((item) => item.status === 'enabled')
                      .map((item) => ({ label: proxyLabel(item), value: item.id }));
                  }}
                />
              )}
            </>
          );
        }}
      </ProFormDependency>
    </>
  );
}
