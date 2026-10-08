import React, { useState, useRef, useEffect } from 'react';
import {
  Globe,
  Eye,
  Menu,
  ChevronDown,
  LogOut
} from 'lucide-react';
import CustomSelect from './CustomSelect';

export default function AppTopBar({
  activeTab,
  onTabChange,
  selectedPlatform,
  platformsList,
  onSelectPlatform,
  usersList,
  targetUserId,
  currentUser,
  onSelectTargetUser,
  isReadOnly,
  isSidebarExpanded,
  onToggleSidebar,
  onToggleMobileMenu,
  onOpenUserManagement,
  onOpenTokenModal,
  onLogout
}) {
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const userMenuRef = useRef(null);

  const isSuperAdmin = currentUser && currentUser.role === 'SUPER_ADMIN';
  const canSwitchView = (isSuperAdmin || (usersList && usersList.length > 1)) && usersList && usersList.length > 0;

  // 点击外部关闭用户下拉菜单
  useEffect(() => {
    function handleClickOutside(e) {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target)) {
        setIsUserMenuOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const getPageTitle = () => {
    switch (activeTab) {
      case 'distribution':
        return '每日充值分析';
      case 'global-distribution':
        return '平台充值汇总';
      case 'settlement':
        return '财务结算单';
      case 'users':
        return '用户管理';
      case 'ltv':
      default:
        return 'LTV核心报表';
    }
  };

  const pageTitle = getPageTitle();
  const username = currentUser?.username || localStorage.getItem('admin_username') || 'Admin';
  const firstLetter = username.charAt(0).toUpperCase();

  return (
    <header className="app-topbar">
      {/* 左侧：移动端呼出按钮 + 当前页面标题 */}
      <div className="topbar-left">
        {/* 移动端汉堡呼出按钮 */}
        <button
          className="topbar-mobile-toggle"
          onClick={onToggleMobileMenu}
          aria-label="打开移动端导航"
        >
          <Menu size={20} />
        </button>

        <div className="topbar-page-badge">
          <span className="topbar-page-title">{pageTitle}</span>
        </div>
      </div>

      {/* 右侧：全局过滤器 + 顶部最右侧的用户头像与用户名 */}
      <div className="topbar-right">
        {/* 报表页面专属过滤器：平台选择器与视图选择器 */}
        {activeTab !== 'users' && (
          <>
            {platformsList && platformsList.length > 0 && (
              <div
                className="topbar-filter-pill platform-pill"
                title="数据源业务平台（默认：中文在线）"
              >
                <Globe size={15} color="#6366f1" />
                <span className="pill-label" style={{ color: '#6366f1' }}>平台</span>
                <CustomSelect
                  value={selectedPlatform || 'rocnovel'}
                  onChange={(val) => onSelectPlatform && onSelectPlatform(val)}
                  options={platformsList.map((p) => ({
                    label: p.name || p.code,
                    value: p.code
                  }))}
                  className="custom-select-sm"
                  style={{ width: '100%' }}
                />
              </div>
            )}

            {canSwitchView && (
              <div
                className={`topbar-filter-pill view-pill ${isReadOnly ? 'readonly' : ''}`}
                title={isReadOnly ? '只读模式：您正在查看其他被授权账户的数据视图' : '主视图：您正在查看当前登录账户的数据'}
              >
                <Eye size={15} color={isReadOnly ? '#f43f5e' : '#6366f1'} />
                <span className="pill-label" style={{ color: isReadOnly ? '#f43f5e' : '#6366f1' }}>
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
                  style={{ width: '100%' }}
                />
              </div>
            )}
          </>
        )}

        {/* 用户头像与用户名放置在顶部最右侧 */}
        <div className="topbar-user-container" ref={userMenuRef}>
          <button
            className="topbar-user-btn"
            onClick={() => setIsUserMenuOpen(prev => !prev)}
            title={`当前用户: ${username}`}
          >
            <div className="topbar-user-avatar">
              {firstLetter}
            </div>
            <span className="topbar-user-name">{username}</span>
            <ChevronDown size={14} className={`topbar-user-chevron ${isUserMenuOpen ? 'open' : ''}`} />
          </button>

          {isUserMenuOpen && (
            <div className="topbar-user-dropdown">
              <button
                className="topbar-dropdown-item logout-item"
                onClick={() => {
                  setIsUserMenuOpen(false);
                  onLogout();
                }}
              >
                <LogOut size={15} />
                <span>退出登录</span>
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
