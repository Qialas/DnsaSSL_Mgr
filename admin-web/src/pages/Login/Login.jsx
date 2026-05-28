import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { App, ConfigProvider } from 'antd';
import { api, setToken } from '../../services/api';

export function Login({ onSuccess }) {
  return (
    <ConfigProvider theme={{ token: { colorPrimary: '#1677ff' } }}>
      <App>
        <div className="login-page">
          <LoginForm
            logo={<img className="qdl-login-logo" src="/logo.svg" alt="DaSSLm" />}
            title=""
            subTitle="DNS 管理与 SSL 自动化平台"
            onFinish={async (values) => {
              const res = await api('/auth/login', { method: 'POST', body: JSON.stringify(values) });
              setToken(res.data.token);
              onSuccess(res.data.user);
              return true;
            }}
          >
            <ProFormText
              name="username"
              fieldProps={{ size: 'large', prefix: <UserOutlined /> }}
              placeholder="用户名"
              rules={[{ required: true, message: '请输入用户名' }]}
            />
            <ProFormText.Password
              name="password"
              fieldProps={{ size: 'large', prefix: <LockOutlined /> }}
              placeholder="密码"
              rules={[{ required: true, message: '请输入密码' }]}
            />
          </LoginForm>
        </div>
      </App>
    </ConfigProvider>
  );
}
