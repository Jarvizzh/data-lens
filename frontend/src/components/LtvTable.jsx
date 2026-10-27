import React, { useState } from 'react';
import { Edit2, Download } from 'lucide-react';
import { exportLtvTable } from '../utils/exportExcel';
import ExportModal from './ExportModal';

export default function LtvTable({ data, onEditRow, isReadOnly, isAdmin, isSuperAdmin, hasPermPredictPayback, hasPermRoiPredict, selectedPlatform }) {
  const isAllPlatform = (selectedPlatform || '').toUpperCase() === 'ALL';
  const [hoveredRemark, setHoveredRemark] = useState(null);
  const [hoveredPrediction, setHoveredPrediction] = useState(null);
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);
  const showPrediction = isSuperAdmin || Boolean(hasPermPredictPayback);
  const showRoiPredict = isSuperAdmin || Boolean(hasPermRoiPredict);

  const handleConfirmExport = (dateRange) => {
    exportLtvTable(data, showPrediction, '', dateRange);
  };

  const [colWidths, setColWidths] = useState({
    col0: 110, // 投放日期
    col1: 110, // 备注
    col2: 100, // 账户消耗
    col3: 100, // 累计充值
    col4: 100, // 已退款
    col5: 100, // 累计盈亏
    col6: 90,  // 累计 ROI
    col7: 75,  // 订阅用户
    col8: 80,  // 订阅成本
    colRet2: 100, // 次日留存
    colRet3: 100, // 三日留存
    col9: 110, // 7日留存
    col10: 110,// 15日留存
    col11: 88, // 预测回本
  });

  const DAY_COL_WIDTH = 90; // Day1~Day60 所有 120 个子列固定 90px 独立宽度，不允许拖动

  const left0 = 0;
  const totalBaseWidth =
    colWidths.col0 +
    colWidths.col1 +
    colWidths.col2 +
    colWidths.col3 +
    colWidths.col4 +
    colWidths.col5 +
    colWidths.col6 +
    colWidths.col7 +
    colWidths.col8 +
    colWidths.colRet2 +
    colWidths.colRet3 +
    colWidths.col9 +
    colWidths.col10 +
    (showPrediction ? colWidths.col11 : 0);

  const totalTableWidth = totalBaseWidth + (60 * 2 * DAY_COL_WIDTH);

  const handleMouseDown = (key, e) => {
    e.preventDefault();
    e.stopPropagation();
    const startX = e.clientX;
    const startWidth = colWidths[key] || 100;

    const onMouseMove = (moveEvent) => {
      const deltaX = moveEvent.clientX - startX;
      const newWidth = Math.max(30, startWidth + deltaX);
      setColWidths((prev) => ({
        ...prev,
        [key]: newWidth,
      }));
    };

    const onMouseUp = () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };

    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  };

  const formatUsd = (num) => {
    const val = parseFloat(num || 0);
    if (val < 0) {
      return `-$${Math.abs(val).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
    }
    return `$${val.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
  };

  const formatRoi = (roi) => {
    const val = parseFloat(roi || 0) * 100;
    const isHigh = val >= 100;
    const badgeClass = isHigh ? 'roi-high' : 'roi-low';

    return (
      <span className={`roi-badge ${badgeClass}`}>
        {val.toFixed(2)}%
      </span>
    );
  };

  const formatDay7Retention = (count, rate) => {
    if (count === null || count === undefined || rate === null || rate === undefined) {
      return <span style={{ color: 'var(--text-muted)' }}>-</span>;
    }
    const percent = (parseFloat(rate) * 100).toFixed(2);
    return `${count}人 / ${percent}%`;
  };

  const formatPaybackDays = (days) => {
    if (days === null || days === undefined) {
      return <span style={{ color: 'var(--text-muted)' }}>-</span>;
    }
    if (days === -1) {
      return (
        <span style={{ background: 'rgba(244, 63, 94, 0.15)', color: '#f43f5e', border: '1px solid rgba(244, 63, 94, 0.3)', padding: '0.15rem 0.4rem', borderRadius: '0.25rem', fontSize: '0.78rem', fontWeight: 600 }}>
          停滞
        </span>
      );
    }
    if (days > 365) {
      return (
        <span style={{ background: 'rgba(244, 63, 94, 0.15)', color: '#f43f5e', border: '1px solid rgba(244, 63, 94, 0.3)', padding: '0.15rem 0.4rem', borderRadius: '0.25rem', fontSize: '0.78rem', fontWeight: 600 }}>
          &gt;365天
        </span>
      );
    }
    let badgeStyle = { background: 'rgba(99, 102, 241, 0.15)', color: '#6366f1', border: '1px solid rgba(99, 102, 241, 0.3)' }; // > 90d
    if (days <= 45) {
      badgeStyle = { background: 'rgba(16, 185, 129, 0.15)', color: '#10b981', border: '1px solid rgba(16, 185, 129, 0.3)' };
    } else if (days <= 90) {
      badgeStyle = { background: 'rgba(245, 158, 11, 0.15)', color: '#f59e0b', border: '1px solid rgba(245, 158, 11, 0.3)' };
    }
    return (
      <span style={{ ...badgeStyle, padding: '0.15rem 0.4rem', borderRadius: '0.25rem', fontSize: '0.78rem', fontWeight: 600 }}>
        {days}天
      </span>
    );
  };

  const formatPredictedRoi = (roi) => {
    if (roi === null || roi === undefined || parseFloat(roi) === 0) {
      return <span style={{ color: 'var(--text-muted)' }}>-</span>;
    }
    const val = (parseFloat(roi) * 100).toFixed(2);
    return (
      <span style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--accent-cyan)' }}>
        {val}%
      </span>
    );
  };

  const days = Array.from({ length: 60 }, (_, i) => i + 1);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.65rem', width: '100%' }}>
      <div className="table-container">
        <table
          className="ltv-single-table"
          style={{ width: `${totalTableWidth}px`, minWidth: `${totalTableWidth}px`, tableLayout: 'fixed' }}
        >
          <thead>
            <tr className="header-row-1">
              <th
                className="sticky-col col-0 text-center th-resizable col-boundary"
                rowSpan={2}
                style={{ left: `${left0}px`, width: `${colWidths.col0}px`, minWidth: `${colWidths.col0}px` }}
              >
                投放日期
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col0', e)} />
              </th>
              <th
                className="col-1 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col1}px`, minWidth: `${colWidths.col1}px` }}
              >
                备注
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col1', e)} />
              </th>
              <th
                className="col-2 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col2}px`, minWidth: `${colWidths.col2}px` }}
              >
                账户消耗
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col2', e)} />
              </th>
              <th
                className="col-3 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col3}px`, minWidth: `${colWidths.col3}px` }}
              >
                累计充值
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col3', e)} />
              </th>
              <th
                className="col-4 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col4}px`, minWidth: `${colWidths.col4}px` }}
              >
                已退款
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col4', e)} />
              </th>
              <th
                className="col-5 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col5}px`, minWidth: `${colWidths.col5}px` }}
              >
                累计盈亏
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col5', e)} />
              </th>
              <th
                className="col-6 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col6}px`, minWidth: `${colWidths.col6}px` }}
              >
                累计 ROI
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col6', e)} />
              </th>
              <th
                className="col-7 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col7}px`, minWidth: `${colWidths.col7}px` }}
              >
                订阅用户
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col7', e)} />
              </th>
              <th
                className="col-8 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col8}px`, minWidth: `${colWidths.col8}px` }}
              >
                订阅成本
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col8', e)} />
              </th>
              <th
                className="col-ret2 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.colRet2}px`, minWidth: `${colWidths.colRet2}px` }}
              >
                次日留存
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('colRet2', e)} />
              </th>
              <th
                className="col-ret3 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.colRet3}px`, minWidth: `${colWidths.colRet3}px` }}
              >
                3日留存
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('colRet3', e)} />
              </th>
              <th
                className="col-9 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col9}px`, minWidth: `${colWidths.col9}px` }}
              >
                7日留存
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col9', e)} />
              </th>
              <th
                className="col-10 text-center th-resizable"
                rowSpan={2}
                style={{ width: `${colWidths.col10}px`, minWidth: `${colWidths.col10}px` }}
              >
                15日留存
                <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col10', e)} />
              </th>
              {showPrediction && (
                <th
                  className="col-11 text-center th-resizable"
                  rowSpan={2}
                  style={{ width: `${colWidths.col11}px`, minWidth: `${colWidths.col11}px` }}
                >
                  预测回本
                  <div className="resize-handle" onMouseDown={(e) => handleMouseDown('col11', e)} />
                </th>
              )}

              {days.map((day) => (
                <th key={day} colSpan={2} className="text-center day-header-group" style={{ width: `${DAY_COL_WIDTH * 2}px`, minWidth: `${DAY_COL_WIDTH * 2}px` }}>
                  Day {day}
                </th>
              ))}
            </tr>
            <tr className="header-row-2">
              {days.map((day) => (
                <React.Fragment key={day}>
                  <th
                    className="text-center sub-header"
                    style={{ width: `${DAY_COL_WIDTH}px`, minWidth: `${DAY_COL_WIDTH}px` }}
                  >
                    充值($)
                  </th>
                  <th
                    className="text-center sub-header"
                    style={{ width: `${DAY_COL_WIDTH}px`, minWidth: `${DAY_COL_WIDTH}px` }}
                  >
                    ROI
                  </th>
                </React.Fragment>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.map((row) => {
              const profit = parseFloat(row.totalProfit || 0);
              return (
                <tr key={row.launchDate}>
                  <td
                    className="sticky-col col-0 text-center col-boundary"
                    style={{ left: `${left0}px`, width: `${colWidths.col0}px`, minWidth: `${colWidths.col0}px`, fontWeight: 600, color: 'var(--text-main)' }}
                  >
                    {row.launchDate}
                  </td>
                  <td
                    className={`col-1 text-left remark-cell ${isReadOnly || isAllPlatform ? '' : 'editable-cell'}`}
                    title={isAllPlatform ? '大盘汇总备注不可直接编辑，请先选择具体平台' : (isReadOnly ? '只读模式不可编辑' : '点击编辑备注与消耗')}
                    onClick={() => onEditRow && onEditRow(row)}
                    onMouseEnter={(e) => {
                      if (row.remark && row.remark.trim()) {
                        const rect = e.currentTarget.getBoundingClientRect();
                        setHoveredRemark({
                          date: row.launchDate,
                          text: row.remark,
                          left: rect.left + rect.width / 2,
                          top: rect.top - 6,
                        });
                      }
                    }}
                    onMouseLeave={() => setHoveredRemark(null)}
                    style={{ width: `${colWidths.col1}px`, minWidth: `${colWidths.col1}px`, cursor: 'pointer' }}
                  >
                    <div className="remark-inner-container">
                      <span className="remark-text-content">
                        {row.remark || <span style={{ color: 'var(--text-muted)' }}>-</span>}
                      </span>
                      {!isReadOnly && !isAllPlatform && <Edit2 size={11} color="var(--text-muted)" style={{ flexShrink: 0 }} />}
                    </div>
                  </td>
                  <td
                    className={`col-2 text-right ${isReadOnly || isAllPlatform ? '' : 'editable-cell'}`}
                    title={isAllPlatform ? '大盘汇总消耗不可直接编辑，请先选择具体平台' : (isReadOnly ? '只读模式不可编辑' : '点击编辑备注与消耗')}
                    onClick={() => onEditRow && onEditRow(row)}
                    style={{ width: `${colWidths.col2}px`, minWidth: `${colWidths.col2}px`, cursor: 'pointer' }}
                  >
                    <span style={{ fontWeight: 600 }}>{formatUsd(row.spend)}</span>
                  </td>
                  <td
                    className="col-3 text-right"
                    style={{ width: `${colWidths.col3}px`, minWidth: `${colWidths.col3}px`, fontWeight: 600, color: 'var(--text-main)' }}
                  >
                    {formatUsd(row.totalRecharge)}
                  </td>
                  <td
                    className="col-4 text-right"
                    style={{ width: `${colWidths.col4}px`, minWidth: `${colWidths.col4}px`, fontWeight: 600, color: 'var(--text-sub)' }}
                  >
                    {formatUsd(row.totalRefund)}
                  </td>
                  <td
                    className="col-5 text-right"
                    style={{ width: `${colWidths.col5}px`, minWidth: `${colWidths.col5}px`, fontWeight: 700, color: profit >= 0 ? '#10b981' : '#f43f5e' }}
                  >
                    {formatUsd(profit)}
                  </td>
                  <td
                    className="col-6 text-center"
                    style={{ width: `${colWidths.col6}px`, minWidth: `${colWidths.col6}px` }}
                  >
                    {row.spend > 0 ? formatRoi(row.totalRoi) : <span style={{ color: 'var(--text-muted)' }}>-</span>}
                  </td>
                  <td
                    className="col-7 text-center"
                    style={{ width: `${colWidths.col7}px`, minWidth: `${colWidths.col7}px`, fontWeight: 600, color: 'var(--text-main)' }}
                  >
                    {row.subUserCount || 0}
                  </td>
                  <td
                    className="col-8 text-right"
                    style={{ width: `${colWidths.col8}px`, minWidth: `${colWidths.col8}px`, fontWeight: 500 }}
                  >
                    {formatUsd(row.subUserCost)}
                  </td>
                  <td
                    className="col-ret2 text-center"
                    style={{ width: `${colWidths.colRet2}px`, minWidth: `${colWidths.colRet2}px`, fontWeight: 500 }}
                  >
                    {formatDay7Retention(row.day2SubUserCount, row.day2SubUserRetention)}
                  </td>
                  <td
                    className="col-ret3 text-center"
                    style={{ width: `${colWidths.colRet3}px`, minWidth: `${colWidths.colRet3}px`, fontWeight: 500 }}
                  >
                    {formatDay7Retention(row.day3SubUserCount, row.day3SubUserRetention)}
                  </td>
                  <td
                    className="col-9 text-center"
                    style={{ width: `${colWidths.col9}px`, minWidth: `${colWidths.col9}px`, fontWeight: 500 }}
                  >
                    {formatDay7Retention(row.day7SubUserCount, row.day7SubUserRetention)}
                  </td>
                  <td
                    className="col-10 text-center"
                    style={{ width: `${colWidths.col10}px`, minWidth: `${colWidths.col10}px`, fontWeight: 500 }}
                  >
                    {formatDay7Retention(row.day15SubUserCount, row.day15SubUserRetention)}
                  </td>
                  {showPrediction ? (
                    <td
                      className="col-11 text-center"
                      onMouseEnter={(e) => {
                        if (row.spend > 0 && showRoiPredict) {
                          const rect = e.currentTarget.getBoundingClientRect();
                          setHoveredPrediction({
                            date: row.launchDate,
                            spend: row.spend,
                            d30Roi: row.predictedDay30Roi,
                            d60Roi: row.predictedDay60Roi,
                            d90Roi: row.predictedDay90Roi,
                            d30Recharge: row.predictedDay30Recharge,
                            d60Recharge: row.predictedDay60Recharge,
                            d90Recharge: row.predictedDay90Recharge,
                            left: rect.left + rect.width / 2,
                            top: rect.top - 6,
                          });
                        }
                      }}
                      onMouseLeave={() => setHoveredPrediction(null)}
                      style={{ width: `${colWidths.col11}px`, minWidth: `${colWidths.col11}px`, cursor: showRoiPredict ? 'pointer' : 'default' }}
                    >
                      {row.spend > 0 ? formatPaybackDays(row.predictedPaybackDays) : <span style={{ color: 'var(--text-muted)' }}>-</span>}
                    </td>
                  ) : null}

                  {/* Right Scrollable Columns (Day 1 ~ Day 30) - Fixed 90px width, no resize handles */}
                  {days.map((day) => {
                    const rechargeKey = `day${day}Recharge`;
                    const roiKey = `day${day}Roi`;
                    const rechargeVal = row[rechargeKey];
                    const roiVal = row[roiKey];
                    const isArrived = rechargeVal !== null && rechargeVal !== undefined;
                    return (
                      <React.Fragment key={day}>
                        <td
                          className="text-right"
                          style={{ width: `${DAY_COL_WIDTH}px`, minWidth: `${DAY_COL_WIDTH}px` }}
                        >
                          {isArrived ? formatUsd(rechargeVal) : '-'}
                        </td>
                        <td
                          className="text-center"
                          style={{ width: `${DAY_COL_WIDTH}px`, minWidth: `${DAY_COL_WIDTH}px` }}
                        >
                          {isArrived ? (row.spend > 0 ? formatRoi(roiVal) : <span style={{ color: 'var(--text-muted)' }}>-</span>) : <span style={{ color: 'var(--text-muted)' }}>-</span>}
                        </td>
                      </React.Fragment>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>

        {/* 鼠标划过即刻浮现完整备注 Popover 气泡 (0ms 延迟, position: fixed, 不被任何容器遮挡) */}
        {hoveredRemark && (
          <div
            className="instant-remark-popover"
            style={{
              position: 'fixed',
              left: `${hoveredRemark.left}px`,
              top: `${hoveredRemark.top}px`,
              transform: 'translate(-50%, -100%)',
              zIndex: 99999,
              pointerEvents: 'none'
            }}
          >
            <div className="popover-title">{hoveredRemark.date} 备注：</div>
            <div className="popover-body">{hoveredRemark.text}</div>
          </div>
        )}

        {/* 拥有 ROI 预测权限 (showRoiPredict) 鼠标移入“预测回本”单元格悬浮展示 D30, D60, D90 ROI 预测卡片 Tooltip */}
        {showRoiPredict && hoveredPrediction && (
          <div
            className="instant-prediction-popover"
            style={{
              position: 'fixed',
              left: `${hoveredPrediction.left}px`,
              top: `${hoveredPrediction.top}px`,
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
                🔮 ROI 趋势外推预测
              </span>
              <span style={{ fontSize: '0.72rem', color: '#94a3b8' }}>{hoveredPrediction.date}</span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.45rem', fontSize: '0.8rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ color: '#cbd5e1', fontWeight: 500 }}>D30 预测 ROI:</span>
                <span style={{ color: '#38bdf8', fontWeight: 700 }}>
                  {hoveredPrediction.d30Roi !== null && hoveredPrediction.d30Roi !== undefined ? `${(parseFloat(hoveredPrediction.d30Roi) * 100).toFixed(2)}%` : '-'}
                </span>
              </div>

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ color: '#cbd5e1', fontWeight: 500 }}>D60 预测 ROI:</span>
                <span style={{ color: '#818cf8', fontWeight: 700 }}>
                  {hoveredPrediction.d60Roi !== null && hoveredPrediction.d60Roi !== undefined ? `${(parseFloat(hoveredPrediction.d60Roi) * 100).toFixed(2)}%` : '-'}
                </span>
              </div>

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ color: '#cbd5e1', fontWeight: 500 }}>D90 预测 ROI:</span>
                <span style={{ color: '#c084fc', fontWeight: 700 }}>
                  {hoveredPrediction.d90Roi !== null && hoveredPrediction.d90Roi !== undefined ? `${(parseFloat(hoveredPrediction.d90Roi) * 100).toFixed(2)}%` : '-'}
                </span>
              </div>
            </div>
          </div>
        )}

        <ExportModal
          isOpen={isExportModalOpen}
          onClose={() => setIsExportModalOpen(false)}
          onConfirmExport={handleConfirmExport}
          title="导出 LTV 统计报表"
          maxDays={90}
          data={data}
          dateField="launchDate"
        />
      </div>
    </div>
  );
}
