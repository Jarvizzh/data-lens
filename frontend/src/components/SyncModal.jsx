import React, { useState } from 'react';
import { RefreshCw, X, DownloadCloud, BarChart2, Zap } from 'lucide-react';

export default function SyncModal({
  isOpen,
  onClose,
  onSyncOrders,
  onRecalculateAllReports,
  onSyncAndCalcAll,
  loading,
  loadingType, // 'orders' | 'calc' | 'all' | null
  platforms = [],
  selectedPlatform = 'ALL',
}) {
  const getTodayStr = () => {
    const today = new Date();
    const yyyy = today.getFullYear();
    const mm = String(today.getMonth() + 1).padStart(2, '0');
    const dd = String(today.getDate()).padStart(2, '0');
    return `${yyyy}-${mm}-${dd}`;
  };

  const getPastDateStr = (daysAgo) => {
    const d = new Date();
    d.setDate(d.getDate() - daysAgo);
    const yyyy = d.getFullYear();
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    return `${yyyy}-${mm}-${dd}`;
  };

  const [activeTab, setActiveTab] = useState('sync'); // 'sync' | 'calc'
  const [syncPlatform, setSyncPlatform] = useState(selectedPlatform || 'ALL');
  const [startTime, setStartTime] = useState(getPastDateStr(3));
  const [endTime, setEndTime] = useState(getTodayStr());

  const getPlatformLaunchStartDate = (code) => {
    const platObj = platforms?.find(p => p.code?.toLowerCase() === (code || 'ALL').toLowerCase());
    if (platObj?.launchStartDate) return platObj.launchStartDate;
    if ((code || '').toLowerCase() === 'flicknovel') return '2026-09-16';
    return '2026-07-10';
  };

  const currentPlatformStartDate = getPlatformLaunchStartDate(syncPlatform);

  // Update syncPlatform when modal opens
  React.useEffect(() => {
    if (isOpen) {
      setSyncPlatform(selectedPlatform || 'ALL');
    }
  }, [isOpen, selectedPlatform]);

  if (!isOpen) return null;

  const handleApplyShortcut = (daysAgo) => {
    setEndTime(getTodayStr());

    if (daysAgo === 'all') {
      setStartTime(currentPlatformStartDate);
    } else {
      setStartTime(getPastDateStr(daysAgo));
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" style={{ maxWidth: '620px', width: '92%' }} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{ padding: '0.5rem', background: 'rgba(6, 182, 212, 0.15)', borderRadius: '0.5rem', color: 'var(--accent-cyan)' }}>
              <RefreshCw size={20} />
            </div>
            <div>
              <h3 className="modal-title">数据同步与重算中心</h3>
              <p style={{ margin: '2px 0 0', fontSize: '0.75rem', color: 'var(--text-sub)' }}>
                提供独立订单抓取、全量报表重算及一键增量更新
              </p>
            </div>
          </div>
          <button className="btn btn-secondary" style={{ padding: '0.25rem' }} onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        {/* Platform Selection */}
        <div style={{ padding: '0.85rem 1.5rem 0', display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          <span style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)' }}>目标平台：</span>
          <div style={{ display: 'flex', gap: '0.4rem', flexWrap: 'wrap' }}>
            <button
              type="button"
              className={`btn ${syncPlatform === 'ALL' ? 'btn-primary' : 'btn-secondary'}`}
              style={{ padding: '0.3rem 0.75rem', fontSize: '0.82rem', fontWeight: syncPlatform === 'ALL' ? 600 : 400 }}
              onClick={() => setSyncPlatform('ALL')}
            >
              全平台综合 (ALL)
            </button>
            {platforms.filter(p => p.code !== 'ALL' && !p.isAll).map(p => {
              const active = syncPlatform.toLowerCase() === p.code.toLowerCase();
              return (
                <button
                  key={p.code}
                  type="button"
                  className={`btn ${active ? 'btn-primary' : 'btn-secondary'}`}
                  style={{ padding: '0.3rem 0.75rem', fontSize: '0.82rem', fontWeight: active ? 600 : 400 }}
                  onClick={() => setSyncPlatform(p.code)}
                >
                  {p.name || p.code}
                </button>
              );
            })}
          </div>
        </div>

        {/* Tab Selection Navigation */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '0.75rem', padding: '1rem 1.5rem 0' }}>
          <button
            type="button"
            className={`segmented-btn ${activeTab === 'sync' ? 'active' : ''}`}
            onClick={() => setActiveTab('sync')}
            style={{ padding: '0.65rem 0.4rem', justifyContent: 'center' }}
          >
            <DownloadCloud size={16} />
            <span>1. 仅抓取订单</span>
          </button>

          <button
            type="button"
            className={`segmented-btn ${activeTab === 'calc' ? 'active' : ''}`}
            onClick={() => setActiveTab('calc')}
            style={{ padding: '0.65rem 0.4rem', justifyContent: 'center' }}
          >
            <BarChart2 size={16} />
            <span>2. 仅重算 LTV & 充值分析</span>
          </button>
        </div>

        {/* Body Contents */}
        <div className="modal-body" style={{ padding: '1.25rem 1.5rem' }}>
          {/* Panel 1: 仅同步订单 */}
          {activeTab === 'sync' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div style={{ fontSize: '0.82rem', color: 'var(--text-sub)', background: 'var(--bg-primary)', padding: '0.75rem', borderRadius: '0.5rem', border: '1px solid var(--border-color)' }}>
                💡 <strong>功能说明：</strong> 向远程第三方 API 批量抓取指定时间范围内的订单落库到原始订单表，<strong>不自动重算</strong>任何分析报表。
              </div>

              <div className="form-group">
                <label className="form-label">快捷时间区间：</label>
                <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                  <button type="button" className="btn btn-secondary" style={{ fontSize: '0.8rem', padding: '0.3rem 0.65rem' }} onClick={() => handleApplyShortcut(3)}>
                    近 3 天
                  </button>
                  <button type="button" className="btn btn-secondary" style={{ fontSize: '0.8rem', padding: '0.3rem 0.65rem' }} onClick={() => handleApplyShortcut(7)}>
                    近 7 天
                  </button>
                  <button type="button" className="btn btn-secondary" style={{ fontSize: '0.8rem', padding: '0.3rem 0.65rem' }} onClick={() => handleApplyShortcut(14)}>
                    近 14 天
                  </button>
                  <button type="button" className="btn btn-secondary" style={{ fontSize: '0.8rem', padding: '0.3rem 0.65rem' }} onClick={() => handleApplyShortcut(30)}>
                    近 30 天
                  </button>
                  <button type="button" className="btn btn-secondary" style={{ fontSize: '0.8rem', padding: '0.3rem 0.65rem' }} onClick={() => handleApplyShortcut('all')}>
                    全量 ({currentPlatformStartDate} 至今)
                  </button>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                <div className="form-group">
                  <label className="form-label">起始日期 (startTime)</label>
                  <input
                    type="date"
                    className="form-input"
                    value={startTime}
                    onChange={(e) => setStartTime(e.target.value)}
                  />
                </div>

                <div className="form-group">
                  <label className="form-label">截止日期 (endTime)</label>
                  <input
                    type="date"
                    className="form-input"
                    value={endTime}
                    onChange={(e) => setEndTime(e.target.value)}
                  />
                </div>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '0.5rem' }}>
                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={loading}
                  onClick={() => onSyncOrders(startTime, endTime, syncPlatform)}
                >
                  <DownloadCloud size={16} className={loading && loadingType === 'orders' ? 'spin' : ''} />
                  <span>{loading && loadingType === 'orders' ? '正在抓取订单...' : '开始抓取订单'}</span>
                </button>
              </div>
            </div>
          )}

          {/* Panel 2: 仅重算 LTV & 充值分析 */}
          {activeTab === 'calc' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
              <div style={{ fontSize: '0.85rem', color: 'var(--text-main)', background: 'rgba(59, 130, 246, 0.1)', padding: '0.85rem 1rem', borderRadius: '0.5rem', border: '1px solid rgba(59, 130, 246, 0.3)' }}>
                📊 <strong>全量报表本地重算：</strong> 读取数据库已有的全量订单，重新计算 {currentPlatformStartDate} 至今投放日期的 LTV Cohort (Day1~60 ROI) 以及 充值分析 (新老用户占比、ARPU、复充率)。适用于修改消耗/备注或更新落地页配置后快速刷新报表。
              </div>

              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '1.1rem 1rem', background: 'var(--bg-primary)', borderRadius: '0.5rem', border: '1px solid var(--border-color)' }}>
                <div>
                  <div style={{ fontWeight: 600, fontSize: '0.92rem' }}>全量 LTV & 充值分析重新计算</div>
                  <div style={{ fontSize: '0.78rem', color: 'var(--text-sub)', marginTop: '3px' }}>不抓取订单，纯本地 CPU 高速重新聚合所有报表</div>
                </div>

                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={loading}
                  onClick={() => onRecalculateAllReports(syncPlatform)}
                  style={{ background: 'linear-gradient(135deg, var(--accent-cyan), var(--accent-blue))' }}
                >
                  <BarChart2 size={16} className={loading && loadingType === 'calc' ? 'spin' : ''} />
                  <span>{loading && loadingType === 'calc' ? '重算中...' : '执行重算'}</span>
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Footer with Optional Combined Action */}
        <div className="modal-footer" style={{ justifyContent: 'space-between' }}>
          <button
            type="button"
            className="btn btn-secondary"
            disabled={loading}
            onClick={() => onSyncAndCalcAll(startTime, endTime, syncPlatform)}
            title="一键按顺序执行：1.抓取订单 -> 2.重算LTV与充值分析"
            style={{ fontSize: '0.8rem' }}
          >
            <Zap size={14} color="#f59e0b" className={loading && loadingType === 'all' ? 'spin' : ''} />
            <span>{loading && loadingType === 'all' ? '正在全流程执行...' : '一键抓取订单 + 重算全量报表'}</span>
          </button>

          <button type="button" className="btn btn-secondary" onClick={onClose} disabled={loading}>
            关闭
          </button>
        </div>
      </div>
    </div>
  );
}
