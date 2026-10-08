import React, { useState } from 'react';
import { Lock, User, KeyRound, ShieldCheck, AlertCircle } from 'lucide-react';

export default function Login({ onLoginSuccess }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!username.trim() || !password.trim()) {
      setErrorMsg('请输入账号和密码');
      return;
    }

    setLoading(true);
    setErrorMsg('');

    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username.trim(), password: password.trim() }),
      });

      const data = await res.json();
      if (res.ok && data.code === 0) {
        localStorage.setItem('admin_token', data.token);
        localStorage.setItem('admin_username', data.username);
        localStorage.setItem('admin_role', data.role || 'USER');
        localStorage.setItem('admin_user_id', data.userId);
        localStorage.setItem('admin_perm_predict_payback', data.permPredictPayback || 0);
        localStorage.setItem('admin_perm_roi_predict', data.permRoiPredict || 0);
        localStorage.setItem('admin_perm_global_distribution', data.permGlobalDistribution || 0);
        localStorage.setItem('admin_perm_export', data.permExport || 0);
        localStorage.setItem('admin_perm_settlement', data.permSettlement || 0);
        localStorage.setItem('admin_perm_video_gen', data.permVideoGen || 0);
        onLoginSuccess(data);
      } else {
        setErrorMsg(data.msg || '登录失败，请检查账号密码');
      }
    } catch (err) {
      setErrorMsg('网络请求失败，请检查后端服务是否已启动');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-overlay">
      <div className="login-card">
        <div className="login-header">
          <div className="login-brand-icon">
            <ShieldCheck size={28} color="#ffffff" />
          </div>
          <h2 className="login-title">DataLens</h2>
          <p className="login-subtitle">全域 LTV 与投流分析中台</p>
        </div>

        <form onSubmit={handleSubmit} className="login-form">
          {errorMsg && (
            <div className="login-error-alert">
              <AlertCircle size={16} color="#f43f5e" />
              <span>{errorMsg}</span>
            </div>
          )}

          <div className="form-group">
            <label className="form-label">登录账号</label>
            <div className="input-with-icon">
              <User size={18} className="input-icon" />
              <input
                type="text"
                className="form-input"
                placeholder="请输入账号"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoFocus
              />
            </div>
          </div>

          <div className="form-group">
            <label className="form-label">登录密码</label>
            <div className="input-with-icon">
              <KeyRound size={18} className="input-icon" />
              <input
                type="password"
                className="form-input"
                placeholder="请输入密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
          </div>

          <button type="submit" className="btn btn-primary login-btn" disabled={loading}>
            {loading ? '正在登录...' : '安全登录'}
          </button>
        </form>

        <div className="login-footer">
          <Lock size={12} style={{ marginRight: 4 }} /> Token 有效期 3 天，到期需重新登录
        </div>
      </div>
    </div>
  );
}
