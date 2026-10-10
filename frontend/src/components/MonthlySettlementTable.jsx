import React, { useState, useEffect, useMemo } from 'react';
import {
  Calculator,
  Globe,
  UserCheck,
  Unlink,
  Save,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  CheckCheck,
  DollarSign,
  Wallet,
  Percent,
  Check,
  Sparkles,
  ArrowRight,
  TrendingDown,
  Info,
  Users,
  Shield,
  Layers,
  Crown
} from 'lucide-react';
import CustomSelect from './CustomSelect';

export default function MonthlySettlementTable({ token, currentUser, showToast, selectedPlatform = 'ALL' }) {
  const isAdmin = currentUser && (currentUser.role === 'SUPER_ADMIN' || currentUser.role === 'ADMIN');

  const [settlementType, setSettlementType] = useState(isAdmin ? 'PLATFORM_ALL' : 'USER_ACCOUNT'); // 'PLATFORM_ALL' | 'USER_ACCOUNT' | 'UNLINKED_PID'
  const [accounts, setAccounts] = useState([]);
  const [selectedUserId, setSelectedUserId] = useState(isAdmin ? null : (currentUser?.userId || currentUser?.id));
  const [loading, setLoading] = useState(false);
  const [savingRowMonth, setSavingRowMonth] = useState(null);
  const [savingAll, setSavingAll] = useState(false);
  const [rows, setRows] = useState([]);
  const [dirtyMap, setDirtyMap] = useState({}); // { [monthStr]: boolean }

  useEffect(() => {
    if (!isAdmin) {
      setSettlementType('USER_ACCOUNT');
      const selfId = currentUser?.userId || currentUser?.id;
      if (selfId) setSelectedUserId(selfId);
    }
  }, [isAdmin, currentUser]);

  // 1. 获取可结算账号列表（管理员和超管获取所有，普通用户仅获取自身账号）
  const fetchAccounts = async () => {
    if (!token) return;
    try {
      const res = await fetch('/api/settlement/accounts', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      const data = await res.json();
      if (res.ok && data.code === 0 && Array.isArray(data.data)) {
        setAccounts(data.data);
        if (isAdmin) {
          if (data.data.length > 0) {
            setSelectedUserId(prev => {
              const exists = data.data.some(a => a.id === prev);
              return exists ? prev : data.data[0].id;
            });
          } else {
            setSelectedUserId(null);
          }
        } else {
          const selfId = currentUser?.userId || currentUser?.id;
          setSelectedUserId(selfId || (data.data.length > 0 ? data.data[0].id : null));
        }
      }
    } catch (e) {
      console.error('Failed to fetch settlement accounts:', e);
    }
  };

  useEffect(() => {
    fetchAccounts();
  }, [token, isAdmin]);

  // 2. 获取月度结算数据
  const fetchSettlementList = async (type = settlementType, uid = selectedUserId) => {
    if (!token) return;
    setLoading(true);
    try {
      const effectiveType = isAdmin ? type : 'USER_ACCOUNT';
      const selfId = currentUser?.userId || currentUser?.id;
      const effectiveUid = effectiveType === 'USER_ACCOUNT' ? (isAdmin ? (uid || selfId) : selfId) : '';
      const platParam = selectedPlatform ? `&platformCode=${encodeURIComponent(selectedPlatform)}` : '';
      const res = await fetch(`/api/settlement/list?settlementType=${effectiveType}&targetUserId=${effectiveUid || ''}${platParam}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      const data = await res.json();
      if (res.ok && data.code === 0 && Array.isArray(data.data)) {
        setRows(data.data);
        setDirtyMap({});
      } else {
        if (showToast) showToast(data.msg || '获取结算数据失败', 'error');
      }
    } catch (e) {
      if (showToast) showToast('获取结算数据发生网络异常', 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSettlementList(settlementType, selectedUserId);
  }, [settlementType, selectedUserId, isAdmin, selectedPlatform]);

  // 3. 处理单元格编辑与实时计算
  const handleCellChange = (monthStr, field, rawValue, displayVal, displayField) => {
    setRows(prevRows => {
      return prevRows.map(row => {
        if (row.monthStr !== monthStr) return row;

        const updated = { ...row, [field]: rawValue };
        if (displayField) {
          updated[displayField] = displayVal;
        }

        const totalRecharge = parseFloat(updated.totalRecharge) || 0;
        const totalRefund = parseFloat(updated.totalRefund) || 0;
        const settledRefund = parseFloat(updated.settledRefundAmount) || 0;
        const monthSettledRefund = parseFloat(updated.monthSettledRefundAmount) || 0;
        const crossRefund = parseFloat(updated.crossPeriodRefundAmount) || 0;
        const shareRatio = parseFloat(updated.shareRatio) || 0.95;
        const channelFeeRate = parseFloat(updated.channelFeeRate) || 0.07;

        // 未结算退款 = 累计退款 - 已结算退款
        const unsettledRefund = totalRefund - settledRefund;
        updated.unsettledRefundAmount = unsettledRefund.toFixed(2);

        // 有效结算基数 = 累计充值 - 当月结算退款 - 跨周期退款
        const effectiveBase = totalRecharge - monthSettledRefund - crossRefund;
        updated.effectiveBaseAmount = effectiveBase.toFixed(2);

        // 最终结算金额 = 有效基数 * 分成比例 * (1 - 渠道费率)
        if (effectiveBase > 0) {
          const finalAmt = effectiveBase * shareRatio * (1 - channelFeeRate);
          updated.finalSettlementAmount = finalAmt.toFixed(2);
        } else {
          const finalAmt = effectiveBase * shareRatio;
          updated.finalSettlementAmount = finalAmt.toFixed(2);
        }

        return updated;
      });
    });

    setDirtyMap(prev => ({ ...prev, [monthStr]: true }));
  };

  // 4. 保存单行月度结算参数
  const handleSaveRow = async (row) => {
    if (!token) return;
    setSavingRowMonth(row.monthStr);
    try {
      const selfId = currentUser?.userId || currentUser?.id;
      const payload = {
        platformCode: selectedPlatform || 'ALL',
        settlementType: isAdmin ? settlementType : 'USER_ACCOUNT',
        targetUserId: isAdmin ? (settlementType === 'USER_ACCOUNT' ? selectedUserId : null) : selfId,
        monthStr: row.monthStr,
        settledRefundAmount: parseFloat(row.settledRefundAmount) || 0,
        monthSettledRefundAmount: parseFloat(row.monthSettledRefundAmount) || 0,
        crossPeriodRefundAmount: parseFloat(row.crossPeriodRefundAmount) || 0,
        shareRatio: parseFloat(row.shareRatio) || 0.95,
        channelFeeRate: parseFloat(row.channelFeeRate) || 0.07,
        remark: row.remark || '',
      };

      const res = await fetch('/api/settlement/save', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(payload)
      });
      const data = await res.json();
      if (res.ok && data.code === 0) {
        if (showToast) showToast(`【${row.monthStr}】结算参数已成功保存！`, 'success');
        setDirtyMap(prev => ({ ...prev, [row.monthStr]: false }));
      } else {
        if (showToast) showToast(data.msg || '保存失败', 'error');
      }
    } catch (e) {
      if (showToast) showToast('保存结算数据异常', 'error');
    } finally {
      setSavingRowMonth(null);
    }
  };

  // 5. 一键保存所有被修改的行
  const dirtyMonths = useMemo(() => {
    return Object.keys(dirtyMap).filter(m => dirtyMap[m]);
  }, [dirtyMap]);

  const handleSaveAllDirtyRows = async () => {
    if (dirtyMonths.length === 0 || !token) return;
    setSavingAll(true);
    let successCount = 0;
    try {
      const selfId = currentUser?.userId || currentUser?.id;
      for (const m of dirtyMonths) {
        const row = rows.find(r => r.monthStr === m);
        if (!row) continue;

        const payload = {
          platformCode: selectedPlatform || 'ALL',
          settlementType: isAdmin ? settlementType : 'USER_ACCOUNT',
          targetUserId: isAdmin ? (settlementType === 'USER_ACCOUNT' ? selectedUserId : null) : selfId,
          monthStr: row.monthStr,
          settledRefundAmount: parseFloat(row.settledRefundAmount) || 0,
          monthSettledRefundAmount: parseFloat(row.monthSettledRefundAmount) || 0,
          crossPeriodRefundAmount: parseFloat(row.crossPeriodRefundAmount) || 0,
          shareRatio: parseFloat(row.shareRatio) || 0.95,
          channelFeeRate: parseFloat(row.channelFeeRate) || 0.07,
          remark: row.remark || '',
        };

        const res = await fetch('/api/settlement/save', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
          },
          body: JSON.stringify(payload)
        });
        if (res.ok) successCount++;
      }

      if (showToast) showToast(`成功保存 ${successCount} 个月份的结算配置！`, 'success');
      setDirtyMap({});
    } catch (e) {
      if (showToast) showToast('批量保存过程中遇到异常', 'error');
    } finally {
      setSavingAll(false);
    }
  };

  // 格式化金额展示
  const formatUsd = (val) => {
    const num = parseFloat(val || 0);
    if (num < 0) {
      return `-$${Math.abs(num).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
    }
    return `$${num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  // 汇总统计指标
  const totalRechargeSum = rows.reduce((acc, r) => acc + (parseFloat(r.totalRecharge) || 0), 0);
  const totalRefundSum = rows.reduce((acc, r) => acc + (parseFloat(r.totalRefund) || 0), 0);
  const totalSettledRefundSum = rows.reduce((acc, r) => acc + (parseFloat(r.settledRefundAmount) || 0), 0);
  const totalUnsettledRefundSum = rows.reduce((acc, r) => acc + (parseFloat(r.unsettledRefundAmount) || 0), 0);
  const totalFinalSettlementSum = rows.reduce((acc, r) => acc + (parseFloat(r.finalSettlementAmount) || 0), 0);
  const overallRefundRate = totalRechargeSum > 0 ? ((totalRefundSum / totalRechargeSum) * 100).toFixed(2) + '%' : '0.00%';

  // 当前真实自然月份 (e.g. "2026-09")
  const currentMonthStr = useMemo(() => {
    const d = new Date();
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    return `${y}-${m}`;
  }, []);

  // 历史月份已结算总额（不计算当月结算，历史月份结算总和）
  const historySettledSum = useMemo(() => {
    return rows
      .filter(r => r.monthStr < currentMonthStr)
      .reduce((acc, r) => acc + (parseFloat(r.finalSettlementAmount) || 0), 0);
  }, [rows, currentMonthStr]);

  const currentAccountObj = accounts.find(a => a.id === selectedUserId);

  return (
    <div className="settlement-container" style={{ display: 'flex', flexDirection: 'column', gap: '1.15rem', paddingBottom: '3.5rem' }}>

      {/* 顶部吸顶控制区：类型切换 Segment + (若为B则展示账号下拉选择器) + 批量保存 + 刷新按钮 */}
      <div style={{
        position: 'sticky',
        top: 0,
        zIndex: 100,
        background: 'var(--header-bg)',
        backdropFilter: 'blur(16px)',
        WebkitBackdropFilter: 'blur(16px)',
        padding: '0.85rem 1.25rem',
        borderRadius: '0.75rem',
        border: '1px solid var(--border-color)',
        boxShadow: 'var(--shadow-md)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        gap: '0.85rem'
      }}>
        {/* 左侧：三大类型分段按钮 + 账号选择下拉框 (超级管理员、管理员可见 A/B/C及所有账号；普通用户仅展示 账号分配结算 及自身账号) */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.85rem', flexWrap: 'wrap' }}>
          {isAdmin ? (
            <>
              <div style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.35rem',
                background: 'var(--bg-secondary)',
                padding: '0.3rem',
                borderRadius: '0.6rem',
                border: '1px solid var(--border-light)'
              }}>
                <button
                  onClick={() => setSettlementType('PLATFORM_ALL')}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.45rem',
                    padding: '0.45rem 0.95rem',
                    borderRadius: '0.45rem',
                    border: 'none',
                    cursor: 'pointer',
                    fontSize: '0.82rem',
                    fontWeight: settlementType === 'PLATFORM_ALL' ? 600 : 500,
                    background: settlementType === 'PLATFORM_ALL' ? 'linear-gradient(135deg, #3b82f6, #2563eb)' : 'transparent',
                    color: settlementType === 'PLATFORM_ALL' ? '#ffffff' : 'var(--text-sub)',
                    boxShadow: settlementType === 'PLATFORM_ALL' ? '0 2px 8px rgba(37, 99, 235, 0.35)' : 'none',
                    transition: 'all 0.2s ease'
                  }}
                >
                  <Globe size={15} />
                  <span>平台汇总</span>
                </button>

                <button
                  onClick={() => setSettlementType('USER_ACCOUNT')}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.45rem',
                    padding: '0.45rem 0.95rem',
                    borderRadius: '0.45rem',
                    border: 'none',
                    cursor: 'pointer',
                    fontSize: '0.82rem',
                    fontWeight: settlementType === 'USER_ACCOUNT' ? 600 : 500,
                    background: settlementType === 'USER_ACCOUNT' ? 'linear-gradient(135deg, #3b82f6, #2563eb)' : 'transparent',
                    color: settlementType === 'USER_ACCOUNT' ? '#ffffff' : 'var(--text-sub)',
                    boxShadow: settlementType === 'USER_ACCOUNT' ? '0 2px 8px rgba(37, 99, 235, 0.35)' : 'none',
                    transition: 'all 0.2s ease'
                  }}
                >
                  <UserCheck size={15} />
                  <span>账号分配结算</span>
                  {accounts.length > 0 && (
                    <span style={{
                      fontSize: '0.7rem',
                      padding: '0.1rem 0.35rem',
                      borderRadius: '0.3rem',
                      background: settlementType === 'USER_ACCOUNT' ? 'rgba(255,255,255,0.25)' : 'var(--bg-hover)',
                      color: settlementType === 'USER_ACCOUNT' ? '#ffffff' : 'var(--text-main)',
                      fontWeight: 700
                    }}>
                      {accounts.length}
                    </span>
                  )}
                </button>

                <button
                  onClick={() => setSettlementType('UNLINKED_PID')}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.45rem',
                    padding: '0.45rem 0.95rem',
                    borderRadius: '0.45rem',
                    border: 'none',
                    cursor: 'pointer',
                    fontSize: '0.82rem',
                    fontWeight: settlementType === 'UNLINKED_PID' ? 600 : 500,
                    background: settlementType === 'UNLINKED_PID' ? 'linear-gradient(135deg, #3b82f6, #2563eb)' : 'transparent',
                    color: settlementType === 'UNLINKED_PID' ? '#ffffff' : 'var(--text-sub)',
                    boxShadow: settlementType === 'UNLINKED_PID' ? '0 2px 8px rgba(37, 99, 235, 0.35)' : 'none',
                    transition: 'all 0.2s ease'
                  }}
                >
                  <Unlink size={15} />
                  <span>无关联落地页订单</span>
                </button>
              </div>

              {/* 当选择 账号分配结算时，账号选择下拉框直接置顶展示于控制栏 */}
              {settlementType === 'USER_ACCOUNT' && (
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.45rem', position: 'relative', zIndex: 1001 }}>
                  <span style={{ fontSize: '0.82rem', color: 'var(--text-sub)', fontWeight: 600, whiteSpace: 'nowrap' }}>
                    结算账号:
                  </span>
                  {accounts.length > 0 ? (
                    <CustomSelect
                      value={selectedUserId || ''}
                      onChange={(val) => setSelectedUserId(Number(val))}
                      options={accounts.map(a => ({
                        label: a.username,
                        value: a.id
                      }))}
                      style={{ minWidth: '180px' }}
                      placement="bottom"
                    />
                  ) : (
                    <span style={{ fontSize: '0.78rem', color: '#f59e0b' }}>（暂无参与结算的账号）</span>
                  )}
                </div>
              )}
            </>
          ) : (
            /* 普通用户：仅展示【账号分配结算】及自身登录账号 */
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: '0.65rem',
              background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.12), rgba(37, 99, 235, 0.06))',
              padding: '0.45rem 0.95rem',
              borderRadius: '0.55rem',
              border: '1px solid rgba(59, 130, 246, 0.25)',
              color: '#3b82f6',
              fontWeight: 600,
              fontSize: '0.84rem'
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                <UserCheck size={16} />
                <span>账号分配结算</span>
              </div>
              <span style={{ color: 'rgba(59, 130, 246, 0.3)', margin: '0 0.1rem' }}>|</span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', fontSize: '0.82rem', color: 'var(--text-sub)' }}>
                <span>结算账号:</span>
                <strong style={{ color: 'var(--text-main)', fontWeight: 700 }}>
                  {currentUser?.username || (accounts.length > 0 ? accounts[0].username : '')}
                </strong>
              </div>
            </div>
          )}
        </div>

        {/* 右侧快捷操作区 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          {dirtyMonths.length > 0 && (
            <button
              className="btn btn-primary"
              onClick={handleSaveAllDirtyRows}
              disabled={savingAll}
              style={{
                padding: '0.45rem 0.85rem',
                fontSize: '0.82rem',
                gap: '0.35rem',
                background: 'linear-gradient(135deg, #10b981, #059669)',
                boxShadow: '0 3px 10px rgba(16, 185, 129, 0.35)',
                borderColor: '#10b981'
              }}
            >
              <Save size={14} />
              <span>{savingAll ? '保存中...' : `保存全部修改 (${dirtyMonths.length})`}</span>
            </button>
          )}

          <button
            className="btn btn-secondary"
            onClick={() => fetchSettlementList(settlementType, selectedUserId)}
            disabled={loading}
            style={{ padding: '0.45rem 0.8rem', fontSize: '0.82rem', gap: '0.35rem' }}
          >
            <RefreshCw size={14} className={loading ? 'spin' : ''} />
            <span>刷新</span>
          </button>
        </div>
      </div>

      {/* 结算指标核心卡片区 (紧凑小尺寸 + 极简风格) */}
      <div className="stats-summary" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))', gap: '0.75rem' }}>

        {/* 卡片 1: 累计充值 */}
        <div className="stat-card" style={{
          background: 'var(--bg-card)',
          borderRadius: '0.6rem',
          padding: '0.75rem 1rem',
          border: '1px solid var(--border-color)',
          boxShadow: 'var(--shadow-sm)'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.35rem' }}>
            <span className="stat-label" style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-sub)' }}>累计充值</span>
            <div style={{ width: 26, height: 26, borderRadius: '0.375rem', background: 'rgba(59, 130, 246, 0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Wallet size={14} color="#3b82f6" />
            </div>
          </div>
          <div className="stat-value" style={{ fontSize: '1.12rem', fontWeight: 700, color: 'var(--text-main)', letterSpacing: '-0.01em' }}>
            {formatUsd(totalRechargeSum)}
          </div>
          <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', marginTop: '0.2rem', display: 'block' }}>
            全部有效订单充值收入汇总
          </span>
        </div>

        {/* 卡片 2: 累计退款 */}
        <div className="stat-card" style={{
          background: 'var(--bg-card)',
          borderRadius: '0.6rem',
          padding: '0.75rem 1rem',
          border: '1px solid var(--border-color)',
          boxShadow: 'var(--shadow-sm)'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.35rem' }}>
            <span className="stat-label" style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-sub)' }}>累计退款</span>
            <div style={{ width: 26, height: 26, borderRadius: '0.375rem', background: 'rgba(244, 63, 94, 0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <TrendingDown size={14} color="#f43f5e" />
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: '0.45rem' }}>
            <span className="stat-value" style={{ fontSize: '1.12rem', fontWeight: 700, color: '#f43f5e', letterSpacing: '-0.01em' }}>
              {formatUsd(totalRefundSum)}
            </span>
            <span style={{
              fontSize: '0.72rem',
              fontWeight: 600,
              color: '#f43f5e',
              background: 'rgba(244, 63, 94, 0.1)',
              padding: '0.08rem 0.35rem',
              borderRadius: '0.25rem'
            }}>
              退款率: {overallRefundRate}
            </span>
          </div>
          <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', marginTop: '0.2rem', display: 'block' }}>
            已结算退款: {formatUsd(totalSettledRefundSum)}
          </span>
        </div>

        {/* 卡片 3: 累计结算总额 (含当月在内的所有月份结算总额) */}
        <div className="stat-card" style={{
          background: 'var(--bg-card)',
          borderRadius: '0.6rem',
          padding: '0.75rem 1rem',
          border: '1px solid var(--border-color)',
          boxShadow: 'var(--shadow-sm)'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.35rem' }}>
            <span className="stat-label" style={{ fontSize: '0.78rem', fontWeight: 700, color: '#10b981', display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
              <Sparkles size={13} />
              累计结算总额
            </span>
            <div style={{ width: 26, height: 26, borderRadius: '0.375rem', background: 'rgba(16, 185, 129, 0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Calculator size={14} color="#10b981" />
            </div>
          </div>
          <div className="stat-value" style={{ fontSize: '1.12rem', fontWeight: 700, color: '#10b981', letterSpacing: '-0.01em' }}>
            {formatUsd(totalFinalSettlementSum)}
          </div>
          <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', marginTop: '0.2rem', display: 'block' }}>
            有效基数 × 分成比例 × (1 - 渠道费率)
          </span>
        </div>

        {/* 卡片 4: 已结算总额 (历史月份结算总和，不计算当月) */}
        <div className="stat-card" style={{
          background: 'var(--bg-card)',
          borderRadius: '0.6rem',
          padding: '0.75rem 1rem',
          border: '1px solid var(--border-color)',
          boxShadow: 'var(--shadow-sm)'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.35rem' }}>
            <span className="stat-label" style={{ fontSize: '0.78rem', fontWeight: 700, color: '#10b981', display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
              <CheckCheck size={13} />
              已结算总额
            </span>
            <div style={{ width: 26, height: 26, borderRadius: '0.375rem', background: 'rgba(16, 185, 129, 0.15)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <CheckCheck size={14} color="#10b981" />
            </div>
          </div>
          <div className="stat-value" style={{ fontSize: '1.12rem', fontWeight: 700, color: '#10b981', letterSpacing: '-0.01em' }}>
            {formatUsd(historySettledSum)}
          </div>
          <span style={{ fontSize: '0.72rem', color: 'var(--text-sub)', marginTop: '0.2rem', display: 'block' }}>
            历史月份结算总和 (不含当月)
          </span>
        </div>

        {/* 卡片 5: 未结算退款 */}
        <div className="stat-card" style={{
          background: 'var(--bg-card)',
          borderRadius: '0.6rem',
          padding: '0.75rem 1rem',
          border: '1px solid var(--border-color)',
          boxShadow: 'var(--shadow-sm)'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.35rem' }}>
            <span className="stat-label" style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-sub)' }}>未结算退款</span>
            <div style={{ width: 26, height: 26, borderRadius: '0.375rem', background: totalUnsettledRefundSum > 0 ? 'rgba(245, 158, 11, 0.12)' : 'rgba(16, 185, 129, 0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              {totalUnsettledRefundSum > 0 ? <AlertCircle size={14} color="#f59e0b" /> : <CheckCircle2 size={14} color="#10b981" />}
            </div>
          </div>
          <div className="stat-value" style={{ fontSize: '1.12rem', fontWeight: 700, color: totalUnsettledRefundSum > 0 ? '#f59e0b' : '#10b981', letterSpacing: '-0.01em' }}>
            {formatUsd(totalUnsettledRefundSum)}
          </div>
          <span style={{ fontSize: '0.72rem', color: totalUnsettledRefundSum > 0 ? '#f59e0b' : 'var(--text-sub)', marginTop: '0.2rem', display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
            {totalUnsettledRefundSum <= 0 ? '历史退款已全额结清' : '⚠️ 存在待结清退款差额'}
          </span>
        </div>
      </div>

      {/* 结算公式与逻辑说明提示 */}
      <div style={{
        background: 'var(--bg-secondary)',
        border: '1px solid var(--border-light)',
        borderRadius: '0.65rem',
        padding: '0.65rem 1.1rem',
        fontSize: '0.82rem',
        color: 'var(--text-sub)',
        display: 'flex',
        alignItems: 'center',
        gap: '0.75rem',
        lineHeight: '1.6',
        flexWrap: 'wrap'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem', color: 'var(--text-sub)', fontWeight: 600, flexShrink: 0 }}>
          <Percent size={15} />
          <span>核算公式：</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.45rem', flexWrap: 'wrap', fontSize: '0.82rem', color: 'var(--text-sub)', fontWeight: 500 }}>
          <span>结算金额 = (累计充值 - 当月结算退款 - 跨周期退款) × 分成比例 × ( 1 - 渠道费率 )</span>
        </div>
      </div>

      {/* 核心结算明细表格 (支持小屏幕横向自适应滚动) */}
      <div className="table-responsive" style={{
        background: 'var(--bg-card)',
        borderRadius: '0.75rem',
        border: '1px solid var(--border-color)',
        boxShadow: 'var(--shadow-md)',
        overflowX: 'auto',
        overflowY: 'hidden',
        maxWidth: '100%',
        WebkitOverflowScrolling: 'touch'
      }}>
        <table className="ltv-table" style={{ width: '100%', minWidth: isAdmin ? '1300px' : '1200px', borderCollapse: 'collapse', textAlign: 'left' }}>
          <thead>
            <tr style={{ background: 'var(--bg-th)', borderBottom: '1px solid var(--border-color)', whiteSpace: 'nowrap' }}>
              <th
                className="sticky-col col-boundary"
                style={{
                  position: 'sticky',
                  left: 0,
                  zIndex: 30,
                  background: 'var(--bg-th)',
                  minWidth: '95px',
                  textAlign: 'center',
                  padding: '0.85rem 0.6rem',
                  fontSize: '0.82rem',
                  fontWeight: 600,
                  color: 'var(--text-sub)',
                  whiteSpace: 'nowrap'
                }}
              >
                月份
              </th>
              <th style={{ minWidth: '95px', textAlign: 'right', padding: '0.85rem 0.6rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>累计充值</th>
              <th style={{ minWidth: '85px', textAlign: 'right', padding: '0.85rem 0.55rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>累计退款</th>
              <th style={{ minWidth: '82px', textAlign: 'right', padding: '0.85rem 0.45rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>已结算退款</th>
              <th style={{ minWidth: '82px', textAlign: 'center', padding: '0.85rem 0.45rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>未结算退款</th>
              <th style={{ minWidth: '85px', textAlign: 'right', padding: '0.85rem 0.45rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>当月结算退款</th>
              <th style={{ minWidth: '82px', textAlign: 'right', padding: '0.85rem 0.45rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>跨周期退款</th>
              <th style={{ minWidth: '82px', textAlign: 'right', padding: '0.85rem 0.5rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>分成比例</th>
              <th style={{ minWidth: '82px', textAlign: 'right', padding: '0.85rem 0.5rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>渠道费率</th>
              <th style={{
                minWidth: '120px',
                textAlign: 'right',
                padding: '0.85rem 0.75rem',
                fontSize: '0.84rem',
                fontWeight: 700,
                color: '#10b981',
                background: 'rgba(16, 185, 129, 0.08)',
                borderLeft: '1px solid rgba(16, 185, 129, 0.2)',
                borderRight: '1px solid rgba(16, 185, 129, 0.2)',
                whiteSpace: 'nowrap'
              }}>
                最终结算金额
              </th>
              <th style={{ minWidth: '68px', textAlign: 'right', padding: '0.85rem 0.4rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>退款率</th>
              <th style={{ minWidth: '105px', textAlign: 'right', padding: '0.85rem 0.5rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>充值/退款笔数</th>
              <th style={{ minWidth: '130px', textAlign: 'left', padding: '0.85rem 0.6rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>备注说明</th>
              {isAdmin && (
                <th style={{ minWidth: '85px', textAlign: 'center', padding: '0.85rem 0.5rem', fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', whiteSpace: 'nowrap' }}>操作</th>
              )}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={isAdmin ? 14 : 13} style={{ textAlign: 'center', padding: '3.5rem 1rem', color: 'var(--text-sub)' }}>
                  {loading ? (
                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.6rem' }}>
                      <RefreshCw size={24} className="spin" color="#3b82f6" />
                      <span>正在核算结算数据...</span>
                    </div>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.4rem' }}>
                      <Info size={24} color="var(--text-muted)" />
                      <span>暂无对应月份的结算记录</span>
                    </div>
                  )}
                </td>
              </tr>
            ) : (
              rows.map((row, idx) => {
                const isDirty = Boolean(dirtyMap[row.monthStr]);
                const isSaving = savingRowMonth === row.monthStr;
                const unsettledNum = parseFloat(row.unsettledRefundAmount) || 0;

                return (
                  <tr
                    key={row.monthStr}
                    style={{
                      background: isDirty
                        ? 'rgba(59, 130, 246, 0.04)'
                        : idx % 2 === 1 ? 'var(--bg-secondary)' : 'transparent',
                      borderBottom: '1px solid var(--border-color)',
                      transition: 'background-color 0.15s ease'
                    }}
                  >
                    {/* 1. 月份 */}
                    <td
                      className="sticky-col col-boundary"
                      style={{
                        position: 'sticky',
                        left: 0,
                        zIndex: 20,
                        background: isDirty ? 'rgba(59, 130, 246, 0.12)' : (idx % 2 === 1 ? 'var(--bg-secondary)' : 'var(--bg-sticky-td)'),
                        textAlign: 'center',
                        fontWeight: 700,
                        fontSize: '0.88rem',
                        color: 'var(--text-main)',
                        fontFamily: 'monospace',
                        padding: '0.65rem 0.6rem',
                        whiteSpace: 'nowrap'
                      }}
                    >
                      <div style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
                        {isDirty && (
                          <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#3b82f6', display: 'inline-block' }} title="有未保存修改" />
                        )}
                        <span>{row.monthStr}</span>
                      </div>
                    </td>

                    {/* 2. 总充值金额 */}
                    <td style={{
                      textAlign: 'right',
                      fontWeight: 600,
                      color: 'var(--text-main)',
                      fontFamily: 'monospace',
                      padding: '0.65rem 0.65rem'
                    }}>
                      {formatUsd(row.totalRecharge)}
                    </td>

                    {/* 3. 总退款金额 */}
                    <td style={{
                      textAlign: 'right',
                      color: parseFloat(row.totalRefund) > 0 ? '#f43f5e' : 'var(--text-sub)',
                      fontWeight: 600,
                      fontFamily: 'monospace',
                      padding: '0.65rem 0.6rem'
                    }}>
                      {formatUsd(row.totalRefund)}
                    </td>

                    {/* 4. 已结算退款金额 (管理员可编辑，普通用户只读) */}
                    <td style={{ textAlign: 'right', padding: '0.45rem 0.4rem' }}>
                      {isAdmin ? (
                        <input
                          type="number"
                          step="0.01"
                          min="0"
                          className="form-input"
                          value={row.settledRefundAmount !== undefined ? row.settledRefundAmount : 0}
                          onChange={(e) => handleCellChange(row.monthStr, 'settledRefundAmount', e.target.value)}
                          style={{
                            width: '76px',
                            textAlign: 'right',
                            padding: '0.28rem 0.35rem',
                            fontSize: '0.82rem',
                            fontFamily: 'monospace',
                            borderColor: isDirty ? '#3b82f6' : 'var(--border-color)',
                            background: 'var(--bg-input)',
                            borderRadius: '0.35rem'
                          }}
                          placeholder="0.00"
                        />
                      ) : (
                        <span style={{ fontFamily: 'monospace', fontSize: '0.84rem', color: 'var(--text-main)', fontWeight: 500 }}>
                          {formatUsd(row.settledRefundAmount)}
                        </span>
                      )}
                    </td>

                    {/* 5. 未结算退款 (系统核算标签) */}
                    <td style={{ textAlign: 'center', padding: '0.45rem 0.4rem' }}>
                      {unsettledNum <= 0 ? (
                        <span style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '0.25rem',
                          padding: '0.2rem 0.45rem',
                          borderRadius: '0.35rem',
                          fontSize: '0.74rem',
                          fontWeight: 600,
                          background: 'rgba(16, 185, 129, 0.12)',
                          color: '#10b981',
                          border: '1px solid rgba(16, 185, 129, 0.25)'
                        }}>
                          <CheckCircle2 size={12} />
                          已结清
                        </span>
                      ) : (
                        <span style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '0.25rem',
                          padding: '0.2rem 0.45rem',
                          borderRadius: '0.35rem',
                          fontSize: '0.74rem',
                          fontWeight: 600,
                          background: 'rgba(245, 158, 11, 0.12)',
                          color: '#f59e0b',
                          border: '1px solid rgba(245, 158, 11, 0.25)',
                          fontFamily: 'monospace'
                        }}>
                          {formatUsd(unsettledNum)}
                        </span>
                      )}
                    </td>

                    {/* 6. 当月结算退款金额 (管理员可编辑，普通用户只读) */}
                    <td style={{ textAlign: 'right', padding: '0.45rem 0.4rem' }}>
                      {isAdmin ? (
                        <input
                          type="number"
                          step="0.01"
                          min="0"
                          className="form-input"
                          value={row.monthSettledRefundAmount !== undefined ? row.monthSettledRefundAmount : 0}
                          onChange={(e) => handleCellChange(row.monthStr, 'monthSettledRefundAmount', e.target.value)}
                          style={{
                            width: '76px',
                            textAlign: 'right',
                            padding: '0.28rem 0.35rem',
                            fontSize: '0.82rem',
                            fontFamily: 'monospace',
                            borderColor: isDirty ? '#3b82f6' : 'var(--border-color)',
                            background: 'var(--bg-input)',
                            borderRadius: '0.35rem'
                          }}
                          placeholder="0.00"
                        />
                      ) : (
                        <span style={{ fontFamily: 'monospace', fontSize: '0.84rem', color: 'var(--text-main)', fontWeight: 500 }}>
                          {formatUsd(row.monthSettledRefundAmount)}
                        </span>
                      )}
                    </td>

                    {/* 7. 跨周期退款金额 (管理员可编辑，普通用户只读) */}
                    <td style={{ textAlign: 'right', padding: '0.45rem 0.4rem' }}>
                      {isAdmin ? (
                        <input
                          type="number"
                          step="0.01"
                          min="0"
                          className="form-input"
                          value={row.crossPeriodRefundAmount !== undefined ? row.crossPeriodRefundAmount : 0}
                          onChange={(e) => handleCellChange(row.monthStr, 'crossPeriodRefundAmount', e.target.value)}
                          style={{
                            width: '76px',
                            textAlign: 'right',
                            padding: '0.28rem 0.35rem',
                            fontSize: '0.82rem',
                            fontFamily: 'monospace',
                            borderColor: isDirty ? '#3b82f6' : 'var(--border-color)',
                            background: 'var(--bg-input)',
                            borderRadius: '0.35rem'
                          }}
                          placeholder="0.00"
                        />
                      ) : (
                        <span style={{ fontFamily: 'monospace', fontSize: '0.84rem', color: 'var(--text-main)', fontWeight: 500 }}>
                          {formatUsd(row.crossPeriodRefundAmount)}
                        </span>
                      )}
                    </td>

                    {/* 8. 分成比例 (管理员可编辑，普通用户只读) */}
                    <td style={{ textAlign: 'right', padding: '0.5rem 0.5rem' }}>
                      {isAdmin ? (
                        <div style={{ display: 'inline-flex', alignItems: 'center', gap: '0.2rem' }}>
                          <input
                            type="number"
                            step="0.01"
                            min="0"
                            max="100"
                            className="form-input"
                            value={row._displayShareRatio !== undefined ? row._displayShareRatio : (row.shareRatio !== undefined ? (parseFloat(row.shareRatio) * 100).toFixed(2) : '95.00')}
                            onChange={(e) => {
                              const val = e.target.value;
                              const num = parseFloat(val) || 0;
                              handleCellChange(row.monthStr, 'shareRatio', (num / 100).toFixed(4), val, '_displayShareRatio');
                            }}
                            onBlur={(e) => {
                              const num = parseFloat(e.target.value) || 0;
                              handleCellChange(row.monthStr, 'shareRatio', (num / 100).toFixed(4), num.toFixed(2), '_displayShareRatio');
                            }}
                            style={{
                              width: '68px',
                              textAlign: 'right',
                              padding: '0.3rem 0.35rem',
                              fontSize: '0.84rem',
                              fontFamily: 'monospace',
                              borderColor: isDirty ? '#3b82f6' : 'var(--border-color)',
                              background: 'var(--bg-input)',
                              borderRadius: '0.35rem'
                            }}
                          />
                          <span style={{ fontSize: '0.78rem', color: 'var(--text-sub)' }}>%</span>
                        </div>
                      ) : (
                        <span style={{ fontFamily: 'monospace', fontSize: '0.84rem', color: 'var(--text-main)', fontWeight: 500 }}>
                          {row.shareRatio !== undefined ? (parseFloat(row.shareRatio) * 100).toFixed(2) + '%' : '95.00%'}
                        </span>
                      )}
                    </td>

                    {/* 9. 渠道费率 (管理员可编辑，普通用户只读) */}
                    <td style={{ textAlign: 'right', padding: '0.5rem 0.5rem' }}>
                      {isAdmin ? (
                        <div style={{ display: 'inline-flex', alignItems: 'center', gap: '0.2rem' }}>
                          <input
                            type="number"
                            step="0.01"
                            min="0"
                            max="100"
                            className="form-input"
                            value={row._displayChannelFeeRate !== undefined ? row._displayChannelFeeRate : (row.channelFeeRate !== undefined ? (parseFloat(row.channelFeeRate) * 100).toFixed(2) : '7.00')}
                            onChange={(e) => {
                              const val = e.target.value;
                              const num = parseFloat(val) || 0;
                              handleCellChange(row.monthStr, 'channelFeeRate', (num / 100).toFixed(4), val, '_displayChannelFeeRate');
                            }}
                            onBlur={(e) => {
                              const num = parseFloat(e.target.value) || 0;
                              handleCellChange(row.monthStr, 'channelFeeRate', (num / 100).toFixed(4), num.toFixed(2), '_displayChannelFeeRate');
                            }}
                            style={{
                              width: '68px',
                              textAlign: 'right',
                              padding: '0.3rem 0.35rem',
                              fontSize: '0.84rem',
                              fontFamily: 'monospace',
                              borderColor: isDirty ? '#3b82f6' : 'var(--border-color)',
                              background: 'var(--bg-input)',
                              borderRadius: '0.35rem'
                            }}
                          />
                          <span style={{ fontSize: '0.78rem', color: 'var(--text-sub)' }}>%</span>
                        </div>
                      ) : (
                        <span style={{ fontFamily: 'monospace', fontSize: '0.84rem', color: 'var(--text-main)', fontWeight: 500 }}>
                          {row.channelFeeRate !== undefined ? (parseFloat(row.channelFeeRate) * 100).toFixed(2) + '%' : '7.00%'}
                        </span>
                      )}
                    </td>

                    {/* 10. 最终结算金额 (高亮计算结果) */}
                    <td style={{
                      textAlign: 'right',
                      fontWeight: 800,
                      fontSize: '0.96rem',
                      color: '#10b981',
                      fontFamily: 'monospace',
                      background: 'rgba(16, 185, 129, 0.08)',
                      borderLeft: '1px solid rgba(16, 185, 129, 0.2)',
                      borderRight: '1px solid rgba(16, 185, 129, 0.2)',
                      padding: '0.75rem 0.85rem'
                    }}>
                      {formatUsd(row.finalSettlementAmount)}
                    </td>

                    {/* 11. 退款率 */}
                    <td style={{ textAlign: 'right', fontSize: '0.82rem', color: 'var(--text-sub)', fontFamily: 'monospace', padding: '0.65rem 0.4rem' }}>
                      {row.refundRate || '0.00%'}
                    </td>

                    {/* 12. 订单数 */}
                    <td style={{ textAlign: 'right', fontSize: '0.8rem', color: 'var(--text-sub)', fontFamily: 'monospace', padding: '0.75rem 0.6rem' }}>
                      {row.totalOrders || 0} / <span style={{ color: '#f43f5e' }}>{row.refundOrders || 0}</span>
                    </td>

                    {/* 13. 备注 */}
                    <td style={{ textAlign: 'left', padding: '0.5rem 0.6rem' }}>
                      {isAdmin ? (
                        <input
                          type="text"
                          className="form-input"
                          value={row.remark || ''}
                          onChange={(e) => handleCellChange(row.monthStr, 'remark', e.target.value)}
                          placeholder="添加备注..."
                          style={{
                            width: '100%',
                            minWidth: '120px',
                            padding: '0.3rem 0.5rem',
                            fontSize: '0.82rem',
                            background: 'var(--bg-input)',
                            borderRadius: '0.35rem'
                          }}
                        />
                      ) : (
                        <span style={{ fontSize: '0.82rem', color: row.remark ? 'var(--text-main)' : 'var(--text-muted)' }}>
                          {row.remark || '-'}
                        </span>
                      )}
                    </td>

                    {/* 14. 操作按钮 (仅管理员可见) */}
                    {isAdmin && (
                      <td style={{ textAlign: 'center', padding: '0.5rem 0.5rem' }}>
                        <button
                          className={`btn ${isDirty ? 'btn-primary' : 'btn-secondary'}`}
                          disabled={isSaving}
                          onClick={() => handleSaveRow(row)}
                          style={{
                            padding: '0.3rem 0.65rem',
                            fontSize: '0.76rem',
                            gap: '0.25rem',
                            whiteSpace: 'nowrap',
                            borderRadius: '0.35rem'
                          }}
                          title={isDirty ? '有未保存修改，点击保存' : '当前配置已存入数据库'}
                        >
                          {isDirty ? <Save size={12} /> : <Check size={12} color="#10b981" />}
                          <span>{isSaving ? '保存中...' : (isDirty ? '保存' : '已存')}</span>
                        </button>
                      </td>
                    )}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
