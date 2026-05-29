import { CloudServerOutlined, LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { App, ConfigProvider } from 'antd';
import { useEffect, useState } from 'react';
import { api, setToken } from '../../services/api';

export function Login({ onSuccess }) {
  const [loginError, setLoginError] = useState('');

  useEffect(() => {
    if (!loginError) return undefined;
    const timer = window.setTimeout(() => setLoginError(''), 3600);
    return () => window.clearTimeout(timer);
  }, [loginError]);

  const handleFinish = async (values) => {
    try {
      const res = await api('/auth/login', { method: 'POST', body: JSON.stringify(values) });
      setToken(res.data.token);
      onSuccess(res.data.user);
      return true;
    } catch (error) {
      setLoginError(error.message || '登录失败，请检查账号或密码');
      return false;
    }
  };

  return (
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: '#0f6bff',
          borderRadius: 8,
          fontFamily:
            '"Avenir Next", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
        },
      }}
    >
      <App>
        <div className="login-page">
          {loginError ? (
            <div className="qdl-login-toast" role="alert" aria-live="assertive">
              <span className="qdl-login-toast-mark" />
              <span>{loginError}</span>
            </div>
          ) : null}

          <div className="qdl-login-shell">
            <section className="qdl-login-brand">
              <img className="qdl-login-logo" src="/logo.svg" alt="DaSSLm" />
              <div className="qdl-login-brand-copy">
                <p className="qdl-login-kicker">Qialas DNS Lab</p>
                <h1>
                  <span>DNS 与 SSL</span>
                  <span>自动化控制台</span>
                </h1>
                <p>
                  统一管理域名解析、证书签发、部署账号与任务日志，让证书生命周期更清晰。
                </p>
              </div>
            </section>

            <section className="qdl-login-panel">
              <div className="qdl-login-panel-center">
                <img className="qdl-login-mobile-logo" src="/logo.svg" alt="DaSSLm" />
                <div className="qdl-login-panel-head">
                  <div>
                    <p className="qdl-login-eyebrow">Admin Access</p>
                    <h2>欢迎回来</h2>
                  </div>
                  <div className="qdl-login-lock">
                    <SafetyCertificateOutlined />
                  </div>
                </div>

                <LoginForm
                  className="qdl-login-form"
                  logo={false}
                  title={false}
                  subTitle={false}
                  submitter={{ searchConfig: { submitText: '登录控制台' } }}
                  onFinish={handleFinish}
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

                <div className="qdl-login-panel-foot">
                  <span>
                    <CloudServerOutlined />
                    API / DNS / SSL
                  </span>
                  <span>Secure Gateway</span>
                </div>
              </div>
            </section>
          </div>
        </div>
      </App>
    </ConfigProvider>
  );
}
