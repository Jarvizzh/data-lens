import React, { useState, useRef, useEffect } from 'react';
import { createPortal } from 'react-dom';
import {
  BarChart3,
  TrendingUp,
  PieChart,
  Globe,
  Receipt,
  RefreshCw,
  Upload,
  Sliders,
  Download,
  Video,
  Shield,
  Users,
  Key,
  LogOut,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  X
} from 'lucide-react';

export default function AppSidebar({
  activeTab,
  onTabChange,
  loading,
  onOpenSyncModal,
  onOpenBatchSpend,
  onOpenConfig,
  onOpenExportModal,
  onOpenUserManagement,
  onOpenTokenModal,
  onLogout,
  currentUser,
  isReadOnly,
  isExpanded,
  onToggleSidebar,
  isMobileMenuOpen,
  setIsMobileMenuOpen
}) {
  const [isAdminSubmenuOpen, setIsAdminSubmenuOpen] = useState(false);
  const [isAdminFlyoutOpen, setIsAdminFlyoutOpen] = useState(false);
  const [flyoutPos, setFlyoutPos] = useState({ top: 0, left: 64 });
  const adminBtnRef = useRef(null);
  const adminFlyoutRef = useRef(null);

  const isSuperAdmin = currentUser && currentUser.role === 'SUPER_ADMIN';
  const hasPermGlobalDistribution = isSuperAdmin || currentUser?.permGlobalDistribution === 1;
  const hasPermExport = isSuperAdmin || currentUser?.permExport === 1;
  const hasPermSettlement = isSuperAdmin || currentUser?.permSettlement === 1;
  const hasPermVideoGen = isSuperAdmin || currentUser?.permVideoGen === 1;

  // 展开/收起切换时，关闭悬浮弹窗
  useEffect(() => {
    setIsAdminFlyoutOpen(false);
  }, [isExpanded]);

  // 点击空白处关闭悬浮浮层
  useEffect(() => {
    function handleClickOutside(e) {
      if (
        adminFlyoutRef.current &&
        !adminFlyoutRef.current.contains(e.target) &&
        adminBtnRef.current &&
        !adminBtnRef.current.contains(e.target)
      ) {
        setIsAdminFlyoutOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleTabClick = (tab) => {
    onTabChange(tab);
    setIsMobileMenuOpen(false);
  };

  const handleAdminToggle = (e) => {
    e.stopPropagation();
    if (isExpanded) {
      setIsAdminSubmenuOpen(prev => !prev);
    } else {
      if (adminBtnRef.current) {
        const rect = adminBtnRef.current.getBoundingClientRect();
        setFlyoutPos({
          top: Math.max(10, Math.min(window.innerHeight - 150, rect.top)),
          left: Math.round(rect.right + 8)
        });
      }
      setIsAdminFlyoutOpen(prev => !prev);
    }
  };

  const username = currentUser?.username || localStorage.getItem('admin_username') || 'Admin';
  const firstLetter = username.charAt(0).toUpperCase();

  return (
    <>
      {/* 桌面端常驻侧边栏 (默认展开 168px，收起 56px，点击 Logo 展开/收起) */}
      <aside className={`app-sidebar ${isExpanded ? 'expanded' : ''}`}>
        {/* 顶部 Brand：Logo 与 Meta-LTV，点击 Logo 展开/收缩侧边栏 */}
        <div
          className={`sidebar-brand-header ${isExpanded ? 'expanded' : ''}`}
          onClick={onToggleSidebar}
          title={isExpanded ? '点击收起侧边栏' : '点击展开侧边栏'}
        >
          <div className="sidebar-brand-left">
            <div className="sidebar-brand-icon">
              <BarChart3 size={17} />
            </div>
            {isExpanded && <span className="sidebar-brand-text">DataLens</span>}
          </div>
          {isExpanded && (
            <div className="sidebar-brand-toggle-btn" title="收起侧边栏">
              <ChevronLeft size={14} />
            </div>
          )}
        </div>

        {/* 分组 1: 核心报表导航 */}
        <div className="sidebar-section">
          <button
            className={`sidebar-nav-btn ${activeTab === 'ltv' ? 'active' : ''} ${isExpanded ? 'expanded' : ''}`}
            onClick={() => handleTabClick('ltv')}
            title={!isExpanded ? 'LTV报表' : undefined}
          >
            <TrendingUp size={19} className="flex-shrink-0" />
            {isExpanded && <span className="sidebar-btn-label">LTV报表</span>}
            {!isExpanded && <span className="sidebar-tooltip">LTV报表</span>}
          </button>

          <button
            className={`sidebar-nav-btn ${activeTab === 'distribution' ? 'active' : ''} ${isExpanded ? 'expanded' : ''}`}
            onClick={() => handleTabClick('distribution')}
            title={!isExpanded ? '充值分析' : undefined}
          >
            <PieChart size={19} className="flex-shrink-0" />
            {isExpanded && <span className="sidebar-btn-label">充值分析</span>}
            {!isExpanded && <span className="sidebar-tooltip">充值分析</span>}
          </button>

          {hasPermGlobalDistribution && (
            <button
              className={`sidebar-nav-btn ${activeTab === 'global-distribution' ? 'active' : ''} ${isExpanded ? 'expanded' : ''}`}
              onClick={() => handleTabClick('global-distribution')}
              title={!isExpanded ? '平台汇总' : undefined}
            >
              <Globe size={19} className="flex-shrink-0" />
              {isExpanded && <span className="sidebar-btn-label">平台汇总</span>}
              {!isExpanded && <span className="sidebar-tooltip">平台汇总</span>}
            </button>
          )}

          {hasPermSettlement && (
            <button
              className={`sidebar-nav-btn ${activeTab === 'settlement' ? 'active' : ''} ${isExpanded ? 'expanded' : ''}`}
              onClick={() => handleTabClick('settlement')}
              title={!isExpanded ? '财务结算' : undefined}
            >
              <Receipt size={19} className="flex-shrink-0" />
              {isExpanded && <span className="sidebar-btn-label">财务结算</span>}
              {!isExpanded && <span className="sidebar-tooltip">财务结算</span>}
            </button>
          )}
        </div>

        <div className="sidebar-divider" />

        {/* 分组 2: 常用业务操作 */}
        <div className="sidebar-section">
          <button
            className={`sidebar-action-btn ${isExpanded ? 'expanded' : ''}`}
            onClick={onOpenSyncModal}
            disabled={loading}
            title={!isExpanded ? (loading ? '订单数据同步中...' : '数据同步') : undefined}
          >
            <RefreshCw size={18} className={`flex-shrink-0 ${loading ? 'spin' : ''}`} />
            {isExpanded && <span className="sidebar-btn-label">{loading ? '同步中...' : '数据同步'}</span>}
            {!isExpanded && <span className="sidebar-tooltip">数据同步</span>}
          </button>

          <button
            className={`sidebar-action-btn ${isExpanded ? 'expanded' : ''}`}
            onClick={() => {
              if (isReadOnly) return;
              onOpenBatchSpend();
            }}
            style={{ cursor: isReadOnly ? 'not-allowed' : 'pointer', opacity: isReadOnly ? 0.4 : 1 }}
            title={!isExpanded ? (isReadOnly ? '只读模式不可导入消耗' : '消耗导入') : undefined}
          >
            <Upload size={18} className="flex-shrink-0" />
            {isExpanded && <span className="sidebar-btn-label">消耗导入</span>}
            {!isExpanded && <span className="sidebar-tooltip">消耗导入</span>}
          </button>

          <button
            className={`sidebar-action-btn ${isExpanded ? 'expanded' : ''}`}
            onClick={onOpenConfig}
            title={!isExpanded ? '落地页配置' : undefined}
          >
            <Sliders size={18} className="flex-shrink-0" />
            {isExpanded && <span className="sidebar-btn-label">落地页配置</span>}
            {!isExpanded && <span className="sidebar-tooltip">落地页配置</span>}
          </button>

          {hasPermExport && (
            <button
              className={`sidebar-action-btn ${isExpanded ? 'expanded' : ''}`}
              onClick={onOpenExportModal}
              title={!isExpanded ? '数据导出' : undefined}
            >
              <Download size={18} className="flex-shrink-0" />
              {isExpanded && <span className="sidebar-btn-label">数据导出</span>}
              {!isExpanded && <span className="sidebar-tooltip">数据导出</span>}
            </button>
          )}
        </div>

        {/* 分组 3: 扩展工具与系统管理 */}
        {(hasPermVideoGen || isSuperAdmin) && (
          <>
            <div className="sidebar-divider" />
            <div className="sidebar-section">
              {hasPermVideoGen && (
                <button
                  className={`sidebar-action-btn ${isExpanded ? 'expanded' : ''}`}
                  onClick={() => window.open('https://video.gether.top', '_blank', 'noopener,noreferrer')}
                  title={!isExpanded ? 'AI视频创作' : undefined}
                >
                  <Video size={18} className="flex-shrink-0" />
                  {isExpanded && <span className="sidebar-btn-label">AI视频创作</span>}
                  {!isExpanded && <span className="sidebar-tooltip">AI视频创作</span>}
                </button>
              )}

              {/* 超管系统管理：展开时2级导航栏，收起时点击浮窗（采用 fixed 定位防止被遮挡） */}
              {isSuperAdmin && (
                isExpanded ? (
                  <div className="sidebar-accordion-group">
                    <button
                      ref={adminBtnRef}
                      className="sidebar-action-btn expanded"
                      onClick={handleAdminToggle}
                      title="系统管理"
                    >
                      <Shield size={18} className="flex-shrink-0" />
                      <span className="sidebar-btn-label" style={{ flex: 1, textAlign: 'left' }}>系统管理</span>
                      <ChevronDown
                        size={14}
                        className={`sidebar-submenu-chevron ${isAdminSubmenuOpen ? 'open' : ''}`}
                      />
                    </button>

                    {isAdminSubmenuOpen && (
                      <div className="sidebar-sub-menu">
                        <button
                          className={`sidebar-sub-item ${activeTab === 'users' ? 'active' : ''}`}
                          onClick={() => handleTabClick('users')}
                        >
                          <Users size={14} className="flex-shrink-0" />
                          <span>用户管理</span>
                        </button>
                        <button
                          className="sidebar-sub-item"
                          onClick={() => onOpenTokenModal && onOpenTokenModal()}
                        >
                          <Key size={14} className="flex-shrink-0" />
                          <span>API 密钥</span>
                        </button>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="sidebar-popover-container">
                    <button
                      ref={adminBtnRef}
                      className={`sidebar-action-btn ${activeTab === 'users' ? 'active' : ''}`}
                      onClick={handleAdminToggle}
                      title="系统管理"
                    >
                      <Shield size={19} className="flex-shrink-0" />
                      <span className="sidebar-tooltip">系统管理</span>
                    </button>

                    {isAdminFlyoutOpen && typeof document !== 'undefined' && createPortal(
                      <div
                        className="sidebar-fixed-flyout"
                        ref={adminFlyoutRef}
                        style={{
                          position: 'fixed',
                          top: `${flyoutPos.top}px`,
                          left: `${flyoutPos.left}px`,
                          zIndex: 99999
                        }}
                      >
                        <div className="flyout-header">系统管理</div>
                        <button
                          className={`popover-item ${activeTab === 'users' ? 'active' : ''}`}
                          onClick={() => {
                            setIsAdminFlyoutOpen(false);
                            handleTabClick('users');
                          }}
                        >
                          <Users size={14} />
                          <span>用户管理</span>
                        </button>
                        <button
                          className="popover-item"
                          onClick={() => {
                            setIsAdminFlyoutOpen(false);
                            onOpenTokenModal && onOpenTokenModal();
                          }}
                        >
                          <Key size={14} />
                          <span>API 密钥</span>
                        </button>
                      </div>,
                      document.body
                    )}
                  </div>
                )
              )}
            </div>
          </>
        )}

        {/* 底部弹性垫高 */}
        <div className="sidebar-spacer" />
      </aside>

      {/* 移动端侧滑抽屉 Drawer (仅在 <= 768px 点击汉堡菜单呼出) */}
      {isMobileMenuOpen && (
        <div className="sidebar-mobile-drawer-overlay" onClick={() => setIsMobileMenuOpen(false)}>
          <div className="sidebar-mobile-drawer" onClick={(e) => e.stopPropagation()}>
            <div className="mobile-drawer-header">
              <div className="flex items-center gap-2">
                <div className="sidebar-brand-icon" style={{ width: '28px', height: '28px' }}>
                  <BarChart3 size={18} />
                </div>
                <span className="brand-title" style={{ fontSize: '1.1rem' }}>DataLens</span>
              </div>
              <button
                className="theme-toggle-btn"
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <X size={20} />
              </button>
            </div>

            <div className="mobile-drawer-content">
              <div className="drawer-group-title">核心报表</div>
              <button
                className={`drawer-item ${activeTab === 'ltv' ? 'active' : ''}`}
                onClick={() => handleTabClick('ltv')}
              >
                <TrendingUp size={18} />
                <span>LTV报表</span>
              </button>
              <button
                className={`drawer-item ${activeTab === 'distribution' ? 'active' : ''}`}
                onClick={() => handleTabClick('distribution')}
              >
                <PieChart size={18} />
                <span>充值分析</span>
              </button>
              {hasPermGlobalDistribution && (
                <button
                  className={`drawer-item ${activeTab === 'global-distribution' ? 'active' : ''}`}
                  onClick={() => handleTabClick('global-distribution')}
                >
                  <Globe size={18} />
                  <span>平台汇总</span>
                </button>
              )}
              {hasPermSettlement && (
                <button
                  className={`drawer-item ${activeTab === 'settlement' ? 'active' : ''}`}
                  onClick={() => handleTabClick('settlement')}
                >
                  <Receipt size={18} />
                  <span>财务结算</span>
                </button>
              )}

              <div className="drawer-group-title" style={{ marginTop: '1rem' }}>业务操作</div>
              <button
                className="drawer-item"
                onClick={() => {
                  setIsMobileMenuOpen(false);
                  onOpenSyncModal();
                }}
              >
                <RefreshCw size={18} />
                <span>数据同步</span>
              </button>
              <button
                className="drawer-item"
                onClick={() => {
                  if (isReadOnly) return;
                  setIsMobileMenuOpen(false);
                  onOpenBatchSpend();
                }}
                disabled={isReadOnly}
              >
                <Upload size={18} />
                <span>消耗导入</span>
              </button>
              <button
                className="drawer-item"
                onClick={() => {
                  setIsMobileMenuOpen(false);
                  onOpenConfig();
                }}
              >
                <Sliders size={18} />
                <span>落地页配置</span>
              </button>
              {hasPermExport && (
                <button
                  className="drawer-item"
                  onClick={() => {
                    setIsMobileMenuOpen(false);
                    onOpenExportModal();
                  }}
                >
                  <Download size={18} />
                  <span>数据导出</span>
                </button>
              )}

              {isSuperAdmin && (
                <>
                  <div className="drawer-group-title" style={{ marginTop: '1rem' }}>系统管理</div>
                  <button
                    className={`drawer-item ${activeTab === 'users' ? 'active' : ''}`}
                    onClick={() => handleTabClick('users')}
                  >
                    <Users size={18} />
                    <span>用户管理</span>
                  </button>
                  <button
                    className="drawer-item"
                    onClick={() => {
                      setIsMobileMenuOpen(false);
                      onOpenTokenModal();
                    }}
                  >
                    <Key size={18} />
                    <span>API 密钥</span>
                  </button>
                </>
              )}

              {hasPermVideoGen && (
                <>
                  <div className="drawer-group-title" style={{ marginTop: '1rem' }}>工具</div>
                  <button
                    className="drawer-item"
                    onClick={() => window.open('https://video.gether.top', '_blank', 'noopener,noreferrer')}
                  >
                    <Video size={18} />
                    <span>AI视频创作</span>
                  </button>
                </>
              )}
            </div>

            <div className="mobile-drawer-footer">
              <div className="drawer-user-info">
                <div className="sidebar-avatar-inner" style={{ width: '32px', height: '32px', fontSize: '0.85rem' }}>
                  {firstLetter}
                </div>
                <div>
                  <div className="font-semibold text-sm">{username}</div>
                  <div className="text-xs text-slate-400">
                    {currentUser?.role === 'SUPER_ADMIN' ? '超级管理员' : currentUser?.role === 'ADMIN' ? '管理员' : '用户'}
                  </div>
                </div>
              </div>
              <button
                className="btn btn-secondary"
                style={{ color: '#f43f5e', borderColor: 'rgba(244, 63, 94, 0.3)', padding: '0.35rem 0.65rem' }}
                onClick={() => {
                  setIsMobileMenuOpen(false);
                  onLogout();
                }}
              >
                <LogOut size={16} />
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
