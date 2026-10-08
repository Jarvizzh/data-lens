import React, { useState } from 'react';
import { createPortal } from 'react-dom';
import { BarChart3, PieChart, Settings, RefreshCw, Upload, Users, LogOut, Shield, Eye, Globe, Download, Receipt, Video, Menu, X } from 'lucide-react';
import CustomSelect from './CustomSelect';

export default function LtvHeader({
  activeTab,
  onTabChange,
  selectedPlatform,
  platformsList,
  onSelectPlatform,
  onOpenConfig,
  onOpenTokenModal,
  onOpenSyncModal,
  onOpenBatchSpend,
  onOpenUserManagement,
  onOpenExportModal,
  currentUser,
  usersList,
  targetUserId,
  onSelectTargetUser,
  onLogout,
  loading
}) {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  const isSuperAdmin = currentUser && currentUser.role === 'SUPER_ADMIN';
  const hasPermGlobalDistribution = isSuperAdmin || currentUser?.permGlobalDistribution === 1;
  const hasPermExport = isSuperAdmin || currentUser?.permExport === 1;
  const hasPermSettlement = isSuperAdmin || currentUser?.permSettlement === 1;
  const hasPermVideoGen = isSuperAdmin || currentUser?.permVideoGen === 1;
  const isReadOnly = Boolean(targetUserId && currentUser && targetUserId !== currentUser.userId);
  const canSwitchView = (isSuperAdmin || (usersList && usersList.length > 1)) && usersList && usersList.length > 0;

  const handleAction = (callback) => {
    setIsMobileMenuOpen(false);
    if (typeof callback === 'function') {
      callback();
    }
  };

  return (
    <header className="app-header">
      <div className="header-left-group">
        <div className="header-brand">
          <div className="brand-icon">
            <BarChart3 size={22} />
          </div>
          <div>
            <h1 className="brand-title">DataLens</h1>
          </div>
        </div>

        {/* 核心导航栏 Tab 切换 */}
        <nav className="header-nav-tabs">
          <button
            className={`nav-tab-btn ${activeTab === 'ltv' ? 'active' : ''}`}
            onClick={() => onTabChange('ltv')}
            title="LTV 报表"
          >
            <BarChart3 size={16} />
            <span>LTV</span>
          </button>
          <button
            className={`nav-tab-btn ${activeTab === 'distribution' ? 'active' : ''}`}
            onClick={() => onTabChange('distribution')}
            title="充值分析"
          >
            <PieChart size={16} />
            <span>充值分析</span>
          </button>
          {hasPermGlobalDistribution && (
            <button
              className={`nav-tab-btn ${activeTab === 'global-distribution' ? 'active' : ''}`}
              onClick={() => onTabChange('global-distribution')}
              title="平台所有订单充值汇总（不区分落地页，全量数据）"
            >
              <Globe size={16} />
              <span>平台汇总</span>
            </button>
          )}
          {hasPermSettlement && (
            <button
              className={`nav-tab-btn ${activeTab === 'settlement' ? 'active' : ''}`}
              onClick={() => onTabChange('settlement')}
              title="结算列表（平台汇总、账号分配结算、无关联落地页结算）"
            >
              <Receipt size={16} />
              <span>结算</span>
            </button>
          )}
        </nav>
      </div>

      {/* 桌面端平铺操作区 */}
      <div className="header-actions desktop-actions">
        {/* 平台切换下拉框 */}
        {platformsList && platformsList.length > 0 && (
          <div
            title="选择数据源平台（默认综合大盘）"
            style={{
              position: 'relative',
              zIndex: 1002,
              display: 'flex',
              alignItems: 'center',
              gap: '0.4rem',
              background: 'rgba(21, 30, 50, 0.94)',
              border: '1px solid rgba(16, 185, 129, 0.45)',
              borderRadius: '0.48rem',
              padding: '0.25rem 0.55rem'
            }}
          >
            <Globe size={15} color="#10b981" />
            <span style={{ fontSize: '0.78rem', color: '#10b981', fontWeight: 600, whiteSpace: 'nowrap' }}>
              平台
            </span>
            <CustomSelect
              value={selectedPlatform || 'rocnovel'}
              onChange={(val) => onSelectPlatform && onSelectPlatform(val)}
              options={platformsList.map((p) => ({
                label: p.name || p.code,
                value: p.code
              }))}
              className="custom-select-sm"
              style={{ minWidth: '130px' }}
            />
          </div>
        )}

        {/* 账户视图切换下拉框 (包含被分配只读视图或超级管理员可见) */}
        {canSwitchView && (
          <div
            title={isReadOnly ? '只读模式：您正在查看其他被授权账户的数据视图' : '主视图：您正在查看当前登录账户的数据'}
            style={{
              position: 'relative',
              zIndex: 1001,
              display: 'flex',
              alignItems: 'center',
              gap: '0.4rem',
              background: isReadOnly ? 'rgba(38, 20, 30, 0.94)' : 'rgba(21, 30, 50, 0.94)',
              border: isReadOnly ? '1px solid rgba(244, 63, 94, 0.45)' : '1px solid rgba(99, 102, 241, 0.45)',
              borderRadius: '0.48rem',
              padding: '0.25rem 0.55rem'
            }}
          >
            <Eye size={15} color={isReadOnly ? '#f43f5e' : '#6366f1'} />
            <span style={{ fontSize: '0.78rem', color: isReadOnly ? '#f43f5e' : '#6366f1', fontWeight: 600, whiteSpace: 'nowrap' }}>
              视图
            </span>
            <CustomSelect
              value={targetUserId || currentUser?.userId || ''}
              onChange={(val) => onSelectTargetUser(Number(val))}
              options={usersList.map((u) => {
                const isSelfUser = u.isSelf || u.id === currentUser?.userId;
                const labelText = isSelfUser ? u.username : `${u.username} (只读)`;
                return { label: labelText, value: u.id };
              })}
              className="custom-select-sm"
              style={{ minWidth: '120px' }}
            />
          </div>
        )}

        {/* 超级管理员用户管理按钮 (用户管理仅超级管理员可见) */}
        {isSuperAdmin && (
          <button className="btn btn-secondary" onClick={onOpenUserManagement} title="用户账号与配置管理">
            <Users size={16} color="#6366f1" />
            <span>用户管理</span>
          </button>
        )}

        {/* API 设置按钮 (仅超级管理员可见，全局配置不受视图切换限制) */}
        {isSuperAdmin && (
          <button
            className="btn btn-secondary"
            onClick={onOpenTokenModal}
            title="配置第三方订单数据同步 Token与 Cookie (全局系统配置)"
          >
            <Settings size={16} />
            <span>API 设置</span>
          </button>
        )}

        <button
          className="btn btn-secondary"
          onClick={onOpenConfig}
          title={isReadOnly ? '查看当前账户绑定的落地页 ID（只读模式）' : '管理当前账户绑定的落地页 ID'}
        >
          <Settings size={16} />
          <span>落地页配置</span>
        </button>

        <button
          className="btn btn-secondary"
          onClick={() => {
            if (isReadOnly) return;
            onOpenBatchSpend();
          }}
          style={{ cursor: isReadOnly ? 'not-allowed' : 'pointer' }}
          title={isReadOnly ? '主账号或只读视图模式下不可在此手动导入消耗' : '批量导入每日账户消耗与备注'}
        >
          <Upload size={16} />
          <span>消耗导入</span>
        </button>

        <button
          className="btn btn-secondary"
          onClick={onOpenSyncModal}
          disabled={loading}
        >
          <RefreshCw size={16} className={loading ? 'spin' : ''} />
          <span>{loading ? '同步中...' : '数据同步'}</span>
        </button>

        {/* 导出表格按钮 */}
        {hasPermExport && (
          <button
            className="theme-toggle-btn"
            onClick={onOpenExportModal}
            title="导出表格数据 (支持自定义时间段)"
            style={{ cursor: 'pointer' }}
          >
            <Download size={17} color="#10b981" />
          </button>
        )}

        {/* AI视频生成按钮 */}
        {hasPermVideoGen && (
          <button
            className="theme-toggle-btn"
            onClick={() => window.open('https://video.gether.top', '_blank', 'noopener,noreferrer')}
            title="AI视频生成"
            style={{ cursor: 'pointer' }}
          >
            <Video size={17} color="#8b5cf6" />
          </button>
        )}

        {/* 退出登录按钮 (格式：图标 username) */}
        <button
          className="btn btn-secondary"
          style={{ color: '#f43f5e', borderColor: 'rgba(244, 63, 94, 0.25)', gap: '0.45rem' }}
          onClick={onLogout}
          title={`当前账号: ${currentUser?.username || localStorage.getItem('admin_username') || ''} (点击退出登录)`}
        >
          <LogOut size={16} />
          <span>{currentUser?.username || localStorage.getItem('admin_username') || '未知'}</span>
        </button>
      </div>

      {/* 移动端汉堡菜单触发按钮 */}
      <button
        className="mobile-menu-btn"
        onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
        aria-label="打开操作菜单"
      >
        {isMobileMenuOpen ? <X size={20} /> : <Menu size={20} />}
      </button>

      {/* 移动端抽屉浮层 (挂载到 document.body 防止父级容器样式截断) */}
      {isMobileMenuOpen && typeof document !== 'undefined' && createPortal(
        <div className="mobile-drawer-overlay" onClick={() => setIsMobileMenuOpen(false)}>
          <div className="mobile-drawer" onClick={(e) => e.stopPropagation()}>
            <div className="mobile-drawer-header">
              <div className="mobile-drawer-user">
                <span className="mobile-user-name">
                  {currentUser?.username || localStorage.getItem('admin_username') || '用户'}
                </span>
                {isSuperAdmin && <span className="mobile-admin-badge">超级管理员</span>}
                {isReadOnly && <span className="mobile-readonly-badge">只读视图</span>}
              </div>
              <button
                className="mobile-drawer-close"
                onClick={() => setIsMobileMenuOpen(false)}
                aria-label="关闭操作菜单"
              >
                <X size={20} />
              </button>
            </div>

            <div className="mobile-drawer-content">
              {/* 移动端平台切换 */}
              {platformsList && platformsList.length > 0 && (
                <div className="mobile-drawer-section">
                  <div className="mobile-section-title">
                    <Globe size={14} color="#10b981" />
                    <span>选择平台</span>
                  </div>
                  <CustomSelect
                    value={selectedPlatform || 'rocnovel'}
                    onChange={(val) => {
                      onSelectPlatform && onSelectPlatform(val);
                      setIsMobileMenuOpen(false);
                    }}
                    options={platformsList.map((p) => ({
                      label: p.name || p.code,
                      value: p.code
                    }))}
                    className="custom-select-sm"
                    style={{ width: '100%' }}
                  />
                </div>
              )}

              {/* 移动端视图切换 */}
              {canSwitchView && (
                <div className="mobile-drawer-section">
                  <div className="mobile-section-title">
                    <Eye size={14} color="#6366f1" />
                    <span>切换数据视图</span>
                  </div>
                  <CustomSelect
                    value={targetUserId || currentUser?.userId || ''}
                    onChange={(val) => {
                      onSelectTargetUser(Number(val));
                      setIsMobileMenuOpen(false);
                    }}
                    options={usersList.map((u) => {
                      const isSelfUser = u.isSelf || u.id === currentUser?.userId;
                      const labelText = isSelfUser ? u.username : `${u.username} (只读)`;
                      return { label: labelText, value: u.id };
                    })}
                    className="custom-select-sm"
                    style={{ width: '100%' }}
                  />
                </div>
              )}

              {/* 移动端功能菜单列表 */}
              <div className="mobile-drawer-section">
                <div className="mobile-section-title">常用操作</div>
                <div className="mobile-actions-list">
                  {isSuperAdmin && (
                    <button
                      className="mobile-action-item"
                      onClick={() => handleAction(onOpenUserManagement)}
                    >
                      <Users size={18} color="#6366f1" />
                      <span>用户管理</span>
                    </button>
                  )}

                  {isSuperAdmin && (
                    <button
                      className="mobile-action-item"
                      onClick={() => handleAction(onOpenTokenModal)}
                    >
                      <Settings size={18} />
                      <span>API 设置</span>
                    </button>
                  )}

                  <button
                    className="mobile-action-item"
                    onClick={() => handleAction(onOpenConfig)}
                  >
                    <Settings size={18} />
                    <span>落地页配置</span>
                  </button>

                  <button
                    className="mobile-action-item"
                    onClick={() => {
                      if (isReadOnly) return;
                      handleAction(onOpenBatchSpend);
                    }}
                    disabled={isReadOnly}
                  >
                    <Upload size={18} />
                    <span>消耗导入</span>
                  </button>

                  <button
                    className="mobile-action-item"
                    onClick={() => handleAction(onOpenSyncModal)}
                    disabled={loading}
                  >
                    <RefreshCw size={18} className={loading ? 'spin' : ''} />
                    <span>{loading ? '数据同步中...' : '数据同步'}</span>
                  </button>

                  {hasPermExport && (
                    <button
                      className="mobile-action-item"
                      onClick={() => handleAction(onOpenExportModal)}
                    >
                      <Download size={18} color="#10b981" />
                      <span>导出表格</span>
                    </button>
                  )}

                  {hasPermVideoGen && (
                    <button
                      className="mobile-action-item"
                      onClick={() => {
                        window.open('https://video.gether.top', '_blank', 'noopener,noreferrer');
                        setIsMobileMenuOpen(false);
                      }}
                    >
                      <Video size={18} color="#8b5cf6" />
                      <span>AI视频生成</span>
                    </button>
                  )}
                </div>
              </div>
            </div>

            <div className="mobile-drawer-footer">
              <button
                className="mobile-logout-btn"
                onClick={() => handleAction(onLogout)}
              >
                <LogOut size={16} />
                <span>退出登录</span>
              </button>
            </div>
          </div>
        </div>,
        document.body
      )}

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
        .spin {
          animation: spin 1s linear infinite;
        }
      `}</style>
    </header>
  );
}
