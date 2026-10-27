import * as XLSX from 'xlsx';

/**
 * 导出原生 .xlsx 格式的 Excel 工作表文件
 */
export function exportToXlsx(filename, headers, rows) {
  if (!rows || !rows.length) return;

  const aoaData = [headers, ...rows];
  const worksheet = XLSX.utils.aoa_to_sheet(aoaData);

  // 自动根据内容设置列宽
  const colWidths = headers.map((h, i) => {
    let maxLen = String(h).length;
    rows.forEach((r) => {
      if (r[i] !== null && r[i] !== undefined) {
        const len = String(r[i]).length;
        if (len > maxLen) maxLen = len;
      }
    });
    return { wch: Math.min(Math.max(maxLen + 3, 10), 35) };
  });
  worksheet['!cols'] = colWidths;

  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, worksheet, '数据报表');

  // 确保文件名以 .xlsx 结尾
  const cleanFilename = filename.endsWith('.xlsx') ? filename : `${filename}.xlsx`;
  XLSX.writeFile(workbook, cleanFilename);
}

/**
 * 导出带 UTF-8 BOM 标识的 CSV 文本文件 (包含 \uFEFF 防 Excel 中文乱码)
 */
export function exportToCsv(filename, headers, rows) {
  if (!rows || !rows.length) return;

  const escapeCsv = (val) => {
    if (val === null || val === undefined) return '""';
    const str = String(val).replace(/"/g, '""');
    return `"${str}"`;
  };

  const headerLine = headers.map(escapeCsv).join(',');
  const rowLines = rows.map((r) => r.map(escapeCsv).join(','));
  const csvContent = '\uFEFF' + [headerLine, ...rowLines].join('\n');

  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.setAttribute('href', url);

  const cleanFilename = filename.endsWith('.csv') ? filename : `${filename}.csv`;
  link.setAttribute('download', cleanFilename);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

/**
 * 导出 LTV 统计报表 (支持 xlsx / csv 格式)
 */
export function exportLtvTable(data, showPrediction = true, username = '', dateRange = null) {
  if (!Array.isArray(data) || data.length === 0) return;

  let filteredData = data;
  if (dateRange && dateRange.startDate && dateRange.endDate) {
    filteredData = data.filter((item) => {
      const d = item.launchDate;
      return d && d >= dateRange.startDate && d <= dateRange.endDate;
    });
  }

  if (filteredData.length === 0) return;

  const headers = [
    '投放日期',
    '备注/视图',
    '账号消耗(USD)',
    '累计充值(USD)',
    '已退款(USD)',
    '累计盈亏(USD)',
    '累计ROI',
    '订阅用户(人)',
    '订阅成本(USD)',
    '次日留存人数',
    '次日留存率',
    '三日留存人数',
    '三日留存率',
    '7日留存人数',
    '7日留存率',
    '15日留存人数',
    '15日留存率',
  ];

  if (showPrediction) {
    headers.push('预测回本(天)');
  }

  // 增加 Day 1 ~ Day 60 的充值与 ROI 列
  for (let d = 1; d <= 60; d++) {
    headers.push(`Day ${d} 充值($)`);
    headers.push(`Day ${d} ROI`);
  }

  const rows = filteredData.map((item) => {
    const spend = item.spend != null ? parseFloat(item.spend).toFixed(2) : '0.00';
    const recharge = item.totalRecharge != null ? parseFloat(item.totalRecharge).toFixed(2) : '0.00';
    const refund = item.totalRefund != null ? parseFloat(item.totalRefund).toFixed(2) : '0.00';
    const profit = item.totalProfit != null ? parseFloat(item.totalProfit).toFixed(2) : '0.00';
    const roi = item.totalRoi != null ? `${(parseFloat(item.totalRoi) * 100).toFixed(2)}%` : '0.00%';
    const subUsers = item.subUserCount != null ? item.subUserCount : 0;
    const subCost = item.subUserCost != null ? parseFloat(item.subUserCost).toFixed(2) : '0.00';
    const day2Count = item.day2SubUserCount != null ? item.day2SubUserCount : '-';
    const day2Rate = item.day2SubUserRetention != null ? `${(parseFloat(item.day2SubUserRetention) * 100).toFixed(2)}%` : '-';
    const day3Count = item.day3SubUserCount != null ? item.day3SubUserCount : '-';
    const day3Rate = item.day3SubUserRetention != null ? `${(parseFloat(item.day3SubUserRetention) * 100).toFixed(2)}%` : '-';
    const day7Count = item.day7SubUserCount != null ? item.day7SubUserCount : '-';
    const day7Rate = item.day7SubUserRetention != null ? `${(parseFloat(item.day7SubUserRetention) * 100).toFixed(2)}%` : '-';
    const day15Count = item.day15SubUserCount != null ? item.day15SubUserCount : '-';
    const day15Rate = item.day15SubUserRetention != null ? `${(parseFloat(item.day15SubUserRetention) * 100).toFixed(2)}%` : '-';

    let payback = '-';
    if (showPrediction) {
      if (item.predictedPaybackDays === -1) payback = '停滞';
      else if (item.predictedPaybackDays > 365) payback = '>365天';
      else if (item.predictedPaybackDays != null) payback = `${item.predictedPaybackDays}天`;
    }

    const row = [
      item.launchDate || '',
      item.remark || username || '',
      spend,
      recharge,
      refund,
      profit,
      roi,
      subUsers,
      subCost,
      day2Count,
      day2Rate,
      day3Count,
      day3Rate,
      day7Count,
      day7Rate,
      day15Count,
      day15Rate,
    ];

    if (showPrediction) {
      row.push(payback);
    }

    // Day 1 ~ Day 60
    for (let d = 1; d <= 60; d++) {
      const dayRechargeKey = `day${d}Recharge`;
      const dayRoiKey = `day${d}Roi`;

      const dayRechargeVal = item[dayRechargeKey];
      const dayRoiVal = item[dayRoiKey];

      const formattedRecharge = dayRechargeVal != null ? parseFloat(dayRechargeVal).toFixed(2) : '-';
      const formattedRoi = dayRoiVal != null ? `${(parseFloat(dayRoiVal) * 100).toFixed(2)}%` : '-';

      row.push(formattedRecharge);
      row.push(formattedRoi);
    }

    return row;
  });

  const rangeTag = dateRange ? `${dateRange.startDate}_至_${dateRange.endDate}` : new Date().toISOString().slice(0, 10);
  const format = (dateRange && dateRange.format) ? dateRange.format : 'xlsx';

  if (format === 'csv') {
    const filename = `Meta_LTV_Report_${rangeTag}.csv`;
    exportToCsv(filename, headers, rows);
  } else {
    const filename = `Meta_LTV_Report_${rangeTag}.xlsx`;
    exportToXlsx(filename, headers, rows);
  }
}

/**
 * 导出每日充值分布 / 平台汇总表 (支持 xlsx / csv 格式)
 */
export function exportDistributionTable(data, isGlobal = false, dateRange = null) {
  if (!Array.isArray(data) || data.length === 0) return;

  let filteredData = data;
  if (dateRange && dateRange.startDate && dateRange.endDate) {
    filteredData = data.filter((item) => {
      const d = item.date;
      return d && d >= dateRange.startDate && d <= dateRange.endDate;
    });
  }

  if (filteredData.length === 0) return;

  const headers = [
    '支付日期',
    '总充值(USD)',
    '单次充值(USD)',
    '订阅充值(USD)',
    '付款总人数',
    '单次人数',
    '订阅人数',
    '新客充值(USD)',
    '新客占比',
    '新客ARPU(USD)',
    '新客人数',
    '新客单次人数',
    '新客订阅人数',
    '老客充值(USD)',
    '老客占比',
    '老客ARPU(USD)',
    '老客人数',
    '老客单次人数',
    '老客订阅人数',
    '当日复购人数',
    '当日复购率',
  ];

  const rows = filteredData.map((item) => {
    const totalRecharge = item.totalRecharge != null ? parseFloat(item.totalRecharge).toFixed(2) : '0.00';
    const singleRecharge = item.singleRecharge != null ? parseFloat(item.singleRecharge).toFixed(2) : '0.00';
    const subsRecharge = item.subsRecharge != null ? parseFloat(item.subsRecharge).toFixed(2) : '0.00';

    const newRecharge = item.newRecharge != null ? parseFloat(item.newRecharge).toFixed(2) : '0.00';
    const newRatio = item.newRechargeRatio != null ? `${(parseFloat(item.newRechargeRatio) * 100).toFixed(2)}%` : '0.00%';
    const newArpu = item.newArpu != null ? parseFloat(item.newArpu).toFixed(2) : '0.00';

    const oldRecharge = item.oldRecharge != null ? parseFloat(item.oldRecharge).toFixed(2) : '0.00';
    const oldRatio = item.oldRechargeRatio != null ? `${(parseFloat(item.oldRechargeRatio) * 100).toFixed(2)}%` : '0.00%';
    const oldArpu = item.oldArpu != null ? parseFloat(item.oldArpu).toFixed(2) : '0.00';

    const repeatRate = item.repeatRate != null ? `${(parseFloat(item.repeatRate) * 100).toFixed(2)}%` : '0.00%';

    return [
      item.date || '',
      totalRecharge,
      singleRecharge,
      subsRecharge,
      item.totalPaidUsers || 0,
      item.singlePaidUsers || 0,
      item.subsPaidUsers || 0,
      newRecharge,
      newRatio,
      newArpu,
      item.newPaidUsers || 0,
      item.newSinglePaidUsers || 0,
      item.newSubsPaidUsers || 0,
      oldRecharge,
      oldRatio,
      oldArpu,
      item.oldPaidUsers || 0,
      item.oldSinglePaidUsers || 0,
      item.oldSubsPaidUsers || 0,
      item.repeatPaidUsers || 0,
      repeatRate,
    ];
  });

  const prefix = isGlobal ? 'Meta_LTV_Platform_Global_Summary' : 'Meta_LTV_Daily_Recharge_Distribution';
  const rangeTag = dateRange ? `${dateRange.startDate}_至_${dateRange.endDate}` : new Date().toISOString().slice(0, 10);
  const format = (dateRange && dateRange.format) ? dateRange.format : 'xlsx';

  if (format === 'csv') {
    const filename = `${prefix}_${rangeTag}.csv`;
    exportToCsv(filename, headers, rows);
  } else {
    const filename = `${prefix}_${rangeTag}.xlsx`;
    exportToXlsx(filename, headers, rows);
  }
}
