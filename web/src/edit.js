// web/src/edit.js
import { S, doc_, api, apiPostJson, MOD } from './state.js';
import { showToast, copyToClipboard, vp } from './ui.js';
import { render, layout, placeCaret } from './renderer.js';
import { revealCaretX, clearSelection, updateDomSelection } from './cursor.js';
import { updateStatus } from './status.js';
import { drawTabs, loadGutter } from './tabs.js';
import { refreshTree } from './tree.js';
import { previewing } from './markdown.js';
import { clearFind } from './find.js';
import { clearSelectAll } from './selbar.js';
import { isVimEnabled, getVimMode } from './vim.js';

const EDIT_MAX_BYTES = 2 * 1024 * 1024;
const EDIT_MAX_LINES = 50000;

let loadP = null;
const queued = [];

export function editingEnabled() {
  return S.meta?.edit !== false;
}

export function canEditDoc(d = doc_()) {
  if (!editingEnabled() || !d) return false;
  if (d.isImage || d.diffMode || previewing(d)) return false;
  return true;
}

export function lineLen(d, line) {
  if (d.text) return (d.text[line - 1] ?? '').length;
  const row = document.querySelector('#rows .row[data-l="' + line + '"]');
  const code = row && row.querySelector('.c');
  return code ? code.textContent.length : 0;
}

function colOf(d) {
  if (d.col === Infinity) return lineLen(d, d.cur);
  return d.col || 0;
}

function selRange(d) {
  if (S.selAll === d) {
    return {
      start: { line: 1, col: 0 },
      end: { line: d.total, col: lineLen(d, d.total) },
    };
  }
  if (!d.selAnchor) return null;
  const a = { line: d.selAnchor.line, col: d.selAnchor.col };
  const f = { line: d.cur, col: colOf(d) };
  if (a.line > f.line || (a.line === f.line && a.col > f.col)) return { start: f, end: a };
  if (a.line === f.line && a.col === f.col) return null;
  return { start: a, end: f };
}

function recount(d) {
  if (!d.text.length) d.text = [''];
  d.total = d.text.length;
  let m = 0, bytes = 0;
  const nl = d.nl || '\n';
  for (const ln of d.text) {
    if (ln.length > m) m = ln.length;
    bytes += ln.length + nl.length;
  }
  d.maxCols = m;
  d.size = bytes;
}

function snapshot(d) {
  return { text: d.text.slice(), cur: d.cur, col: d.col, total: d.total };
}

function restore(d, snap) {
  d.text = snap.text.slice();
  d.cur = snap.cur;
  d.col = snap.col;
  recount(d);
}

function beginEdit(d, op) {
  const now = Date.now();
  if (!(op === 'type' && d._lastOp === 'type' && now - d._lastOpAt < 400)) {
    d.undo = d.undo || [];
    d.undo.push(snapshot(d));
    if (d.undo.length > 200) d.undo.shift();
  }
  d._lastOp = op;
  d._lastOpAt = now;
  d.redo = [];
}

function markDirty(d) {
  d.dirty = serialize(d) !== d.original;
  S.occ = null;
  if (S.find) clearFind();
  S.selAll = S.selAll === d ? null : S.selAll;
  recount(d);
  layout();
  render();
  revealCaretX(placeCaret());
  updateDomSelection();
  updateStatus();
  drawTabs();
}

export async function ensureBuffer(d = doc_()) {
  if (!d || d.text) return !!d?.text;
  if (!canEditDoc(d)) return false;
  if ((d.size || 0) > EDIT_MAX_BYTES || (d.total || 0) > EDIT_MAX_LINES) {
    showToast('!', 'File too large to edit here. Use Alt+E.');
    return false;
  }
  try {
    const r = await fetch('/api/raw?path=' + encodeURIComponent(d.path));
    if (!r.ok) throw new Error('could not load file');
    const raw = await r.text();
    if (raw.length > EDIT_MAX_BYTES) {
      showToast('!', 'File too large to edit here. Use Alt+E.');
      return false;
    }
    d.nl = raw.includes('\r\n') ? '\r\n' : '\n';
    d.eofNl = raw.endsWith('\n');
    let body = raw;
    if (raw.endsWith('\r\n')) body = raw.slice(0, -2);
    else if (raw.endsWith('\n')) body = raw.slice(0, -1);
    d.text = body.split(d.nl);
    if (!d.text.length) d.text = [''];
    d.original = raw;
    d.undo = [];
    d.redo = [];
    recount(d);
    return true;
  } catch (e) {
    showToast('!', e.message || 'Could not load file for editing');
    return false;
  }
}

function deleteRange(d, s, e) {
  if (s.line === e.line) {
    const ln = d.text[s.line - 1];
    d.text[s.line - 1] = ln.slice(0, s.col) + ln.slice(e.col);
  } else {
    const head = d.text[s.line - 1].slice(0, s.col);
    const tail = d.text[e.line - 1].slice(e.col);
    d.text.splice(s.line - 1, e.line - s.line + 1, head + tail);
  }
  d.cur = s.line;
  d.col = s.col;
  d.selAnchor = null;
  clearSelection(d);
  clearSelectAll();
}

function insertAt(d, line, col, str) {
  const parts = String(str).split(/\r\n|\n|\r/);
  const ln = d.text[line - 1] ?? '';
  if (parts.length === 1) {
    d.text[line - 1] = ln.slice(0, col) + parts[0] + ln.slice(col);
    d.cur = line;
    d.col = col + parts[0].length;
    return;
  }
  const first = ln.slice(0, col) + parts[0];
  const last = parts[parts.length - 1] + ln.slice(col);
  const mid = parts.slice(1, -1);
  d.text.splice(line - 1, 1, first, ...mid, last);
  d.cur = line + parts.length - 1;
  d.col = parts[parts.length - 1].length;
}

export function insertText(str, op = 'type') {
  const d = doc_();
  if (!d || !d.text) return false;
  beginEdit(d, op);
  const sel = selRange(d);
  if (sel) deleteRange(d, sel.start, sel.end);
  insertAt(d, d.cur, colOf(d), str);
  d.selAnchor = null;
  markDirty(d);
  return true;
}

export function deleteSelection() {
  const d = doc_();
  if (!d || !d.text) return false;
  const sel = selRange(d);
  if (!sel) return false;
  beginEdit(d, 'delete');
  deleteRange(d, sel.start, sel.end);
  markDirty(d);
  return true;
}

export function backspace() {
  const d = doc_();
  if (!d || !d.text) return false;
  if (selRange(d)) return deleteSelection();
  const col = colOf(d);
  if (col === 0 && d.cur <= 1) return true;
  beginEdit(d, 'delete');
  if (col > 0) {
    const ln = d.text[d.cur - 1];
    d.text[d.cur - 1] = ln.slice(0, col - 1) + ln.slice(col);
    d.col = col - 1;
  } else {
    const prev = d.text[d.cur - 2];
    const cur = d.text[d.cur - 1];
    d.text.splice(d.cur - 2, 2, prev + cur);
    d.cur -= 1;
    d.col = prev.length;
  }
  markDirty(d);
  return true;
}

export function deleteForward() {
  const d = doc_();
  if (!d || !d.text) return false;
  if (selRange(d)) return deleteSelection();
  const col = colOf(d);
  const ln = d.text[d.cur - 1];
  if (col >= ln.length && d.cur >= d.total) return true;
  beginEdit(d, 'delete');
  if (col < ln.length) {
    d.text[d.cur - 1] = ln.slice(0, col) + ln.slice(col + 1);
  } else {
    d.text.splice(d.cur - 1, 2, ln + d.text[d.cur]);
    d.col = col;
  }
  markDirty(d);
  return true;
}

export function newline() {
  return insertText('\n', 'nl');
}

export function deleteChar() {
  const d = doc_();
  if (!d || !d.text) return false;
  if (selRange(d)) return deleteSelection();
  const col = colOf(d);
  const ln = d.text[d.cur - 1];
  if (col >= ln.length) return false;
  d.col = col;
  return deleteForward();
}

export function deleteLine() {
  const d = doc_();
  if (!d || !d.text) return false;
  beginEdit(d, 'delete');
  if (d.total === 1) {
    d.text[0] = '';
    d.col = 0;
  } else {
    d.text.splice(d.cur - 1, 1);
    if (d.cur > d.text.length) d.cur = d.text.length;
    d.col = 0;
  }
  d.selAnchor = null;
  clearSelection(d);
  markDirty(d);
  return true;
}

export function undo() {
  const d = doc_();
  if (!d?.undo?.length || !d.text) return false;
  d.redo = d.redo || [];
  d.redo.push(snapshot(d));
  restore(d, d.undo.pop());
  d._lastOp = '';
  markDirty(d);
  return true;
}

export function redo() {
  const d = doc_();
  if (!d?.redo?.length || !d.text) return false;
  d.undo = d.undo || [];
  d.undo.push(snapshot(d));
  restore(d, d.redo.pop());
  markDirty(d);
  return true;
}

function serialize(d) {
  const nl = d.nl || '\n';
  let out = d.text.join(nl);
  if (d.eofNl) out += nl;
  return out;
}

export async function saveActive(force = false) {
  const d = doc_();
  if (!d || !editingEnabled()) return false;
  if (d.isImage) return false;
  if (!d.dirty) {
    showToast('', 'No changes to save');
    return true;
  }
  if (!(await ensureBuffer(d))) return false;
  try {
    const j = await apiPostJson('/api/save', {
      path: d.path,
      content: serialize(d),
      mtime: d.mtime || '',
      force: !!force,
    });
    d.dirty = false;
    d.mtime = j.mtime || d.mtime;
    d.size = j.size || d.size;
    d.original = serialize(d);
    d.undo = [];
    d.redo = [];
    d._lastOp = '';
    d.chunks = new Set();
    d.pending = new Set();
    d.refining = new Set();
    d.lines = new Array(d.total);
    d.gen = (d.gen || 0) + 1;
    drawTabs();
    layout();
    render();
    updateStatus();
    try {
      await api('/api/reindex');
      await refreshTree();
      await loadGutter(d);
    } catch {}
    showToast('✓', 'Saved ' + d.name);
    return true;
  } catch (e) {
    if (e.message === 'file changed on disk') {
      if (confirm(d.name + ' changed on disk. Overwrite?')) return saveActive(true);
      return false;
    }
    showToast('!', e.message || 'Save failed');
    return false;
  }
}

function applyKey(e) {
  const d = doc_();
  if (!d || !d.text) return false;
  const mod = e[MOD];

  if (mod && !e.altKey && (e.key === 's' || e.key === 'S')) {
    e.preventDefault();
    saveActive();
    return true;
  }
  if (mod && !e.altKey && !e.shiftKey && (e.key === 'z' || e.key === 'Z')) {
    e.preventDefault();
    undo();
    return true;
  }
  if (mod && !e.altKey && ((e.shiftKey && (e.key === 'z' || e.key === 'Z')) || e.key === 'y' || e.key === 'Y')) {
    e.preventDefault();
    redo();
    return true;
  }
  if (mod && !e.altKey && (e.key === 'x' || e.key === 'X')) {
    const sel = selRange(d);
    if (!sel) return false;
    e.preventDefault();
    const text = d.text.slice(sel.start.line - 1, sel.end.line).map((ln, i, arr) => {
      if (arr.length === 1) return ln.slice(sel.start.col, sel.end.col);
      if (i === 0) return ln.slice(sel.start.col);
      if (i === arr.length - 1) return ln.slice(0, sel.end.col);
      return ln;
    }).join('\n');
    copyToClipboard(text, 'Cut');
    deleteSelection();
    return true;
  }

  if (mod || e.altKey) return false;

  if (e.key === 'Backspace') { e.preventDefault(); backspace(); return true; }
  if (e.key === 'Delete') { e.preventDefault(); deleteForward(); return true; }
  if (e.key === 'Enter') { e.preventDefault(); newline(); return true; }
  if (e.key === 'Tab') { e.preventDefault(); insertText('\t', 'type'); return true; }
  if (e.key.length === 1) { e.preventDefault(); insertText(e.key, 'type'); return true; }
  return false;
}

export function handleEditKeyDown(e) {
  if (!editingEnabled()) return false;
  const d = doc_();
  if (!canEditDoc(d)) return false;
  if (isVimEnabled() && getVimMode() !== 'INSERT') {
    // Cmd+S still saves from Normal mode.
    if (e[MOD] && !e.altKey && (e.key === 's' || e.key === 'S')) {
      e.preventDefault();
      saveActive();
      return true;
    }
    return false;
  }

  const wantsBuffer = e.key === 'Backspace' || e.key === 'Delete' || e.key === 'Enter' || e.key === 'Tab' ||
    e.key.length === 1 || (e[MOD] && /[szyxSZYX]/.test(e.key));
  if (!wantsBuffer) return false;

  if (!d.text) {
    e.preventDefault();
    queued.push({
      key: e.key, code: e.code, altKey: e.altKey, shiftKey: e.shiftKey,
      ctrlKey: e.ctrlKey, metaKey: e.metaKey,
    });
    if (!loadP) {
      loadP = ensureBuffer(d).then(ok => {
        const keys = queued.splice(0);
        loadP = null;
        if (!ok) return;
        for (const k of keys) applyKey(k);
      });
    }
    return true;
  }
  return applyKey(e);
}

export function initEdit() {
  addEventListener('beforeunload', e => {
    if (S.tabs.some(t => t.dirty)) {
      e.preventDefault();
      e.returnValue = '';
    }
  });
  vp.addEventListener('paste', e => {
    if (!canEditDoc() || (isVimEnabled() && getVimMode() !== 'INSERT')) return;
    const text = e.clipboardData?.getData('text/plain');
    if (!text) return;
    e.preventDefault();
    const run = async () => {
      const d = doc_();
      if (!(await ensureBuffer(d))) return;
      insertText(text, 'paste');
    };
    run();
  });
}
