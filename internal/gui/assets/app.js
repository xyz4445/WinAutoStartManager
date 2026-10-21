(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);

  const state = {
    items: [],
    views: new Map(),
    selectedID: null,
    boot: null,
    dirty: false,
    saving: false,
    autoBusy: false,
    rev: 0,
    ready: false,
  };

  const els = {
    bootSplash: $('bootSplash'),
    autoBox: $('autoBox'),
    autoToggle: $('autoToggle'),
    autoSpinner: $('autoSpinner'),
    autoState: $('autoState'),
    runChip: $('runChip'),
    itemList: $('itemList'),
    listEmpty: $('listEmpty'),
    countBadge: $('countBadge'),
    newBtn: $('newBtn'),
    detailEmpty: $('detailEmpty'),
    detailForm: $('detailForm'),
    detailTitle: $('detailTitle'),
    fName: $('fName'),
    fPath: $('fPath'),
    fArgs: $('fArgs'),
    fWorkDir: $('fWorkDir'),
    fDelay: $('fDelay'),
    fEnabled: $('fEnabled'),
    pathHint: $('pathHint'),
    lastRunBox: $('lastRunBox'),
    browseExe: $('browseExe'),
    browseDir: $('browseDir'),
    runBtn: $('runBtn'),
    deleteBtn: $('deleteBtn'),
    saveBtn: $('saveBtn'),
    settingsBtn: $('settingsBtn'),
    logsBtn: $('logsBtn'),
    dirBtn: $('dirBtn'),
    configPath: $('configPath'),
    saveState: $('saveState'),
    toast: $('toast'),
    settingsModal: $('settingsModal'),
    settingsErr: $('settingsErr'),
    settingsBusy: $('settingsBusy'),
    settingsBusyText: $('settingsBusyText'),
    sDataDir: $('sDataDir'),
    sRetention: $('sRetention'),
    sMigrate: $('sMigrate'),
    sBrowse: $('sBrowse'),
    sResetDir: $('sResetDir'),
    sCancel: $('sCancel'),
    sClose: $('sClose'),
    sSave: $('sSave'),
  };

  let toastTimer = null;

  function toast(message, kind) {
    els.toast.hidden = false;
    els.toast.textContent = message;
    els.toast.className = 'toast show ' + (kind || 'info');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      els.toast.classList.remove('show');
      setTimeout(() => { els.toast.hidden = true; }, 200);
    }, kind === 'error' ? 6500 : 3500);
  }

  function callErr(e) {
    if (!e) return '未知错误';
    if (typeof e === 'string') return e;
    return e.message || String(e);
  }

  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  async function holdMin(start, ms) {
    const left = ms - (performance.now() - start);
    if (left > 0) await sleep(left);
  }

  function newID() {
    if (window.crypto && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID().replace(/-/g, '');
    }
    return 'id' + Date.now().toString(16) + Math.random().toString(16).slice(2, 10);
  }

  function strip(v) {
    return {
      id: v.id || '',
      name: v.name || '',
      path: v.path || '',
      args: v.args || '',
      workDir: v.workDir || '',
      elevated: !!v.elevated,
      delaySeconds: Number(v.delaySeconds) || 0,
      enabled: !!v.enabled,
    };
  }

  function wire(it) {
    return {
      id: it.id,
      name: it.name.trim(),
      path: it.path.trim(),
      args: it.args,
      workDir: it.workDir.trim(),
      elevated: !!it.elevated,
      delaySeconds: Math.max(0, Math.min(300, Math.round(Number(it.delaySeconds) || 0))),
      enabled: !!it.enabled,
    };
  }

  function selected() {
    return state.items.find((it) => it.id === state.selectedID) || null;
  }

  function viewOf(it) {
    return state.views.get(it.id) || null;
  }

  function hasRun(v) {
    if (!v || !v.lastRunAt) return false;
    const t = new Date(v.lastRunAt);
    return !isNaN(t.getTime()) && t.getFullYear() > 2000;
  }

  function fmtTime(iso) {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    const p = (n) => String(n).padStart(2, '0');
    const today = new Date();
    const sameDay = d.toDateString() === today.toDateString();
    const hm = p(d.getHours()) + ':' + p(d.getMinutes());
    return sameDay ? ('今天 ' + hm) : (p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + hm);
  }

  /* ---------------- rendering ---------------- */

  function metaText(it, v) {
    if (!it.enabled) return '已禁用';
    if (!it.path.trim()) return '草稿 · 未填写路径，不会启动';
    const parts = ['延迟 ' + (Number(it.delaySeconds) || 0) + ' 秒'];
    if (v && v.exists === false) parts.push('路径不存在');
    if (hasRun(v)) parts.push(v.lastOK ? '上次成功' : '上次失败');
    return parts.join(' · ');
  }

  function renderList() {
    els.itemList.textContent = '';
    els.listEmpty.hidden = state.items.length > 0;
    els.countBadge.textContent = String(state.items.length);

    for (const it of state.items) {
      const v = viewOf(it);
      const isDraft = !it.path.trim();
      const missing = !!(v && v.exists === false);

      const li = document.createElement('li');
      li.className = 'item' +
        (it.id === state.selectedID ? ' selected' : '') +
        (it.enabled ? '' : ' disabled');
      li.tabIndex = 0;
      li.setAttribute('role', 'button');

      const main = document.createElement('div');
      main.className = 'item-main';

      const dot = document.createElement('span');
      dot.className = 'dot ' + (!it.enabled ? 'off' : ((isDraft || missing) ? 'warn' : 'on'));
      main.appendChild(dot);

      const name = document.createElement('span');
      name.className = 'item-name';
      name.textContent = it.name || '未命名';
      main.appendChild(name);

      const badge = document.createElement('span');
      badge.className = 'badge ' + (it.elevated ? 'admin' : 'normal');
      badge.textContent = it.elevated ? '管理员' : '普通';
      main.appendChild(badge);

      const path = document.createElement('div');
      path.className = 'item-path';
      path.textContent = it.path || '未设置路径';

      const meta = document.createElement('div');
      meta.className = 'item-meta';
      meta.textContent = metaText(it, v);

      li.append(main, path, meta);
      li.addEventListener('click', () => select(it.id));
      li.addEventListener('keydown', (ev) => {
        if (ev.key === 'Enter' || ev.key === ' ') {
          ev.preventDefault();
          select(it.id);
        }
      });
      els.itemList.appendChild(li);
    }
  }

  function updatePathHint() {
    const it = selected();
    els.pathHint.className = 'hint';
    if (!it) { els.pathHint.textContent = ''; return; }
    if (!it.path.trim()) {
      els.pathHint.textContent = '未填写路径：作为草稿保存，不会在登录时启动。';
      return;
    }
    const v = viewOf(it);
    if (v && v.exists === false) {
      els.pathHint.className = 'hint bad';
      els.pathHint.textContent = '路径不存在，请检查（修改后保存会更新状态）。';
      return;
    }
    if (v && v.exists) {
      els.pathHint.textContent = '已找到该程序。支持 %LOCALAPPDATA% 等环境变量。';
      return;
    }
    els.pathHint.textContent = '支持 %LOCALAPPDATA% 等环境变量。';
  }

  function renderLastRun() {
    const it = selected();
    const v = it ? viewOf(it) : null;
    if (!it || !hasRun(v)) {
      els.lastRunBox.hidden = true;
      els.lastRunBox.textContent = '';
      return;
    }
    els.lastRunBox.hidden = false;
    els.lastRunBox.textContent = '';

    const label = document.createElement('span');
    label.textContent = '上次运行：';
    const strong = document.createElement('strong');
    strong.textContent = fmtTime(v.lastRunAt);
    const res = document.createElement('span');
    res.className = v.lastOK ? 'good' : 'bad';
    res.textContent = v.lastOK ? ' · 成功' : ' · 失败';
    els.lastRunBox.append(label, strong, res);

    if (!v.lastOK && v.lastError) {
      const err = document.createElement('div');
      err.className = 'bad';
      err.textContent = v.lastError;
      els.lastRunBox.appendChild(err);
    }
  }

  function renderDetail() {
    const pane = document.querySelector('.detail-pane');
    if (pane) pane.scrollTop = 0;

    const it = selected();
    els.detailEmpty.hidden = !!it;
    els.detailForm.hidden = !it;
    if (!it) return;

    els.detailTitle.textContent = it.name || '未命名条目';
    els.fName.value = it.name;
    els.fPath.value = it.path;
    els.fArgs.value = it.args;
    els.fWorkDir.value = it.workDir;
    els.fDelay.value = String(it.delaySeconds ?? 0);
    els.fEnabled.checked = !!it.enabled;
    document.querySelectorAll('input[name="tier"]').forEach((radio) => {
      radio.checked = (radio.value === 'admin') === !!it.elevated;
    });

    updatePathHint();
    renderLastRun();
  }

  function setAutoState(text, warn) {
    els.autoState.textContent = text;
    els.autoState.className = warn ? 'warn' : '';
  }

  function renderStatus() {
    const b = state.boot || {};
    const status = b.taskStatus || '';
    if (status === 'ok' || status === 'stale' || status === 'missing') {
      els.autoToggle.checked = status === 'ok';
    }
    setAutoBusy(state.autoBusy);
    if (status === 'ok') {
      setAutoState('已开启 · 登录时按清单启动', false);
    } else if (status === 'stale') {
      setAutoState('需更新 · 任务指向旧路径', true);
    } else if (status === 'missing') {
      setAutoState('未开启 · 登录时不会启动', false);
    } else if (status === 'unknown') {
      setAutoState('状态未知 · 无法读取计划任务，开关保持原状', true);
    } else {
      setAutoState('正在检查任务状态', true);
    }

    if (b.lastRun) {
      els.runChip.textContent = '上次运行 ' + b.lastRun;
    } else {
      els.runChip.textContent = '尚无运行记录';
    }
    els.runChip.className = 'chip ghost';

    if (b.dataDir) {
      els.configPath.textContent = b.dataDir;
      els.configPath.title = b.dataDir;
    }
  }

  function setSaveState(text, bad) {
    els.saveState.textContent = text;
    els.saveState.className = 'savestate' + (bad ? ' bad' : '');
  }

  function setBtnBusy(btn, busy, label) {
    if (!btn) return;
    if (!btn.dataset.idleLabel) btn.dataset.idleLabel = btn.textContent.trim();
    if (busy) {
      if (!btn.classList.contains('is-busy')) {
        btn.style.minWidth = btn.offsetWidth + 'px';
      }
      btn.disabled = true;
      btn.classList.add('is-busy');
      btn.setAttribute('aria-busy', 'true');
      const sp = document.createElement('span');
      sp.className = 'spinner';
      sp.setAttribute('aria-hidden', 'true');
      const tx = document.createElement('span');
      tx.textContent = label || btn.dataset.idleLabel;
      btn.replaceChildren(sp, tx);
    } else {
      btn.classList.remove('is-busy');
      btn.removeAttribute('aria-busy');
      btn.disabled = false;
      btn.style.minWidth = '';
      btn.textContent = btn.dataset.idleLabel;
    }
  }

  function setAutoBusy(busy) {
    state.autoBusy = !!busy;
    els.autoToggle.disabled = state.autoBusy;
    els.autoBox.classList.toggle('busy', state.autoBusy);
    els.autoSpinner.hidden = !state.autoBusy;
    els.autoBox.setAttribute('aria-busy', state.autoBusy ? 'true' : 'false');
  }

  function setSettingsBusy(busy, text) {
    els.settingsBusy.hidden = !busy;
    if (busy) els.settingsBusyText.textContent = text || '正在保存设置…';
    els.settingsModal.setAttribute('aria-busy', busy ? 'true' : 'false');
  }

  function reportDirty(dirty) {
    state.dirty = dirty;
    if (!window.setDirty) return;
    const p = window.setDirty(dirty);
    if (p && p.catch) p.catch(() => {});
  }

  function select(id) {
    state.selectedID = id;
    renderList();
    renderDetail();
  }

  /* ---------------- data flow ---------------- */

  function localErrors() {
    const errs = [];
    const seen = new Set();
    state.items.forEach((it, i) => {
      const n = i + 1;
      const d = Number(it.delaySeconds);
      if (!Number.isFinite(d) || d < 0 || d > 300) {
        errs.push('第 ' + n + ' 项：延迟必须在 0–300 秒之间');
      }
      if (seen.has(it.id)) errs.push('第 ' + n + ' 项：ID 重复');
      seen.add(it.id);
    });
    return errs;
  }

  function mergeBoot(b) {
    state.boot = b;
    const list = Array.isArray(b.items) ? b.items : [];
    state.views = new Map(list.map((v) => [v.id, v]));
  }

  async function refresh() {
    const b = await window.loadApp();
    mergeBoot(b);
    state.items = (b.items || []).map(strip);
    if (!state.items.some((it) => it.id === state.selectedID)) {
      state.selectedID = state.items.length ? state.items[0].id : null;
    }
    state.rev = 0;
    reportDirty(false);
    renderStatus();
    renderList();
    renderDetail();
    setSaveState('', false);
    state.ready = true;
  }

  async function doSave(opts) {
    opts = opts || {};
    if (state.saving) return false;
    if (!state.ready) {
      if (!opts.silent) toast('清单尚未载入，请重新打开程序', 'error');
      return false;
    }
    const errs = localErrors();
    if (errs.length) {
      setSaveState('有未保存的更改', false);
      if (!opts.silent) toast(errs.join('；'), 'error');
      return false;
    }

    const rev = state.rev;
    const started = performance.now();
    state.saving = true;
    setBtnBusy(els.saveBtn, true, '保存中…');
    setSaveState('保存中…', false);
    try {
      const b = await window.saveItems(state.items.map(wire));
      mergeBoot(b);
      const stillDirty = state.rev !== rev;
      if (!stillDirty) reportDirty(false);
      setSaveState(stillDirty ? '有未保存的更改' : '已保存', false);
      renderStatus();
      renderList();
      renderLastRun();
      updatePathHint();
      if (opts.notify) {
        if (stillDirty) toast('已保存当前清单，但保存期间的新修改尚未写入', 'info');
        else toast('清单已保存', 'ok');
      }
      return true;
    } catch (e) {
      setSaveState('保存失败', true);
      if (!opts.silent) toast('保存失败：' + callErr(e), 'error');
      return false;
    } finally {
      await holdMin(started, 350);
      setBtnBusy(els.saveBtn, false);
      state.saving = false;
    }
  }

  function markDirty() {
    state.rev += 1;
    if (!state.dirty) reportDirty(true);
    setSaveState('有未保存的更改', false);
  }

  async function manualSave() {
    await doSave({ notify: true });
  }

  async function toggleAutoStart() {
    if (state.autoBusy) return;
    const on = els.autoToggle.checked;
    if (!on && !window.confirm('关闭后，登录 Windows 时不再自动启动清单中的程序。确定关闭吗？')) {
      els.autoToggle.checked = true;
      return;
    }

    const started = performance.now();
    setAutoBusy(true);
    setAutoState(on ? '正在开启…（可能弹出 UAC）' : '正在关闭…（可能弹出 UAC）', false);
    try {
      const r = await window.setAutoStart(on);
      if (r && r.ok) {
        let msg = on ? '已开启开机自启：登录时按清单启动' : '已关闭开机自启';
        if (on && state.dirty) msg += '（当前有未保存的修改，登录时使用上次保存的清单）';
        toast(msg, 'ok');
      } else {
        toast((on ? '开启失败：' : '关闭失败：') + ((r && r.error) || '未知错误'), 'error');
      }
    } catch (e) {
      toast((on ? '开启失败：' : '关闭失败：') + callErr(e), 'error');
    } finally {
      let loadErr = null;
      try {
        const b = await window.loadApp();
        mergeBoot(b);
        renderList();
        renderLastRun();
        updatePathHint();
      } catch (e) {
        loadErr = e;
      }
      await holdMin(started, 500);
      setAutoBusy(false);
      renderStatus();
      if (loadErr) toast('刷新任务状态失败：' + callErr(loadErr), 'error');
    }
  }

  function openSettings() {
    const b = state.boot || {};
    els.sDataDir.value = b.dataDir || '';
    els.sRetention.value = String(b.logRetentionDays || 14);
    els.sMigrate.checked = true;
    els.settingsErr.hidden = true;
    els.settingsErr.textContent = '';
    setSettingsBusy(false);
    setBtnBusy(els.sSave, false);
    els.settingsModal.hidden = false;
    els.sDataDir.focus();
    els.sDataDir.select();
  }

  function closeSettings() {
    if (!els.settingsBusy.hidden) return;
    els.settingsModal.hidden = true;
  }

  async function saveSettings() {
    if (!els.settingsBusy.hidden) return;
    const dir = els.sDataDir.value.trim();
    const days = Number(els.sRetention.value);
    const fail = (msg) => {
      els.settingsErr.textContent = msg;
      els.settingsErr.hidden = false;
    };
    if (!dir) {
      fail('数据目录不能为空');
      return;
    }
    if (!Number.isInteger(days) || days < 1 || days > 365) {
      fail('日志保留天数必须是 1–365 的整数');
      return;
    }
    const current = String((state.boot && state.boot.dataDir) || '');
    const dirChanged = dir.toLowerCase() !== current.toLowerCase();
    if (dirChanged && state.dirty) {
      fail('清单有未保存的更改。请先取消，点击“保存清单”，再更改数据目录。');
      return;
    }
    if (dirChanged && !els.sMigrate.checked) {
      if (!window.confirm('不迁移时，新目录不会包含现有清单和日志。确定继续吗？')) return;
    }

    const heavy = dirChanged;
    const started = performance.now();
    els.settingsErr.hidden = true;
    setBtnBusy(els.sSave, true, heavy ? '迁移中…' : '保存中…');
    if (heavy) setSettingsBusy(true, els.sMigrate.checked ? '正在迁移数据…' : '正在应用设置…');
    try {
      const r = await window.saveSettings(dir, els.sMigrate.checked, days);
      if (r && r.restartRequired) {
        setBtnBusy(els.sSave, true, '重启中…');
        setSettingsBusy(true, '设置已保存，正在重启程序…');
        await holdMin(started, 600);
        try {
          await window.restartApp();
        } catch (e) {
          setSettingsBusy(false);
          setBtnBusy(els.sSave, false);
          fail('重启失败，请手动关闭并重新打开程序：' + callErr(e));
        }
        return;
      }
      await holdMin(started, heavy ? 500 : 320);
      setSettingsBusy(false);
      setBtnBusy(els.sSave, false);
      closeSettings();
      mergeBoot(r.bootstrap);
      renderStatus();
      renderList();
      renderLastRun();
      updatePathHint();
      toast('设置已保存', 'ok');
    } catch (e) {
      setSettingsBusy(false);
      setBtnBusy(els.sSave, false);
      fail('保存失败：' + callErr(e));
    }
  }

  async function runSelected() {
    const it = selected();
    if (!it) return;
    if (!it.path.trim()) {
      toast('请先选择程序路径', 'error');
      return;
    }
    if (it.elevated) {
      const ok = window.confirm('“' + (it.name || '该条目') + '”需要管理员权限，测试运行会弹出一次 UAC 确认。继续吗？');
      if (!ok) return;
    }
    const started = performance.now();
    setBtnBusy(els.runBtn, true, '启动中…');
    try {
      const r = await window.runItem(it);
      if (r && r.ok) {
        toast('已启动“' + (it.name || it.path) + '”，进程 PID ' + r.pid, 'ok');
      } else {
        toast('启动失败：' + ((r && r.error) || '未知错误'), 'error');
      }
    } catch (e) {
      toast('启动失败：' + callErr(e), 'error');
    } finally {
      await holdMin(started, 350);
      setBtnBusy(els.runBtn, false);
    }
  }

  function addItem() {
    const it = {
      id: newID(),
      name: '',
      path: '',
      args: '',
      workDir: '',
      elevated: false,
      delaySeconds: 0,
      enabled: true,
    };
    state.items.push(it);
    markDirty();
    select(it.id);
    renderList();
    els.fName.focus();
  }

  async function deleteSelected() {
    const it = selected();
    if (!it) return;
    const label = it.name || it.path || '该条目';
    if (!window.confirm('确定删除“' + label + '”吗？此操作不可撤销。')) return;

    const idx = state.items.findIndex((x) => x.id === it.id);
    state.items.splice(idx, 1);
    state.views.delete(it.id);
    state.selectedID = state.items.length
      ? state.items[Math.min(idx, state.items.length - 1)].id
      : null;
    markDirty();
    renderList();
    renderDetail();
  }

  /* ---------------- bindings ---------------- */

  function bindField(input, key, onChange) {
    input.addEventListener('input', () => {
      const it = selected();
      if (!it) return;
      it[key] = input.value;
      if (onChange) onChange(it);
      markDirty();
      renderList();
    });
  }

  function bindStatic() {
    els.newBtn.addEventListener('click', addItem);
    els.saveBtn.addEventListener('click', manualSave);
    els.autoToggle.addEventListener('change', toggleAutoStart);
    els.deleteBtn.addEventListener('click', deleteSelected);
    els.runBtn.addEventListener('click', runSelected);

    els.logsBtn.addEventListener('click', async () => {
      try { await window.openLogs(); } catch (e) { toast('无法打开日志目录：' + callErr(e), 'error'); }
    });
    els.dirBtn.addEventListener('click', async () => {
      try { await window.openDataDir(); } catch (e) { toast('无法打开数据目录：' + callErr(e), 'error'); }
    });

    els.settingsBtn.addEventListener('click', openSettings);
    els.sCancel.addEventListener('click', closeSettings);
    els.sClose.addEventListener('click', closeSettings);
    els.settingsModal.addEventListener('click', (ev) => {
      if (ev.target === els.settingsModal) closeSettings();
    });
    document.addEventListener('keydown', (ev) => {
      if (ev.key === 'Escape' && !els.settingsModal.hidden) closeSettings();
      if ((ev.ctrlKey || ev.metaKey) && ev.key.toLowerCase() === 's') {
        ev.preventDefault();
        manualSave();
      }
    });
    els.sBrowse.addEventListener('click', async () => {
      try {
        const picked = await window.pickDir(els.sDataDir.value);
        if (picked) els.sDataDir.value = picked;
      } catch (e) {
        toast('选择目录失败：' + callErr(e), 'error');
      }
    });
    els.sResetDir.addEventListener('click', () => {
      els.sDataDir.value = (state.boot && state.boot.defaultDataDir) || '';
      els.sMigrate.checked = true;
    });
    els.sSave.addEventListener('click', saveSettings);
    els.settingsModal.addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter' && ev.target.tagName === 'INPUT' && ev.target.type !== 'checkbox') {
        ev.preventDefault();
        saveSettings();
      }
    });

    bindField(els.fName, 'name', (it) => {
      els.detailTitle.textContent = it.name || '未命名条目';
    });
    bindField(els.fPath, 'path', () => updatePathHint());
    bindField(els.fArgs, 'args');
    bindField(els.fWorkDir, 'workDir');
    bindField(els.fDelay, 'delaySeconds', (it) => {
      it.delaySeconds = Number(it.delaySeconds) || 0;
    });

    els.fEnabled.addEventListener('change', () => {
      const it = selected();
      if (!it) return;
      it.enabled = els.fEnabled.checked;
      markDirty();
      renderList();
    });

    document.querySelectorAll('input[name="tier"]').forEach((radio) => {
      radio.addEventListener('change', () => {
        const it = selected();
        if (!it || !radio.checked) return;
        it.elevated = radio.value === 'admin';
        markDirty();
        renderList();
      });
    });

    els.browseExe.addEventListener('click', async () => {
      const it = selected();
      if (!it) return;
      try {
        const picked = await window.pickExe(it.path);
        if (picked) {
          it.path = picked;
          els.fPath.value = picked;
          if (!it.name.trim()) {
            const base = picked.split(/[\\/]/).pop() || '';
            it.name = base.replace(/\.exe$/i, '');
            els.fName.value = it.name;
            els.detailTitle.textContent = it.name || '未命名条目';
          }
          markDirty();
          renderList();
          updatePathHint();
        }
      } catch (e) {
        toast('选择程序失败：' + callErr(e), 'error');
      }
    });

    els.browseDir.addEventListener('click', async () => {
      const it = selected();
      if (!it) return;
      try {
        const picked = await window.pickDir(it.workDir);
        if (picked) {
          it.workDir = picked;
          els.fWorkDir.value = picked;
          markDirty();
          renderList();
        }
      } catch (e) {
        toast('选择目录失败：' + callErr(e), 'error');
      }
    });

    els.detailForm.addEventListener('submit', (ev) => {
      ev.preventDefault();
      manualSave();
    });

  }

  let bootSplashHidden = false;

  function hideBootSplash() {
    if (bootSplashHidden) return;
    bootSplashHidden = true;
    const el = els.bootSplash;
    if (!el) return;
    el.classList.add('done');
    el.setAttribute('aria-hidden', 'true');
    setTimeout(() => { el.hidden = true; }, 320);
  }

  async function init() {
    const started = performance.now();
    const fallback = setTimeout(hideBootSplash, 8000);
    try {
      bindStatic();
      await refresh();
    } catch (e) {
      setSaveState('加载失败', true);
      setAutoState('任务状态未知', true);
      toast('加载失败：' + callErr(e), 'error');
      renderList();
      renderDetail();
    } finally {
      clearTimeout(fallback);
      await holdMin(started, 500);
      hideBootSplash();
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
