import React, { useState, useEffect } from 'react';
import AppSidebar from './components/AppSidebar';
import AppTopBar from './components/AppTopBar';
import LtvTable from './components/LtvTable';
import DailyRechargeDistributionTable from './components/DailyRechargeDistributionTable';
import LandingPageConfigModal from './components/LandingPageConfigModal';
import EditSpendModal from './components/EditSpendModal';
import TokenConfigModal from './components/TokenConfigModal';
import SyncModal from './components/SyncModal';
import BatchSpendModal from './components/BatchSpendModal';
import UserManagementPage from './components/UserManagementPage';
import LogoutConfirmModal from './components/LogoutConfirmModal';
import ExportModal from './components/ExportModal';
import MonthlySettlementTable from './components/MonthlySettlementTable';
import { exportLtvTable, exportDistributionTable } from './utils/exportExcel';
import Login from './components/Login';
import Toast from './components/Toast';
import { DollarSign, TrendingUp, Users, Wallet, AlertTriangle, Calendar, Info, X } from 'lucide-react';

const VALID_TABS = ['ltv', 'distribution', 'global-distribution', 'settlement', 'users'];

const getTabFromHash = () => {
  if (typeof window === 'undefined') return 'ltv';
  const cleanHash = window.location.hash.replace(/^#\/?/, '').trim().toLowerCase();
  return VALID_TABS.includes(cleanHash) ? cleanHash : 'ltv';
};

export default function App() {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState(() => {
    return !!localStorage.getItem('admin_token');
  });

  const [currentUser, setCurrentUser] = useState(() => {
    const uid = localStorage.getItem('admin_user_id');
    return {
      userId: uid ? Number(uid) : 1,
      username: localStorage.getItem('admin_username') || 'admin',
      role: localStorage.getItem('admin_role') || 'USER',
      permPredictPayback: Number(localStorage.getItem('admin_perm_predict_payback') || 0),
      permRoiPredict: Number(localStorage.getItem('admin_perm_roi_predict') || 0),
      permGlobalDistribution: Number(localStorage.getItem('admin_perm_global_distribution') || 0),
      permExport: Number(localStorage.getItem('admin_perm_export') || 0),
      permSettlement: Number(localStorage.getItem('admin_perm_settlement') || 0),
      permVideoGen: Number(localStorage.getItem('admin_perm_video_gen') || 0),
    };
  });

  const isSuperAdmin = Boolean(currentUser && currentUser.role === 'SUPER_ADMIN');
  const hasPermPredictPayback = isSuperAdmin || Boolean(currentUser?.permPredictPayback === 1);
  const hasPermRoiPredict = isSuperAdmin || Boolean(currentUser?.permRoiPredict === 1);
  const hasPermGlobalDistribution = isSuperAdmin || Boolean(currentUser?.permGlobalDistribution === 1);
  const hasPermExport = isSuperAdmin || Boolean(currentUser?.permExport === 1);
  const hasPermSettlement = isSuperAdmin || Boolean(currentUser?.permSettlement === 1);
  const hasPermVideoGen = isSuperAdmin || Boolean(currentUser?.permVideoGen === 1);

  useEffect(() => {
    const token = localStorage.getItem('admin_token');
    if (token) {
      fetch('/api/auth/check', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      .then(res => res.json())
      .then(data => {
        if (data && data.code === 0) {
          localStorage.setItem('admin_perm_predict_payback', data.permPredictPayback || 0);
          localStorage.setItem('admin_perm_roi_predict', data.permRoiPredict || 0);
          localStorage.setItem('admin_perm_global_distribution', data.permGlobalDistribution || 0);
          localStorage.setItem('admin_perm_export', data.permExport || 0);
          localStorage.setItem('admin_perm_settlement', data.permSettlement || 0);
          localStorage.setItem('admin_perm_video_gen', data.permVideoGen || 0);
          localStorage.setItem('admin_role', data.role || 'USER');
          setCurrentUser(prev => ({
            ...prev,
            role: data.role || prev.role,
            permPredictPayback: data.permPredictPayback || 0,
            permRoiPredict: data.permRoiPredict || 0,
            permGlobalDistribution: data.permGlobalDistribution || 0,
            permExport: data.permExport || 0,
            permSettlement: data.permSettlement || 0,
            permVideoGen: data.permVideoGen || 0,
          }));
        }
      })
      .catch(() => {});
    }
  }, []);

  const [targetUserId, setTargetUserId] = useState(() => {
    const sessionTarget = sessionStorage.getItem('admin_target_user_id');
    if (sessionTarget && !isNaN(Number(sessionTarget))) {
      return Number(sessionTarget);
    }
    const localTarget = localStorage.getItem('admin_target_user_id');
    if (localTarget && !isNaN(Number(localTarget))) {
      return Number(localTarget);
    }
    const uid = localStorage.getItem('admin_user_id');
    return uid ? Number(uid) : 1;
  });

  const [usersList, setUsersList] = useState([]);
  const [platformsList, setPlatformsList] = useState([]);
  const [selectedPlatform, setSelectedPlatform] = useState(() => {
    const sessionSaved = sessionStorage.getItem('admin_selected_platform');
    if (sessionSaved && (sessionSaved.toLowerCase() === 'rocnovel' || sessionSaved.toLowerCase() === 'flicknovel')) {
      return sessionSaved;
    }
    const localSaved = localStorage.getItem('admin_selected_platform');
    if (localSaved && (localSaved.toLowerCase() === 'rocnovel' || localSaved.toLowerCase() === 'flicknovel')) {
      return localSaved;
    }
    return 'rocnovel'; // 默认进入页面选择 <中文在线> (rocnovel)
  });
  const [isSidebarExpanded, setIsSidebarExpanded] = useState(() => {
    const saved = localStorage.getItem('sidebar_expanded');
    return saved !== null ? saved === 'true' : true; // 默认展开 (true)
  });

  const toggleSidebar = () => {
    setIsSidebarExpanded(prev => {
      const next = !prev;
      localStorage.setItem('sidebar_expanded', String(next));
      return next;
    });
  };

  const [activeTab, setActiveTab] = useState(() => getTabFromHash());

  // 监听浏览器前进、后退以及直接在地址栏变更 URL hash
  useEffect(() => {
    const handleHashChange = () => {
      const tab = getTabFromHash();
      setActiveTab(tab);
    };
    window.addEventListener('hashchange', handleHashChange);
    return () => window.removeEventListener('hashchange', handleHashChange);
  }, []);

  // 保证 activeTab 变化时 URL hash 随之同步
  useEffect(() => {
    const currentHash = window.location.hash.replace(/^#\/?/, '').trim().toLowerCase();
    if (currentHash !== activeTab) {
      window.location.hash = `#/${activeTab}`;
    }
  }, [activeTab]);

  useEffect(() => {
    if (activeTab === 'global-distribution' && !hasPermGlobalDistribution) {
      setActiveTab('ltv');
    } else if (activeTab === 'settlement' && !hasPermSettlement) {
      setActiveTab('ltv');
    } else if (activeTab === 'users' && !isSuperAdmin) {
      setActiveTab('ltv');
    }
  }, [activeTab, hasPermGlobalDistribution, hasPermSettlement, isSuperAdmin]);
  const [data, setData] = useState([]);
  const [distributionData, setDistributionData] = useState([]);
  const [distributionSummary, setDistributionSummary] = useState(null);
  const [globalDistributionData, setGlobalDistributionData] = useState([]);
  const [globalDistributionSummary, setGlobalDistributionSummary] = useState(null);
  const [loading, setLoading] = useState(false);
  const [loadingType, setLoadingType] = useState(null);
  const [isConfigOpen, setIsConfigOpen] = useState(false);
  const [isTokenModalOpen, setIsTokenModalOpen] = useState(false);
  const [isSyncModalOpen, setIsSyncModalOpen] = useState(false);
  const [isBatchSpendOpen, setIsBatchSpendOpen] = useState(false);
  const [isLogoutModalOpen, setIsLogoutModalOpen] = useState(false);
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);
  const [editingTargetUserLandingPage, setEditingTargetUserLandingPage] = useState(null);

  const [editingRow, setEditingRow] = useState(null);
  const [errorMessage, setErrorMessage] = useState('');
  const [toast, setToast] = useState(null);

  const handleConfirmExport = (dateRange) => {
    if (activeTab === 'ltv') {
      exportLtvTable(displayedLtvData, hasPermPredictPayback, currentUser?.username || '', dateRange);
    } else if (activeTab === 'distribution') {
      exportDistributionTable(distributionData, false, dateRange);
    } else if (activeTab === 'global-distribution') {
      exportDistributionTable(distributionData, true, dateRange);
    }
  };

  const [theme, setTheme] = useState(() => {
    const saved = localStorage.getItem('ltv_theme');
    if (saved === 'dark' || saved === 'light') return saved;
    return 'dark';
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('ltv_theme', theme);
  }, [theme]);

  const toggleTheme = () => {
    const nextTheme = theme === 'dark' ? 'light' : 'dark';
    setTheme(nextTheme);
  };

  const showToast = (message, type = 'info') => {
    setToast({ message, type });
    setTimeout(() => {
      setToast(null);
    }, 4000);
  };

  const authFetch = async (url, options = {}) => {
    const token = localStorage.getItem('admin_token');
    const headers = {
      ...(options.headers || {}),
      'Authorization': token ? `Bearer ${token}` : '',
    };

    const res = await fetch(url, { ...options, headers });
    if (res.status === 401) {
      localStorage.removeItem('admin_token');
      localStorage.removeItem('admin_username');
      localStorage.removeItem('admin_role');
      localStorage.removeItem('admin_user_id');
      localStorage.removeItem('admin_selected_platform');
      sessionStorage.removeItem('admin_selected_platform');
      localStorage.removeItem('admin_target_user_id');
      sessionStorage.removeItem('admin_target_user_id');
      setIsAuthenticated(false);
      showToast('未登录或登录凭证已过 3 天有效期，请重新登录', 'warning');
      throw new Error('UNAUTHORIZED');
    }
    return res;
  };

  const fetchUsersList = async () => {
    const token = localStorage.getItem('admin_token');
    if (!token) return;
    try {
      const res = await authFetch('/api/user/visible-accounts');
      const json = await res.json();
      if (json.code === 0 && Array.isArray(json.data) && json.data.length > 0) {
        setUsersList(json.data);
        const savedTarget = sessionStorage.getItem('admin_target_user_id') || localStorage.getItem('admin_target_user_id');
        if (savedTarget) {
          const targetIdNum = Number(savedTarget);
          const isAllowed = json.data.some(u => u.id === targetIdNum);
          if (!isAllowed) {
            const fallbackUid = currentUser?.userId || Number(localStorage.getItem('admin_user_id')) || json.data[0].id;
            setTargetUserId(fallbackUid);
            sessionStorage.setItem('admin_target_user_id', String(fallbackUid));
            localStorage.setItem('admin_target_user_id', String(fallbackUid));
          }
        }
      }
    } catch (e) {
      console.error('Failed to fetch visible accounts:', e);
    }
  };

  const fetchPlatformsList = async () => {
    const token = localStorage.getItem('admin_token');
    if (!token) return;
    try {
      const res = await authFetch('/api/platform/list');
      const json = await res.json();
      if (json.code === 0 && Array.isArray(json.data) && json.data.length > 0) {
        setPlatformsList(json.data);
        const currentSaved = sessionStorage.getItem('admin_selected_platform') || localStorage.getItem('admin_selected_platform') || 'rocnovel';
        const isCurrentAllowed = json.data.some(p => p.code.toLowerCase() === currentSaved.toLowerCase());
        if (!isCurrentAllowed) {
          const fallbackCode = json.data.find(p => p.code.toLowerCase() === 'rocnovel')?.code || json.data[0].code;
          setSelectedPlatform(fallbackCode);
          sessionStorage.setItem('admin_selected_platform', fallbackCode);
          localStorage.setItem('admin_selected_platform', fallbackCode);
        }
      }
    } catch (e) {
      console.error('Failed to fetch platforms list:', e);
    }
  };

  const handleSelectPlatform = (platformCode) => {
    const nextPlat = platformCode || 'rocnovel';
    setSelectedPlatform(nextPlat);
    sessionStorage.setItem('admin_selected_platform', nextPlat);
    localStorage.setItem('admin_selected_platform', nextPlat);
    setData([]);
    setDistributionData([]);
    setDistributionSummary(null);
    setGlobalDistributionData([]);
    setGlobalDistributionSummary(null);
    const pObj = platformsList.find(p => p.code.toLowerCase() === nextPlat.toLowerCase());
    const name = pObj ? pObj.name : nextPlat;
    showToast(`已切换至平台视图: [${name}]，已自动刷新数据`, 'info');
  };

  const [backendOverallPaybackDays, setBackendOverallPaybackDays] = useState(null);
  const [backendOverallPaybackCycleDays, setBackendOverallPaybackCycleDays] = useState(null);
  const [overallPredictedDay30Roi, setOverallPredictedDay30Roi] = useState(null);
  const [overallPredictedDay60Roi, setOverallPredictedDay60Roi] = useState(null);
  const [overallPredictedDay90Roi, setOverallPredictedDay90Roi] = useState(null);
  const [monthlySummary, setMonthlySummary] = useState(null);
  const [hoveredMonthlyPrediction, setHoveredMonthlyPrediction] = useState(null);
  const [overallRetainedSubUsers, setOverallRetainedSubUsers] = useState(0);
  const [overallRetainedRate, setOverallRetainedRate] = useState('0.00%');

  const fetchLtvData = async (overrideUserId, overridePlatform) => {
    if (!localStorage.getItem('admin_token')) return;
    setLoading(true);
    const uid = overrideUserId !== undefined ? overrideUserId : targetUserId;
    const plat = overridePlatform !== undefined ? overridePlatform : selectedPlatform;
    try {
      const res = await authFetch(`/api/ltv/list?targetUserId=${uid || ''}&platformCode=${encodeURIComponent(plat || 'ALL')}`);
      const json = await res.json();
      if (json.code === 0 && Array.isArray(json.data)) {
        setData(json.data);
        if (json.overallPredictedPaybackDays !== undefined) {
          setBackendOverallPaybackDays(json.overallPredictedPaybackDays);
        }
        if (json.overallPaybackCycleDays !== undefined) {
          setBackendOverallPaybackCycleDays(json.overallPaybackCycleDays);
        }
        if (json.overallPredictedDay30Roi !== undefined) {
          setOverallPredictedDay30Roi(json.overallPredictedDay30Roi);
        }
        if (json.overallPredictedDay60Roi !== undefined) {
          setOverallPredictedDay60Roi(json.overallPredictedDay60Roi);
        }
        if (json.overallPredictedDay90Roi !== undefined) {
          setOverallPredictedDay90Roi(json.overallPredictedDay90Roi);
        }
        if (json.monthlySummary) {
          setMonthlySummary(json.monthlySummary);
        }
        if (json.overallRetainedSubUsers !== undefined) {
          setOverallRetainedSubUsers(json.overallRetainedSubUsers);
        }
        if (json.overallRetainedRate !== undefined) {
          setOverallRetainedRate(json.overallRetainedRate);
        }
      }
    } catch (err) {
      if (err.message !== 'UNAUTHORIZED') {
        console.error('Failed to fetch LTV data:', err);
        showToast('获取 LTV 统计数据失败，请检查后端服务', 'error');
      }
    } finally {
      setLoading(false);
    }
  };

  const fetchDistributionData = async (overrideUserId, overridePlatform) => {
    if (!localStorage.getItem('admin_token')) return;
    setLoading(true);
    const uid = overrideUserId !== undefined ? overrideUserId : targetUserId;
    const plat = overridePlatform !== undefined ? overridePlatform : selectedPlatform;
    try {
      const res = await authFetch(`/api/ltv/daily-distribution?targetUserId=${uid || ''}&platformCode=${encodeURIComponent(plat || 'ALL')}`);
      const json = await res.json();
      if (json.code === 0 && Array.isArray(json.data)) {
        setDistributionData(json.data);
        if (json.summary) {
          setDistributionSummary(json.summary);
        }
      }
    } catch (err) {
      if (err.message !== 'UNAUTHORIZED') {
        console.error('Failed to fetch daily distribution data:', err);
        showToast('获取每日充值分布数据失败', 'error');
      }
    } finally {
      setLoading(false);
    }
  };

  const fetchGlobalDistributionData = async (overridePlatform) => {
    if (!localStorage.getItem('admin_token')) return;
    if (!hasPermGlobalDistribution) return;
    setLoading(true);
    const plat = overridePlatform !== undefined ? overridePlatform : selectedPlatform;
    try {
      const res = await authFetch(`/api/ltv/global-daily-distribution?platformCode=${encodeURIComponent(plat || 'ALL')}`);
      const json = await res.json();
      if (json.code === 0 && Array.isArray(json.data)) {
        setGlobalDistributionData(json.data);
        if (json.summary) {
          setGlobalDistributionSummary(json.summary);
        }
      }
    } catch (err) {
      if (err.message !== 'UNAUTHORIZED') {
        console.error('Failed to fetch global daily distribution data:', err);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isAuthenticated) {
      fetchUsersList();
      fetchPlatformsList();
      if (activeTab === 'ltv') {
        fetchLtvData(targetUserId, selectedPlatform);
      } else if (activeTab === 'distribution') {
        fetchDistributionData(targetUserId, selectedPlatform);
      } else if (activeTab === 'global-distribution') {
        if (hasPermGlobalDistribution) {
          fetchGlobalDistributionData(selectedPlatform);
        }
      }
    }
  }, [isAuthenticated, activeTab, targetUserId, selectedPlatform, hasPermGlobalDistribution]);

  const handleSelectTargetUser = (newUserId) => {
    setTargetUserId(newUserId);
    sessionStorage.setItem('admin_target_user_id', String(newUserId));
    localStorage.setItem('admin_target_user_id', String(newUserId));
    // 切换视图时清空上一视图数据并触发数据自动刷新
    setData([]);
    setDistributionData([]);
    setDistributionSummary(null);

    if (activeTab === 'ltv') {
      fetchLtvData(newUserId, selectedPlatform);
    } else if (activeTab === 'distribution') {
      fetchDistributionData(newUserId, selectedPlatform);
    } else if (activeTab === 'global-distribution') {
      if (hasPermGlobalDistribution) {
        fetchGlobalDistributionData(selectedPlatform);
      }
    }
    const userObj = usersList.find(u => u.id === newUserId);
    showToast(`已切换至用户视图: [${userObj ? userObj.username : newUserId}]，已自动刷新数据`, 'info');
  };

  const handleLoginSuccess = (loginData) => {
    setIsAuthenticated(true);
    const newUid = loginData.userId;
    const userObj = {
      userId: newUid,
      username: loginData.username,
      role: loginData.role,
      permPredictPayback: loginData.permPredictPayback || 0,
      permRoiPredict: loginData.permRoiPredict || 0,
      permGlobalDistribution: loginData.permGlobalDistribution || 0,
      permExport: loginData.permExport || 0,
      permSettlement: loginData.permSettlement || 0,
      permVideoGen: loginData.permVideoGen || 0,
    };
    setCurrentUser(userObj);
    setTargetUserId(newUid);
    sessionStorage.setItem('admin_target_user_id', String(newUid));
    localStorage.setItem('admin_target_user_id', String(newUid));

    // 清空上一个账号的数据缓存
    setData([]);
    setDistributionData([]);
    setDistributionSummary(null);
    setGlobalDistributionData([]);
    setGlobalDistributionSummary(null);

    // 登录后默认选择 <中文在线> (rocnovel) 平台
    setSelectedPlatform('rocnovel');
    sessionStorage.setItem('admin_selected_platform', 'rocnovel');
    localStorage.setItem('admin_selected_platform', 'rocnovel');

    fetchUsersList();
    fetchPlatformsList();

    // 立即自动拉取刷新新登录账号的数据 (默认中文在线平台)
    if (activeTab === 'ltv') {
      fetchLtvData(newUid, 'rocnovel');
    } else if (activeTab === 'distribution') {
      fetchDistributionData(newUid, 'rocnovel');
    } else if (activeTab === 'global-distribution') {
      if (userObj.role === 'SUPER_ADMIN' || userObj.permGlobalDistribution === 1) {
        fetchGlobalDistributionData('rocnovel');
      }
    }

    showToast(`登录成功！欢迎 ${loginData.username}，已加载最新数据`, 'success');
  };

  const handleLogout = () => {
    localStorage.removeItem('admin_token');
    localStorage.removeItem('admin_username');
    localStorage.removeItem('admin_role');
    localStorage.removeItem('admin_user_id');
    localStorage.removeItem('admin_selected_platform');
    sessionStorage.removeItem('admin_selected_platform');
    localStorage.removeItem('admin_target_user_id');
    sessionStorage.removeItem('admin_target_user_id');
    setIsAuthenticated(false);
    setCurrentUser(null);
    setIsLogoutModalOpen(false);
    setSelectedPlatform('rocnovel');
    setTargetUserId(1);
    setUsersList([]);
    setPlatformsList([]);
    setData([]);
    setDistributionData([]);
    setDistributionSummary(null);
    setGlobalDistributionData([]);
    setGlobalDistributionSummary(null);
    setActiveTab('ltv');
    window.location.hash = '#/ltv';
    showToast('已安全退出登录', 'info');
  };

  // 1. 仅抓取/同步订单
  const handleSyncOrdersOnly = async (startTime, endTime, platCode) => {
    setLoading(true);
    setLoadingType('orders');
    setErrorMessage('');
    const p = platCode || selectedPlatform || 'rocnovel';
    try {
      const res = await authFetch(`/api/ltv/sync-orders?platformCode=${encodeURIComponent(p)}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ startTime, endTime, platformCode: p }),
      });
      const json = await res.json();
      if (json.code === 0) {
        setIsSyncModalOpen(false);
        showToast(`[${p}] 订单同步成功 (${startTime} ~ ${endTime})，共抓取 ${json.totalSyncedOrders} 笔订单！`, 'success');
      } else if (json.code === 4002) {
        setErrorMessage(json.msg || '订单接口 Token 已过期');
        setIsSyncModalOpen(false);
        setIsTokenModalOpen(true);
        showToast('Token 已过期，请更新 API 鉴权凭证', 'error');
      } else {
        showToast(`提示: ${json.msg}`, 'warning');
      }
    } catch (err) {
      if (err.message !== 'UNAUTHORIZED') {
        showToast('抓取订单发生网络异常', 'error');
      }
    } finally {
      setLoading(false);
      setLoadingType(null);
    }
  };

  // 2. 仅重算 LTV & 充值分析全量报表
  const handleRecalculateAllReports = async (platCode) => {
    setLoading(true);
    setLoadingType('calc');
    const p = platCode || selectedPlatform || 'rocnovel';
    try {
      const res = await authFetch(`/api/ltv/recalculate?targetUserId=${targetUserId || ''}&platformCode=${encodeURIComponent(p)}`, { method: 'POST' });
      const json = await res.json();
      if (json.code === 0) {
        setIsSyncModalOpen(false);
        fetchLtvData(targetUserId, selectedPlatform);
        fetchDistributionData(targetUserId, selectedPlatform);
        if (hasPermGlobalDistribution) {
          fetchGlobalDistributionData(selectedPlatform);
        }
        showToast(`[${p}] LTV 与 充值分析全量报表重算完成！`, 'success');
      } else {
        showToast(`提示: ${json.msg}`, 'warning');
      }
    } catch (err) {
      if (err.message !== 'UNAUTHORIZED') {
        showToast('重算报表请求失败', 'error');
      }
    } finally {
      setLoading(false);
      setLoadingType(null);
    }
  };

  // 4. 一键抓取订单 + 重算全量报表
  const handleSyncAndCalcAll = async (startTime, endTime, platCode) => {
    setLoading(true);
    setLoadingType('all');
    setErrorMessage('');
    const p = platCode || selectedPlatform || 'rocnovel';
    try {
      const res = await authFetch(`/api/ltv/sync-and-calc?platformCode=${encodeURIComponent(p)}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ startTime, endTime, platformCode: p }),
      });
      const json = await res.json();
      if (json.code === 0) {
        setIsSyncModalOpen(false);
        fetchLtvData(targetUserId, selectedPlatform);
        fetchDistributionData(targetUserId, selectedPlatform);
        if (hasPermGlobalDistribution) {
          fetchGlobalDistributionData(selectedPlatform);
        }
        showToast(`[${p}] 全流程抓取与重算完成！`, 'success');
      } else if (json.code === 4002) {
        setErrorMessage(json.msg);
        setIsSyncModalOpen(false);
        setIsTokenModalOpen(true);
        showToast('Token 已过期，请更新 API 鉴权凭证', 'error');
      } else {
        showToast(`提示: ${json.msg}`, 'warning');
      }
    } catch (err) {
      if (err.message !== 'UNAUTHORIZED') {
        showToast('全流程请求发生网络异常', 'error');
      }
    } finally {
      setLoading(false);
      setLoadingType(null);
    }
  };

  const handleSpendSaved = () => {
    fetchLtvData(targetUserId, selectedPlatform);
    showToast('投放消耗与备注更新成功！', 'success');
  };

  const handleBatchSpendSaved = (count) => {
    fetchLtvData(targetUserId, selectedPlatform);
    showToast(`批量导入成功！共写入/更新 ${count} 条消耗数据`, 'success');
  };

  const handleLandingPagesSaved = () => {
    if (activeTab === 'ltv') fetchLtvData(targetUserId, selectedPlatform);
    if (activeTab === 'distribution') fetchDistributionData(targetUserId, selectedPlatform);
    if (currentUser?.role === 'ADMIN') fetchUsersList();
    showToast('落地页配置保存成功，已完成专属报表实时重算！', 'success');
  };

  const currentPlatformObj = platformsList?.find(p => p.code?.toLowerCase() === (selectedPlatform || 'rocnovel').toLowerCase());
  const currentPlatformLaunchDate = currentPlatformObj?.launchStartDate || (selectedPlatform?.toLowerCase() === 'flicknovel' ? '2026-09-16' : '2026-07-10');

  // 获取平台对应今日的标准日期格式 (YYYY-MM-DD)
  const getPlatformTodayStr = (platformCode) => {
    const isFlick = (platformCode || '').toLowerCase() === 'flicknovel';
    const tz = isFlick ? 'UTC' : 'Asia/Shanghai';
    try {
      const formatter = new Intl.DateTimeFormat('zh-CN', {
        timeZone: tz,
        year: 'numeric',
        month: '2-digit',
        day: '2-digit'
      });
      const parts = formatter.formatToParts(new Date());
      const y = parts.find(p => p.type === 'year')?.value;
      const m = parts.find(p => p.type === 'month')?.value;
      const d = parts.find(p => p.type === 'day')?.value;
      if (y && m && d) return `${y}-${m}-${d}`;
    } catch (e) {
      // fallback
    }
    const now = new Date();
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
  };

  const currentPlatformToday = getPlatformTodayStr(selectedPlatform);

  // 严格从开始投放日期显示到今天 (无论当天是否有订单数据均展示完整 Cohort 行)
  const displayedLtvData = React.useMemo(() => {
    if (!currentPlatformLaunchDate) return data;
    const startStr = currentPlatformLaunchDate;
    const endStr = currentPlatformToday;
    if (startStr > endStr) return data;

    const dataMap = new Map();
    if (Array.isArray(data)) {
      data.forEach(item => {
        if (item && item.launchDate) {
          dataMap.set(item.launchDate, item);
        }
      });
    }

    const result = [];
    let curr = new Date(startStr + 'T00:00:00');
    const end = new Date(endStr + 'T00:00:00');

    while (curr <= end) {
      const y = curr.getFullYear();
      const m = String(curr.getMonth() + 1).padStart(2, '0');
      const d = String(curr.getDate()).padStart(2, '0');
      const dateKey = `${y}-${m}-${d}`;

      if (dataMap.has(dateKey)) {
        result.push(dataMap.get(dateKey));
      } else {
        result.push({
          launchDate: dateKey,
          platformCode: selectedPlatform || 'rocnovel',
          userId: targetUserId || currentUser?.userId,
          spend: 0,
          remark: '',
          totalRecharge: 0,
          totalRefund: 0,
          totalProfit: 0,
          totalRoi: 0,
          subUserCount: 0,
          subUserCost: 0,
          day7RetentionCount: null,
          day7RetentionRate: null,
          day15RetentionCount: null,
          day15RetentionRate: null,
          predictedPaybackDays: null,
        });
      }
      curr.setDate(curr.getDate() + 1);
    }
    return result;
  }, [data, currentPlatformLaunchDate, currentPlatformToday, selectedPlatform, targetUserId, currentUser]);

  if (!isAuthenticated) {
    return (
      <>
        <Toast toast={toast} onClose={() => setToast(null)} />
        <Login onLoginSuccess={handleLoginSuccess} />
      </>
    );
  }

  const totalSpend = displayedLtvData.reduce((acc, cur) => acc + (parseFloat(cur.spend) || 0), 0);
  const totalRecharge = displayedLtvData.reduce((acc, cur) => acc + (parseFloat(cur.totalRecharge) || 0), 0);
  const totalRefund = displayedLtvData.reduce((acc, cur) => acc + (parseFloat(cur.totalRefund) || 0), 0);
  const totalProfit = totalRecharge - totalRefund - totalSpend;
  const totalSubUsers = displayedLtvData.reduce((acc, cur) => acc + (parseInt(cur.subUserCount) || 0), 0);
  const overallRoi = totalSpend > 0 ? (((totalRecharge - totalRefund) / totalSpend) * 100).toFixed(2) : '0.00';

  // 月度卡片指标完全由后端接口计算并返回 (monthlySummary)，支持近6个月动态列表
  const monthlyList = Array.isArray(monthlySummary?.months) && monthlySummary.months.length > 0
    ? monthlySummary.months
    : (monthlySummary?.thisMonth ? [monthlySummary.thisMonth, ...(monthlySummary.lastMonth ? [monthlySummary.lastMonth] : [])] : []);

  const calculateOverallPaybackDays = (rows) => {
    if (!rows || rows.length === 0) return null;
    const validRows = rows.filter(r => parseFloat(r.spend || 0) > 0);
    if (validRows.length === 0) return null;

    const spendSum = validRows.reduce((acc, r) => acc + parseFloat(r.spend || 0), 0);
    const rechargeSum = validRows.reduce((acc, r) => acc + parseFloat(r.totalRecharge || 0), 0);

    if (spendSum > 0 && rechargeSum >= spendSum) {
      return 0; // 已回本
    }

    const xList = [];
    const yList = [];

    for (let t = 1; t <= 30; t++) {
      let cohortSpendSum = 0;
      let cohortRechargeSum = 0;
      let count = 0;

      validRows.forEach(r => {
        const val = r[`day${t}Recharge`];
        if (val !== null && val !== undefined) {
          cohortSpendSum += parseFloat(r.spend || 0);
          cohortRechargeSum += parseFloat(val);
          count++;
        }
      });

      if (count > 0 && cohortSpendSum > 0 && cohortRechargeSum > 0) {
        const roi = cohortRechargeSum / cohortSpendSum;
        xList.push(Math.log(t));
        yList.push(roi);
      }
    }

    if (yList.length < 3) return null;

    const n = xList.length;
    let sumX = 0, sumY = 0, sumXY = 0, sumXX = 0;
    for (let i = 0; i < n; i++) {
      sumX += xList[i];
      sumY += yList[i];
      sumXY += xList[i] * yList[i];
      sumXX += xList[i] * xList[i];
    }

    const denominator = n * sumXX - sumX * sumX;
    if (denominator === 0) return -1;

    const a = (n * sumXY - sumX * sumY) / denominator;
    const b = (sumY - a * sumX) / n;

    if (a <= 0.0001) return -1; // 停滞

    const tPayback = Math.exp((1.0 - b) / a);
    if (tPayback > 0 && tPayback <= 365) {
      return Math.round(tPayback);
    }

    return 366; // >365天
  };

  const overallPaybackDays = backendOverallPaybackDays !== null ? backendOverallPaybackDays : calculateOverallPaybackDays(displayedLtvData);

  const calculateOverallPaybackCycleDays = () => {
    if (backendOverallPaybackCycleDays !== null && backendOverallPaybackCycleDays !== undefined) {
      return backendOverallPaybackCycleDays;
    }
    if (!displayedLtvData || displayedLtvData.length === 0 || overallPaybackDays === null || overallPaybackDays < 0 || overallPaybackDays > 365) {
      return null;
    }
    const validDates = displayedLtvData.filter(d => d.launchDate && d.spend > 0).map(d => new Date(d.launchDate));
    if (validDates.length === 0) return null;
    const minDate = new Date(Math.min(...validDates));
    const today = new Date();
    const elapsedDays = Math.max(1, Math.floor((today - minDate) / (1000 * 60 * 60 * 24)) + 1);
    return elapsedDays + overallPaybackDays;
  };

  const overallPaybackCycleDays = calculateOverallPaybackCycleDays();
  const currentTargetUserObj = usersList.find(u => u.id === (targetUserId || currentUser?.userId));
  const isTargetMaster = currentTargetUserObj ? Boolean(currentTargetUserObj.isMaster === 1) : false;
  const isReadOnlyView = Boolean(targetUserId && currentUser && targetUserId !== currentUser.userId) || isTargetMaster;

  const formatMonthDisplay = (monthStr) => {
    if (!monthStr) return '';
    const clean = String(monthStr).trim();
    const parts = clean.split(/[-/]/);
    if (parts.length === 2) {
      const year = parts[0];
      const month = parts[1].padStart(2, '0');
      const yy = year.length === 4 ? year.slice(2) : year;
      return `${yy}/${month}`;
    }
    return clean;
  };

  const renderActualPaybackTag = (days, monthStr, d30Roi, d60Roi, d90Roi) => {
    if (days === null || days === undefined) return null;
    const hasPred = hasPermRoiPredict && d30Roi !== null && d30Roi !== undefined;
    return (
      <span
        onMouseEnter={(e) => {
          if (!hasPred) return;
          const rect = e.currentTarget.getBoundingClientRect();
          setHoveredMonthlyPrediction({
            left: rect.left + rect.width / 2,
            top: rect.top - 8,
            month: formatMonthDisplay(monthStr),
            d30Roi,
            d60Roi,
            d90Roi
          });
        }}
        onMouseLeave={() => setHoveredMonthlyPrediction(null)}
        style={{
          background: 'rgba(16, 185, 129, 0.15)',
          color: '#10b981',
          border: '1px solid rgba(16, 185, 129, 0.3)',
          padding: '0.08rem 0.32rem',
          borderRadius: '0.25rem',
          fontSize: '0.72rem',
          fontWeight: 600,
          whiteSpace: 'nowrap',
          cursor: hasPred ? 'pointer' : 'default'
        }}
      >
        回本：{days}天
      </span>
    );
  };

  const formatUsd = (val) => {
    const num = parseFloat(val || 0);
    if (num < 0) {
      return `-$${Math.abs(num).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
    }
    return `$${num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  return (
    <div className="app-layout">
      {/* 现代 Toast 全局消息浮层 */}
      <Toast toast={toast} onClose={() => setToast(null)} />

      {/* 左侧常驻侧边栏 (默认展开，顶部包含 Logo 与 Meta-LTV，点击 Logo 展开/收缩) */}
      <AppSidebar
        activeTab={activeTab}
        onTabChange={setActiveTab}
        loading={loading}
        onOpenSyncModal={() => setIsSyncModalOpen(true)}
        onOpenBatchSpend={() => {
          if ((selectedPlatform || '').toUpperCase() === 'ALL') {
            showToast('大盘数据不可直接导入，请先切换至具体平台', 'warning');
            return;
          }
          setIsBatchSpendOpen(true);
        }}
        onOpenConfig={() => {
          setEditingTargetUserLandingPage(null);
          setIsConfigOpen(true);
        }}
        onOpenExportModal={() => setIsExportModalOpen(true)}
        onOpenUserManagement={() => setActiveTab('users')}
        onOpenTokenModal={() => setIsTokenModalOpen(true)}
        onLogout={() => setIsLogoutModalOpen(true)}
        currentUser={currentUser}
        isReadOnly={isReadOnlyView}
        isExpanded={isSidebarExpanded}
        onToggleSidebar={toggleSidebar}
        isMobileMenuOpen={isMobileMenuOpen}
        setIsMobileMenuOpen={setIsMobileMenuOpen}
      />

      {/* 右侧主视口：顶部上下文栏 + 主体内容区 */}
      <div className="app-main-viewport">
        <AppTopBar
          activeTab={activeTab}
          onTabChange={setActiveTab}
          selectedPlatform={selectedPlatform}
          platformsList={platformsList}
          onSelectPlatform={handleSelectPlatform}
          usersList={usersList}
          targetUserId={targetUserId}
          currentUser={currentUser}
          onSelectTargetUser={handleSelectTargetUser}
          isReadOnly={isReadOnlyView}
          onToggleMobileMenu={() => setIsMobileMenuOpen(prev => !prev)}
          onLogout={() => setIsLogoutModalOpen(true)}
        />

        <main className="main-content">
        {errorMessage && (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', background: 'rgba(244, 63, 94, 0.15)', border: '1px solid #f43f5e', color: '#fda4af', padding: '0.75rem 1rem', borderRadius: '0.5rem', fontSize: '0.875rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <AlertTriangle size={18} color="#f43f5e" />
              <span><strong>提示：</strong> {errorMessage}</span>
            </div>
            <button className="btn btn-primary" style={{ padding: '0.25rem 0.75rem', fontSize: '0.8rem' }} onClick={() => setIsTokenModalOpen(true)}>
              去更新 Token
            </button>
          </div>
        )}

        {/* Tab 1: LTV 报表 */}
        {activeTab === 'ltv' && (
          <>
            <div className="stats-summary">
              {/* 卡片 1: 总消耗 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>总消耗 ({currentPlatformLaunchDate}至今)</span>
                  <DollarSign size={16} color="var(--text-sub)" />
                </div>
                <div className="stat-value" style={{ fontSize: '1.12rem' }}>{formatUsd(totalSpend)}</div>
              </div>

              {/* 卡片 2: 累计充值 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>累计充值</span>
                  <Wallet size={16} color="var(--text-sub)" />
                </div>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: '0.35rem', marginTop: '0.1rem' }}>
                  <span className="stat-value" style={{ fontSize: '1.12rem' }}>{formatUsd(totalRecharge)}</span>
                  <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', fontWeight: 500 }}>
                    （退款：{formatUsd(totalRefund)}）
                  </span>
                </div>
              </div>

              {/* 卡片 3: 总 ROI / 盈亏 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>总 ROI / 盈亏</span>
                  <TrendingUp size={16} color={overallRoi >= 100 ? '#10b981' : '#f43f5e'} />
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'nowrap', whiteSpace: 'nowrap' }}>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: '0.35rem' }}>
                    <span className="stat-value" style={{ fontSize: '1.12rem', color: overallRoi >= 100 ? '#10b981' : '#f43f5e' }}>
                      {overallRoi}%
                    </span>
                    <span style={{ fontSize: '0.82rem', color: 'var(--text-sub)', fontWeight: 400, opacity: 0.6 }}>/</span>
                    <span style={{ fontSize: '0.86rem', fontWeight: 600, color: totalProfit >= 0 ? '#10b981' : '#f43f5e', whiteSpace: 'nowrap' }}>
                      {formatUsd(totalProfit)}
                    </span>
                  </div>
                  {overallPaybackDays === 0 || overallRoi >= 100 ? (
                    <span style={{ background: 'rgba(16, 185, 129, 0.15)', color: '#10b981', border: '1px solid rgba(16, 185, 129, 0.3)', padding: '0.08rem 0.32rem', borderRadius: '0.25rem', fontSize: '0.72rem', fontWeight: 600, whiteSpace: 'nowrap' }}>
                      已回本{overallPaybackCycleDays ? ` / 周期：${overallPaybackCycleDays}天` : ''}
                    </span>
                  ) : hasPermPredictPayback && (
                    overallPaybackDays === -1 ? (
                      <span style={{ background: 'rgba(244, 63, 94, 0.15)', color: '#f43f5e', border: '1px solid rgba(244, 63, 94, 0.3)', padding: '0.08rem 0.32rem', borderRadius: '0.25rem', fontSize: '0.72rem', fontWeight: 600, whiteSpace: 'nowrap' }}>
                        回本：停滞
                      </span>
                    ) : overallPaybackDays > 365 ? (
                      <span style={{ background: 'rgba(244, 63, 94, 0.15)', color: '#f43f5e', border: '1px solid rgba(244, 63, 94, 0.3)', padding: '0.08rem 0.32rem', borderRadius: '0.25rem', fontSize: '0.72rem', fontWeight: 600, whiteSpace: 'nowrap' }}>
                        回本：&gt;365天
                      </span>
                    ) : overallPaybackDays !== null ? (
                      <span style={{ background: overallPaybackDays <= 45 ? 'rgba(16, 185, 129, 0.15)' : overallPaybackDays <= 90 ? 'rgba(245, 158, 11, 0.15)' : 'rgba(99, 102, 241, 0.15)', color: overallPaybackDays <= 45 ? '#10b981' : overallPaybackDays <= 90 ? '#f59e0b' : '#6366f1', border: '1px solid currentColor', padding: '0.08rem 0.32rem', borderRadius: '0.25rem', fontSize: '0.72rem', fontWeight: 600, whiteSpace: 'nowrap' }}>
                        回本：{overallPaybackDays}天{overallPaybackCycleDays ? ` / 周期：${overallPaybackCycleDays}天` : ''}
                      </span>
                    ) : null
                  )}
                </div>
              </div>

              {/* 卡片 4: 总订阅用户 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>总订阅用户</span>
                  <Users size={16} color="#10b981" />
                </div>
                <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', flexWrap: 'nowrap', whiteSpace: 'nowrap', marginTop: '0.2rem' }}>
                  <span className="stat-value" style={{ fontSize: '1.12rem' }}>{totalSubUsers}人</span>
                  {overallRetainedSubUsers !== undefined && (
                    <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', fontWeight: 500 }}>
                      （留存：{overallRetainedSubUsers}人 / {overallRetainedRate}）
                    </span>
                  )}
                </div>
              </div>

              {/* 卡片 5: 月度消耗 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>月度消耗</span>
                  <DollarSign size={16} color="var(--text-sub)" />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', marginTop: '0.2rem' }}>
                  {monthlyList.map((m, idx) => (
                    <div key={m.month || idx} style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', borderTop: idx > 0 ? '1px dashed var(--border-color)' : 'none', paddingTop: idx > 0 ? '0.25rem' : '0' }}>
                      <span style={{ fontSize: '0.78rem', color: 'var(--text-sub)', fontWeight: 500 }}>{formatMonthDisplay(m.month)}</span>
                      <span style={{ fontSize: '0.98rem', fontWeight: 500, color: 'var(--text-main)' }}>{formatUsd(m.spend)}</span>
                    </div>
                  ))}
                  {monthlyList.length === 0 && (
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', textAlign: 'center', padding: '0.5rem 0' }}>暂无月度数据</div>
                  )}
                </div>
              </div>

              {/* 卡片 6: 月度充值 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>月度充值</span>
                  <Wallet size={16} color="var(--text-sub)" />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', marginTop: '0.2rem' }}>
                  {monthlyList.map((m, idx) => (
                    <div key={m.month || idx} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: idx > 0 ? '1px dashed var(--border-color)' : 'none', paddingTop: idx > 0 ? '0.25rem' : '0', flexWrap: 'nowrap' }}>
                      <span style={{ fontSize: '0.78rem', color: 'var(--text-sub)', fontWeight: 500 }}>{formatMonthDisplay(m.month)}</span>
                      <div style={{ display: 'flex', alignItems: 'baseline', gap: '0.25rem' }}>
                        <span style={{ fontSize: '0.98rem', fontWeight: 500, color: 'var(--text-main)' }}>{formatUsd(m.recharge)}</span>
                        <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', fontWeight: 500 }}>
                          （退款：{formatUsd(m.refund)}）
                        </span>
                      </div>
                    </div>
                  ))}
                  {monthlyList.length === 0 && (
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', textAlign: 'center', padding: '0.5rem 0' }}>暂无月度数据</div>
                  )}
                </div>
              </div>

              {/* 卡片 7: 月度 ROI / 盈亏 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>月度 ROI / 盈亏</span>
                  <TrendingUp size={16} color={monthlyList.length > 0 && parseFloat(monthlyList[0].roi || 0) >= 100 ? '#10b981' : '#f43f5e'} />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', marginTop: '0.2rem' }}>
                  {monthlyList.map((m, idx) => {
                    const roiVal = m.roi !== undefined ? m.roi : '0.00';
                    const profitVal = m.profit || 0;
                    const isPaidBack = m.actualPaybackDays !== null && m.actualPaybackDays !== undefined;
                    const hasPred = hasPermRoiPredict && m.predictedDay30Roi !== null && m.predictedDay30Roi !== undefined;
                    return (
                      <div key={m.month || idx} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: idx > 0 ? '1px dashed var(--border-color)' : 'none', paddingTop: idx > 0 ? '0.25rem' : '0', flexWrap: 'nowrap' }}>
                        <span style={{ fontSize: '0.78rem', color: 'var(--text-sub)', fontWeight: 500 }}>{formatMonthDisplay(m.month)}</span>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                          <span style={{ fontSize: '0.98rem', fontWeight: 500, color: roiVal >= 100 ? '#10b981' : '#f43f5e' }}>
                            {roiVal}%
                          </span>
                          <span style={{ fontSize: '0.75rem', color: 'var(--text-sub)', fontWeight: 400, opacity: 0.6 }}>/</span>
                          <span style={{ fontSize: '0.80rem', fontWeight: 600, color: profitVal >= 0 ? '#10b981' : '#f43f5e' }}>
                            {formatUsd(profitVal)}
                          </span>
                          {renderActualPaybackTag(m.actualPaybackDays, m.month, m.predictedDay30Roi, m.predictedDay60Roi, m.predictedDay90Roi)}
                          {hasPermRoiPredict && !isPaidBack && hasPred && (
                            <span
                              onMouseEnter={(e) => {
                                const rect = e.currentTarget.getBoundingClientRect();
                                setHoveredMonthlyPrediction({
                                  left: rect.left + rect.width / 2,
                                  top: rect.top - 8,
                                  month: formatMonthDisplay(m.month),
                                  d30Roi: m.predictedDay30Roi,
                                  d60Roi: m.predictedDay60Roi,
                                  d90Roi: m.predictedDay90Roi
                                });
                              }}
                              onMouseLeave={() => setHoveredMonthlyPrediction(null)}
                              style={{
                                background: 'rgba(99, 102, 241, 0.15)',
                                color: '#818cf8',
                                border: '1px solid rgba(99, 102, 241, 0.3)',
                                padding: '0.08rem 0.32rem',
                                borderRadius: '0.25rem',
                                fontSize: '0.72rem',
                                fontWeight: 600,
                                whiteSpace: 'nowrap',
                                cursor: 'pointer'
                              }}
                            >
                              🔮 ROI预测
                            </span>
                          )}
                        </div>
                      </div>
                    );
                  })}
                  {monthlyList.length === 0 && (
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', textAlign: 'center', padding: '0.5rem 0' }}>暂无月度数据</div>
                  )}
                </div>
              </div>

              {/* 卡片 8: 月度订阅用户 */}
              <div className="stat-card">
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span className="stat-label" style={{ fontSize: '0.78rem' }}>月度订阅用户</span>
                  <Users size={16} color="#10b981" />
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', marginTop: '0.2rem' }}>
                  {monthlyList.map((m, idx) => (
                    <div key={m.month || idx} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: idx > 0 ? '1px dashed var(--border-color)' : 'none', paddingTop: idx > 0 ? '0.25rem' : '0', flexWrap: 'nowrap' }}>
                      <span style={{ fontSize: '0.78rem', color: 'var(--text-sub)', fontWeight: 500 }}>{formatMonthDisplay(m.month)}</span>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.2rem' }}>
                        <span style={{ fontSize: '0.98rem', fontWeight: 500, color: 'var(--text-main)' }}>{m.subUsers || 0}人</span>
                        {m.retainedSubUsers !== undefined && (
                          <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', fontWeight: 500 }}>
                            （留存：{m.retainedSubUsers}人 / {m.retainedRate || '0.00%'}）
                          </span>
                        )}
                      </div>
                    </div>
                  ))}
                  {monthlyList.length === 0 && (
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', textAlign: 'center', padding: '0.5rem 0' }}>暂无月度数据</div>
                  )}
                </div>
              </div>
            </div>

            <LtvTable
              data={displayedLtvData}
              selectedPlatform={selectedPlatform}
              onEditRow={(row) => {
                if ((selectedPlatform || '').toUpperCase() === 'ALL') {
                  showToast('大盘数据不可直接编辑，请先切换至具体平台', 'warning');
                  return;
                }
                if (isReadOnlyView) {
                  const msgText = isTargetMaster
                    ? '主账号为数据汇总账号，消耗由子账号自动累加计算，不可直接修改'
                    : '只读视图模式下不可修改他人账户的消耗与备注';
                  showToast(msgText, 'warning');
                  return;
                }
                setEditingRow(row);
              }}
              isReadOnly={isReadOnlyView}
              isAdmin={currentUser?.role === 'ADMIN' || currentUser?.role === 'SUPER_ADMIN'}
              isSuperAdmin={isSuperAdmin}
              hasPermPredictPayback={hasPermPredictPayback}
              hasPermRoiPredict={hasPermRoiPredict}
            />
          </>
        )}

        {/* Tab 2: 每日充值分布 */}
        {activeTab === 'distribution' && (
          <DailyRechargeDistributionTable
            distributionData={distributionData}
            distributionSummary={distributionSummary}
          />
        )}

        {/* Tab 3: 充值分析 (总) - 平台全量订单汇总 */}
        {activeTab === 'global-distribution' && (
          <DailyRechargeDistributionTable
            distributionData={globalDistributionData}
            distributionSummary={globalDistributionSummary}
            isGlobal={true}
          />
        )}

        {/* Tab 4: 月份结算 */}
        {activeTab === 'settlement' && (
          <MonthlySettlementTable
            token={localStorage.getItem('admin_token')}
            currentUser={currentUser}
            showToast={showToast}
            selectedPlatform={selectedPlatform}
          />
        )}

        {/* Tab 5: 用户管理 (全屏列表页) */}
        {activeTab === 'users' && isSuperAdmin && (
          <UserManagementPage
            token={localStorage.getItem('admin_token')}
            currentUser={currentUser}
            onRefreshUsers={fetchUsersList}
            showToast={showToast}
          />
        )}
      </main>
      </div>

      {/* 弹窗组件 */}
      <LandingPageConfigModal
        isOpen={isConfigOpen}
        targetUser={editingTargetUserLandingPage}
        targetUserId={targetUserId}
        isReadOnly={isReadOnlyView}
        platformCode={selectedPlatform}
        platforms={platformsList}
        onClose={() => {
          setIsConfigOpen(false);
          setEditingTargetUserLandingPage(null);
        }}
        onSaved={handleLandingPagesSaved}
        authFetch={authFetch}
        currentUser={currentUser}
      />

      {currentUser && (
        <TokenConfigModal
          isOpen={isTokenModalOpen}
          onClose={() => setIsTokenModalOpen(false)}
          onSaved={() => {
            fetchLtvData();
            showToast('API Token 更新成功！', 'success');
          }}
          authFetch={authFetch}
        />
      )}

      <SyncModal
        isOpen={isSyncModalOpen}
        onClose={() => setIsSyncModalOpen(false)}
        onSyncOrders={handleSyncOrdersOnly}
        onRecalculateAllReports={handleRecalculateAllReports}
        onSyncAndCalcAll={handleSyncAndCalcAll}
        loading={loading}
        loadingType={loadingType}
        platforms={platformsList}
        selectedPlatform={selectedPlatform}
      />

      <BatchSpendModal
        isOpen={isBatchSpendOpen}
        targetUserId={targetUserId}
        platforms={platformsList}
        selectedPlatform={selectedPlatform}
        onClose={() => setIsBatchSpendOpen(false)}
        onSaved={handleBatchSpendSaved}
        authFetch={authFetch}
      />

      <EditSpendModal
        isOpen={!!editingRow}
        item={editingRow}
        targetUserId={targetUserId}
        platformCode={selectedPlatform}
        onClose={() => setEditingRow(null)}
        onSaved={handleSpendSaved}
        authFetch={authFetch}
      />

      <LogoutConfirmModal
        isOpen={isLogoutModalOpen}
        onClose={() => setIsLogoutModalOpen(false)}
        onConfirm={handleLogout}
        username={currentUser?.username}
      />

      <ExportModal
        isOpen={isExportModalOpen}
        onClose={() => setIsExportModalOpen(false)}
        onConfirmExport={handleConfirmExport}
        title={
          activeTab === 'ltv'
            ? '导出 LTV 统计报表'
            : activeTab === 'global-distribution'
              ? '导出平台充值汇总'
              : '导出每日充值分布报表'
        }
        maxDays={90}
        data={activeTab === 'ltv' ? displayedLtvData : distributionData}
        dateField={activeTab === 'ltv' ? 'launchDate' : 'date'}
      />

      {/* 鼠标悬浮“回本周期”或“ROI预测”按钮展示 D30, D60, D90 ROI 预测 Popover */}
      {hoveredMonthlyPrediction && (
        <div
          className="instant-prediction-popover"
          style={{
            position: 'fixed',
            left: `${hoveredMonthlyPrediction.left}px`,
            top: `${hoveredMonthlyPrediction.top}px`,
            transform: 'translate(-50%, -100%)',
            zIndex: 99999,
            pointerEvents: 'none',
            background: 'rgba(15, 23, 42, 0.94)',
            backdropFilter: 'blur(12px)',
            WebkitBackdropFilter: 'blur(12px)',
            border: '1px solid rgba(99, 102, 241, 0.35)',
            borderRadius: '0.65rem',
            padding: '0.75rem 0.95rem',
            boxShadow: '0 12px 30px rgba(0, 0, 0, 0.5), 0 0 15px rgba(99, 102, 241, 0.2)',
            minWidth: '230px',
            color: '#f8fafc'
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid rgba(255, 255, 255, 0.1)', paddingBottom: '0.4rem', marginBottom: '0.55rem' }}>
            <span style={{ fontSize: '0.82rem', fontWeight: 700, color: '#818cf8', display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
              🔮 上月 ROI 预测趋势
            </span>
            <span style={{ fontSize: '0.72rem', color: '#94a3b8' }}>{hoveredMonthlyPrediction.month}</span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.45rem', fontSize: '0.8rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ color: '#cbd5e1', fontWeight: 500 }}>D30 预测 ROI:</span>
              <span style={{ color: '#38bdf8', fontWeight: 700 }}>
                {hoveredMonthlyPrediction.d30Roi !== null && hoveredMonthlyPrediction.d30Roi !== undefined ? `${(parseFloat(hoveredMonthlyPrediction.d30Roi) * 100).toFixed(2)}%` : '-'}
              </span>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ color: '#cbd5e1', fontWeight: 500 }}>D60 预测 ROI:</span>
              <span style={{ color: '#818cf8', fontWeight: 700 }}>
                {hoveredMonthlyPrediction.d60Roi !== null && hoveredMonthlyPrediction.d60Roi !== undefined ? `${(parseFloat(hoveredMonthlyPrediction.d60Roi) * 100).toFixed(2)}%` : '-'}
              </span>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ color: '#cbd5e1', fontWeight: 500 }}>D90 预测 ROI:</span>
              <span style={{ color: '#c084fc', fontWeight: 700 }}>
                {hoveredMonthlyPrediction.d90Roi !== null && hoveredMonthlyPrediction.d90Roi !== undefined ? `${(parseFloat(hoveredMonthlyPrediction.d90Roi) * 100).toFixed(2)}%` : '-'}
              </span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
