import React, { useState, useEffect } from 'react';
import { X, Save, Link2, Plus, Trash2, FileText, List, RotateCcw } from 'lucide-react';
import CustomSelect from './CustomSelect';

const TIMEZONE_OPTIONS = [
  { label: '北京时间 (CST / UTC+8)', value: 'CST' },
  { label: '世界协调时 (UTC)', value: 'UTC' },
  { label: '美东时区 (ET)', value: 'ET' },
];

const getDefaultTzForPlatform = (plat) => {
  return (plat && plat.toLowerCase() === 'flicknovel') ? 'UTC' : 'CST';
};

const normalizeTz = (tz, plat) => {
  if (!tz) return getDefaultTzForPlatform(plat);
  const clean = tz.trim().toUpperCase();
  if (clean === 'BJ' || clean === 'CST') return 'CST';
  if (clean === 'UTC') return 'UTC';
  if (clean === 'ET') return 'ET';
  return getDefaultTzForPlatform(plat);
};

const getTzLabel = (tz) => {
  if (tz === 'UTC') return 'UTC';
  if (tz === 'ET') return '美东';
  return '北京';
};

export default function LandingPageConfigModal({
  isOpen,
  onClose,
  onSaved,
  authFetch,
  targetUser,
  targetUserId,
  isReadOnly,
  platformCode = 'rocnovel',
  platforms = [],
  currentUser,
}) {
  const isAdmin = (currentUser && (currentUser.role === 'ADMIN' || currentUser.role === 'SUPER_ADMIN')) || false;
  const concretePlatforms = (platforms && platforms.length > 0)
    ? platforms.filter((p) => p.code !== 'ALL' && !p.isAll)
    : [
        { code: 'rocnovel', name: 'ROCNOVEL (中文在线)' },
        { code: 'flicknovel', name: 'FLICKNOVEL (番茄司南)' },
      ];

  const initialPlat = (platformCode && platformCode !== 'ALL')
    ? platformCode
    : (concretePlatforms[0]?.code || 'rocnovel');

  const [modalPlatform, setModalPlatform] = useState(initialPlat);
  const isRocnovel = (modalPlatform || '').toLowerCase() === 'rocnovel';
  const [items, setItems] = useState([]); // [{ landingPageId: '', timezone: 'CST' }]
  const [mode, setMode] = useState('list'); // 'list' | 'batch'
  const [batchText, setBatchText] = useState('');
  const [defaultBatchTz, setDefaultBatchTz] = useState(getDefaultTzForPlatform(initialPlat));
  const [loading, setLoading] = useState(false);
  const [msg, setMsg] = useState('');

  const fetchFunc = authFetch || fetch;

  useEffect(() => {
    if (isOpen) {
      const plat = (platformCode && platformCode !== 'ALL')
        ? platformCode
        : (concretePlatforms[0]?.code || 'rocnovel');
      setModalPlatform(plat);
      setDefaultBatchTz(getDefaultTzForPlatform(plat));
    }
  }, [isOpen, platformCode]);

  useEffect(() => {
    if (isOpen) {
      setMsg('');
      setLoading(true);

      const queryParts = [];
      if (modalPlatform) queryParts.push(`platformCode=${encodeURIComponent(modalPlatform)}`);
      if (targetUserId) queryParts.push(`targetUserId=${encodeURIComponent(targetUserId)}`);
      const query = queryParts.length > 0 ? `?${queryParts.join('&')}` : '';

      fetchFunc(`/api/user/landing-pages${query}`)
        .then((res) => res.json())
        .then((data) => {
          if (data && data.code === 0) {
            let list = [];
            const resultList = data.data && data.data.configs ? data.data.configs : data.data;
            if (Array.isArray(resultList)) {
              list = resultList.map((item) => {
                if (typeof item === 'string') {
                  return { landingPageId: item, timezone: getDefaultTzForPlatform(modalPlatform) };
                }
                return {
                  landingPageId: item.landingPageId || '',
                  timezone: normalizeTz(item.timezone, modalPlatform),
                };
              });
            }
            setItems(list);

            // Sync batch text
            const batchLines = list
              .filter((it) => it.landingPageId.trim())
              .map((it) => `${it.landingPageId} ${getTzLabel(it.timezone)}`);
            setBatchText(batchLines.join('\n'));
          }
        })
        .catch((err) => console.error(err))
        .finally(() => setLoading(false));
    }
  }, [isOpen, targetUser, targetUserId, modalPlatform]);

  if (!isOpen) return null;

  const handleLoadAllPids = async () => {
    if (isReadOnly || !isAdmin) return;
    try {
      setLoading(true);
      setMsg('正在从系统加载该平台的全部推广ID...');
      const res = await fetchFunc(`/api/user/all-landing-pages?platformCode=${encodeURIComponent(modalPlatform)}`);
      const data = await res.json();
      if (data && data.code === 0 && Array.isArray(data.data) && data.data.length > 0) {
        const platDefaultTz = getDefaultTzForPlatform(modalPlatform);
        const fullList = data.data.map((pid) => ({
          landingPageId: pid,
          timezone: platDefaultTz,
        }));
        setItems(fullList);
        const batchLines = fullList.map((it) => `${it.landingPageId} ${getTzLabel(it.timezone)}`);
        setBatchText(batchLines.join('\n'));
        setMsg(`已成功载入 ${fullList.length} 个全部推广ID！您可以手动删除无需关注的项后保存。`);
      } else {
        setMsg('系统中暂未记录该平台的推广ID');
      }
    } catch (err) {
      console.error(err);
      setMsg('载入全量推广ID失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  const handleAddItem = () => {
    if (isReadOnly) return;
    setItems((prev) => [...prev, { landingPageId: '', timezone: getDefaultTzForPlatform(modalPlatform) }]);
  };

  const handleRemoveItem = (index) => {
    if (isReadOnly) return;
    setItems((prev) => prev.filter((_, i) => i !== index));
  };

  const handleItemChange = (index, field, value) => {
    if (isReadOnly) return;
    setItems((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  };

  const handleParseBatchText = () => {
    if (isReadOnly) return;
    const lines = batchText.split('\n').map((l) => l.trim()).filter((l) => l.length > 0);
    const parsed = lines.map((line) => {
      const parts = line.split(/[,，\s\t]+/);
      const pid = parts[0].trim();
      let tz = defaultBatchTz;
      if (parts.length > 1) {
        const tag = parts[1].trim().toLowerCase();
        if (tag.includes('utc')) {
          tz = 'UTC';
        } else if (tag.includes('cst') || tag.includes('bj') || tag.includes('北京') || tag.includes('shanghai')) {
          tz = 'CST';
        } else if (tag.includes('et') || tag.includes('美东') || tag.includes('york')) {
          tz = 'ET';
        }
      }
      return { landingPageId: pid, timezone: tz };
    });

    if (parsed.length > 0) {
      setItems(parsed);
      setMode('list');
      setMsg(`已从文本成功解析并导入 ${parsed.length} 条落地页配置！`);
    } else {
      setMsg('文本内容为空，请重新粘贴');
    }
  };

  const handleSave = async () => {
    if (isReadOnly) {
      setMsg('只读/主账号视图模式下无法修改或提交落地页配置');
      return;
    }

    setLoading(true);
    setMsg('保存配置中，并自动完成秒级数据重算...');

    let validItems = items
      .map((it) => ({
        landingPageId: (it.landingPageId || '').trim(),
        timezone: normalizeTz(it.timezone, modalPlatform),
        platformCode: modalPlatform,
      }))
      .filter((it) => it.landingPageId.length > 0);

    if (mode === 'batch' && batchText.trim()) {
      const lines = batchText.split('\n').map((l) => l.trim()).filter((l) => l.length > 0);
      validItems = lines.map((line) => {
        const parts = line.split(/[,，\s\t]+/);
        const pid = parts[0].trim();
        let tz = defaultBatchTz;
        if (parts.length > 1) {
          const tag = parts[1].trim().toLowerCase();
          if (tag.includes('utc')) {
            tz = 'UTC';
          } else if (tag.includes('cst') || tag.includes('bj') || tag.includes('北京') || tag.includes('shanghai')) {
            tz = 'CST';
          } else if (tag.includes('et') || tag.includes('美东') || tag.includes('york')) {
            tz = 'ET';
          }
        }
        return { landingPageId: pid, timezone: tz, platformCode: modalPlatform };
      });
    }

    const endpoint = targetUser
      ? `/api/admin/users/${targetUser.id}/landing-pages`
      : '/api/user/landing-pages';

    const method = targetUser ? 'PUT' : 'POST';

    const payload = targetUser
      ? { platformCode: modalPlatform, landingPages: validItems }
      : { platformCode: modalPlatform, landingPages: validItems, targetUserId: targetUserId || null };

    try {
      const res = await fetchFunc(endpoint, {
        method: method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (res.ok && data && data.code === 0) {
        setMsg(data.msg || '保存成功！报表已根据新配置与时区完成实时计算。');
        setTimeout(() => {
          setLoading(false);
          if (onSaved) onSaved();
          onClose();
        }, 800);
      } else {
        const errorText = (data && (data.msg || data.message)) || `HTTP ${res.status}`;
        setMsg(`保存失败: ${errorText}`);
        setLoading(false);
      }
    } catch (err) {
      if (err.message !== 'UNAUTHORIZED') {
        setMsg('网络异常，保存失败，请稍后重试');
      }
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 680, width: '92%' }}>
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Link2 size={20} className="modal-header-icon" />
            <h3 className="modal-title">
              {targetUser ? `配置用户 [${targetUser.username}] 的落地页配置` : `落地页配置 ${isReadOnly ? '(只读查看)' : '(含时区设置)'}`}
            </h3>
          </div>
          <button className="btn btn-secondary" style={{ padding: '0.25rem' }} onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="modal-body" style={{ maxHeight: '70vh', overflowY: 'auto' }}>
          {isReadOnly && (
            <div style={{
              background: 'rgba(245, 158, 11, 0.12)',
              border: '1px solid rgba(245, 158, 11, 0.3)',
              color: '#f59e0b',
              padding: '0.5rem 0.75rem',
              borderRadius: '0.375rem',
              fontSize: '0.82rem',
              marginBottom: '0.85rem'
            }}>
              ⚠️ 当前视图为只读模式/主账号视图，当前账户配置的落地页仅供查看。
            </div>
          )}

          {concretePlatforms.length > 1 && (
            <div style={{ marginBottom: '1rem' }}>
              <div style={{ fontSize: '0.82rem', fontWeight: 600, color: 'var(--text-sub)', marginBottom: '0.4rem' }}>
                业务平台：
              </div>
              <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                {concretePlatforms.map((p) => {
                  const isActive = modalPlatform.toLowerCase() === p.code.toLowerCase();
                  return (
                    <button
                      key={p.code}
                      type="button"
                      className={`btn ${isActive ? 'btn-primary' : 'btn-secondary'}`}
                      style={{
                        padding: '0.35rem 0.8rem',
                        fontSize: '0.85rem',
                        fontWeight: isActive ? 600 : 400,
                      }}
                      onClick={() => setModalPlatform(p.code)}
                    >
                      {p.name || p.code}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem', flexWrap: 'wrap', gap: '0.5rem' }}>
            <div className="segmented-tab-container">
              <button
                type="button"
                className={`segmented-tab-item ${mode === 'list' ? 'active' : ''}`}
                onClick={() => setMode('list')}
              >
                <List size={15} />
                <span>列表明细</span>
              </button>
              <button
                type="button"
                className={`segmented-tab-item ${mode === 'batch' ? 'active' : ''}`}
                onClick={() => setMode('batch')}
              >
                <FileText size={15} />
                <span>批量文本查看</span>
              </button>
            </div>

            {!isReadOnly && isAdmin && !isRocnovel && (
              <button
                type="button"
                className="btn btn-secondary"
                style={{ fontSize: '0.8rem', padding: '0.32rem 0.75rem', display: 'inline-flex', alignItems: 'center', gap: 5 }}
                onClick={handleLoadAllPids}
                title="载入系统已知当前平台的全部推广ID"
              >
                <RotateCcw size={14} />
                <span>载入全量推广ID</span>
              </button>
            )}
          </div>

          {mode === 'list' ? (
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem', flexWrap: 'wrap', gap: 6 }}>
                <p style={{ fontSize: '0.82rem', color: 'var(--text-sub)', margin: 0 }}>
                  当前共 {items.length} 个推广ID配置项，支持单独修改时区或手动删除。
                  {modalPlatform === 'flicknovel' && isAdmin && (
                    <span style={{ marginLeft: 8, color: '#10b981', fontWeight: 500 }}>
                      （系统每4小时定时自动导入全量推广ID）
                    </span>
                  )}
                </p>
              </div>

              {items.length === 0 ? (
                <div style={{
                  padding: '2rem 1rem',
                  textAlign: 'center',
                  background: 'var(--bg-secondary)',
                  borderRadius: '0.5rem',
                  border: '1px dashed var(--border-color)',
                  color: 'var(--text-sub)',
                  fontSize: '0.85rem',
                }}>
                  <p style={{ marginBottom: '0.75rem' }}>
                    {isAdmin ? '当前暂无配置任何推广ID（保存将按空配置生效）' : '当前暂无配置任何推广ID，请点击下方添加或由管理员配置'}
                  </p>
                  {!isReadOnly && (
                    <div style={{ display: 'flex', justifyContent: 'center', gap: '0.75rem' }}>
                      {isAdmin && !isRocnovel && (
                        <button
                          type="button"
                          className="btn btn-primary"
                          style={{ fontSize: '0.82rem', padding: '0.35rem 0.8rem' }}
                          onClick={handleLoadAllPids}
                        >
                          <RotateCcw size={14} style={{ marginRight: 4 }} />
                          一键载入全量推广ID
                        </button>
                      )}
                      <button
                        type="button"
                        className={isAdmin && !isRocnovel ? "btn btn-secondary" : "btn btn-primary"}
                        style={{ fontSize: '0.82rem', padding: '0.35rem 0.8rem' }}
                        onClick={handleAddItem}
                      >
                        <Plus size={14} style={{ marginRight: 4 }} />
                        手动添加
                      </button>
                    </div>
                  )}
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                  {items.map((item, idx) => (
                    <div
                      key={idx}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '0.5rem',
                        background: 'var(--bg-secondary)',
                        padding: '0.4rem 0.6rem',
                        borderRadius: '0.375rem',
                        border: '1px solid var(--border-color)',
                      }}
                    >
                      <span style={{ fontSize: '0.78rem', color: 'var(--text-muted)', width: '24px', textAlign: 'center' }}>
                        {idx + 1}
                      </span>
                      <input
                        type="text"
                        className="form-input"
                        style={{ flex: 1, padding: '0.35rem 0.6rem', fontSize: '0.85rem' }}
                        placeholder="落地页/推广 ID (pId)"
                        value={item.landingPageId}
                        disabled={isReadOnly}
                        onChange={(e) => handleItemChange(idx, 'landingPageId', e.target.value)}
                      />
                      <CustomSelect
                        value={item.timezone}
                        onChange={(val) => handleItemChange(idx, 'timezone', val)}
                        options={TIMEZONE_OPTIONS}
                        disabled={isReadOnly}
                        style={{ width: '180px' }}
                      />
                      {!isReadOnly && (
                        <button
                          type="button"
                          className="btn btn-secondary"
                          style={{ padding: '0.35rem', color: '#f43f5e' }}
                          onClick={() => handleRemoveItem(idx)}
                          title="删除此项"
                        >
                          <Trash2 size={16} />
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              )}

              {!isReadOnly && items.length > 0 && (
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ marginTop: '0.75rem', width: '100%', justifyContent: 'center', borderStyle: 'dashed' }}
                  onClick={handleAddItem}
                >
                  <Plus size={16} />
                  <span>添加落地页</span>
                </button>
              )}
            </div>
          ) : (
            <div>
              <p style={{ fontSize: '0.82rem', color: 'var(--text-sub)', marginBottom: '0.5rem' }}>
                已配置的落地页 ID 列表文本视图：
              </p>

              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.5rem', fontSize: '0.82rem', flexWrap: 'wrap' }}>
                <span style={{ color: 'var(--text-sub)' }}>未标注行的默认时区：</span>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.25rem', cursor: isReadOnly ? 'not-allowed' : 'pointer' }}>
                  <input
                    type="radio"
                    name="defaultTz"
                    value="CST"
                    checked={defaultBatchTz === 'CST'}
                    disabled={isReadOnly}
                    onChange={() => setDefaultBatchTz('CST')}
                  />
                  北京时间 (CST)
                </label>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.25rem', cursor: isReadOnly ? 'not-allowed' : 'pointer' }}>
                  <input
                    type="radio"
                    name="defaultTz"
                    value="UTC"
                    checked={defaultBatchTz === 'UTC'}
                    disabled={isReadOnly}
                    onChange={() => setDefaultBatchTz('UTC')}
                  />
                  世界协调时 (UTC)
                </label>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: '0.25rem', cursor: isReadOnly ? 'not-allowed' : 'pointer' }}>
                  <input
                    type="radio"
                    name="defaultTz"
                    value="ET"
                    checked={defaultBatchTz === 'ET'}
                    disabled={isReadOnly}
                    onChange={() => setDefaultBatchTz('ET')}
                  />
                  美东时区 (ET)
                </label>
              </div>

              <textarea
                className="form-textarea"
                rows={8}
                placeholder="暂无落地页配置"
                value={batchText}
                disabled={isReadOnly}
                onChange={(e) => setBatchText(e.target.value)}
              />

              {!isReadOnly && (
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ marginTop: '0.5rem', width: '100%', justifyContent: 'center' }}
                  onClick={handleParseBatchText}
                >
                  <span>解析并导入到列表明细</span>
                </button>
              )}
            </div>
          )}

          {msg && (
            <div style={{ fontSize: '0.85rem', marginTop: 12, padding: '0.5rem 0.75rem', borderRadius: '0.375rem', background: msg.includes('失败') ? 'rgba(244, 63, 94, 0.15)' : 'rgba(6, 182, 212, 0.15)', color: msg.includes('失败') ? 'var(--accent-rose)' : 'var(--accent-cyan)' }}>
              {msg}
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose} disabled={loading}>
            关闭
          </button>

          <button
            className="btn btn-primary"
            onClick={handleSave}
            disabled={loading || isReadOnly}
            style={{
              opacity: isReadOnly ? 0.45 : 1,
              cursor: isReadOnly ? 'not-allowed' : 'pointer'
            }}
            title={isReadOnly ? '只读/主账号视图模式下不可提交修改' : '保存配置并秒级重算'}
          >
            <Save size={16} />
            <span>{isReadOnly ? '保存配置 (只读不可提交)' : '保存配置并秒级重算'}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
