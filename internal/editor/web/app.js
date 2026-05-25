// lono studio — a framework-free editor for lono game definitions.
// The whole app is driven by one working definition object (S.def); section
// editors mutate it directly. Text edits call touched() (no re-render, so focus
// is preserved); structural edits (add/remove/retype) call refresh().

// ---------- tiny DOM + fetch helpers ----------
const $ = (sel, root = document) => root.querySelector(sel);

function h(tag, attrs, ...kids) {
  const e = document.createElement(tag);
  if (attrs) for (const [k, v] of Object.entries(attrs)) {
    if (v == null || v === false) continue;
    if (k === 'class') e.className = v;
    else if (k === 'html') e.innerHTML = v;
    else if (k.startsWith('on') && typeof v === 'function') e.addEventListener(k.slice(2), v);
    else if (v === true) e.setAttribute(k, '');
    else e.setAttribute(k, v);
  }
  for (const kid of kids.flat()) {
    if (kid == null || kid === false) continue;
    e.appendChild(typeof kid === 'object' ? kid : document.createTextNode(String(kid)));
  }
  return e;
}

async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : {};
  if (!res.ok) throw Object.assign(new Error(data.error || res.statusText), { data });
  return data;
}

let toastTimer;
function toast(msg, kind = '') {
  const t = $('#toast');
  t.textContent = msg;
  t.className = kind;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.add('hidden'), 2600);
}

const debounce = (fn, ms) => { let t; return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); }; };

// ---------- store ----------
const S = {
  meta: null, files: [], file: null, def: null,
  validation: [], dirty: false, section: 'overview',
  subtab: { types: 'entityTypes', cast: 'entities', systems: 'triggers', map: 'layout' },
  mapSel: null, mapTab: 'layout', mapZoom: 1, mapScroll: null,
  charSel: null, itemSel: null,
  storyTab: null, storySel: null, storyZoom: 1, storyScroll: null,
  pt: null, ptSeed: 42,
};

// Designer-concept navigation: build the cast, items and world; assemble the
// story; reach for the raw schema only when you need it.
const NAV = [
  ['Build', [['overview', 'Overview'], ['characters', 'Characters'], ['items', 'Items'], ['map', 'Map']]],
  ['Story', [['story', 'Story flow'], ['beats', 'Beats'], ['lore', 'Lore']]],
  ['Advanced', [['types', 'Types'], ['world', 'World'], ['systems', 'Systems'], ['json', 'JSON']]],
];
const SECTION_LABEL = Object.fromEntries(NAV.flatMap(([, items]) => items));

function counts() {
  const d = S.def || {};
  const n = (o) => o ? (Array.isArray(o) ? o.length : Object.keys(o).length) : 0;
  const pt = d.entityTypes ? detectPlaceType(d) : '';
  const chars = Object.values(d.entities || {}).filter(e => e.type !== pt).length;
  return {
    characters: chars,
    items: n(d.itemTypes),
    map: placesCount(d),
    story: n(d.machines),
    beats: n(d.beats),
    lore: n(d.lore),
    types: n(d.entityTypes) + n(d.itemTypes) + n(d.relationshipTypes),
    world: n(d.world) + n(d.setup),
    systems: n(d.triggers) + n(d.derived),
  };
}

// ---------- name helpers (for dropdowns) ----------
const keys = (o) => o ? Object.keys(o) : [];
const entityTypeNames = () => keys(S.def.entityTypes);
const itemTypeNames = () => keys(S.def.itemTypes);
const relTypeNames = () => keys(S.def.relationshipTypes);
const entityNames = () => keys(S.def.entities);
const machineNames = () => keys(S.def.machines);
const stateNames = (m) => (S.def.machines && S.def.machines[m] && S.def.machines[m].states) || [];
function categories() {
  const set = new Set();
  for (const it of Object.values(S.def.itemTypes || {})) if (it.category) set.add(it.category);
  for (const et of Object.values(S.def.entityTypes || {})) for (const sl of Object.values(et.slots || {})) (sl.accepts || []).forEach(c => set.add(c));
  return [...set];
}

// ---------- any-value <-> input text ----------
function valToInput(v) {
  if (v === undefined || v === null) return '';
  if (typeof v === 'string') return v;
  return JSON.stringify(v);
}
function inputToVal(s) {
  const t = s.trim();
  if (t === '') return undefined;
  try { return JSON.parse(t); } catch { return s; }
}

// ---------- generic field builders ----------
function field(label, control, hint) {
  return h('div', { class: 'field' },
    label ? h('label', {}, label) : null,
    control,
    hint ? h('div', { class: 'hint' }, hint) : null);
}

function textInput(obj, key, { ph, type = 'text', onChange } = {}) {
  const inp = h('input', { type, value: obj[key] ?? '', placeholder: ph || '' });
  inp.addEventListener('input', () => {
    if (type === 'number') obj[key] = inp.value === '' ? undefined : Number(inp.value);
    else obj[key] = inp.value === '' ? undefined : inp.value;
    onChange ? onChange() : touched();
  });
  return inp;
}

function textArea(obj, key, rows = 3) {
  const ta = h('textarea', { rows });
  ta.value = obj[key] ?? '';
  ta.addEventListener('input', () => { obj[key] = ta.value === '' ? undefined : ta.value; touched(); });
  return ta;
}

function checkbox(obj, key, label) {
  const cb = h('input', { type: 'checkbox' });
  cb.checked = !!obj[key];
  cb.addEventListener('change', () => { obj[key] = cb.checked || undefined; touched(); });
  return h('label', { class: 'checkbox' }, cb, label);
}

function selectInput(obj, key, options, { allowEmpty = true, emptyLabel = '— none —', onChange } = {}) {
  const sel = h('select', {});
  if (allowEmpty) sel.appendChild(h('option', { value: '' }, emptyLabel));
  for (const o of options) {
    const val = typeof o === 'string' ? o : o.value;
    const lab = typeof o === 'string' ? o : o.label;
    const opt = h('option', { value: val }, lab);
    if ((obj[key] ?? '') === val) opt.selected = true;
    sel.appendChild(opt);
  }
  sel.addEventListener('change', () => { obj[key] = sel.value === '' ? undefined : sel.value; onChange ? onChange() : touched(); });
  return sel;
}

function valueInput(obj, key, ph) {
  const inp = h('input', { type: 'text', value: valToInput(obj[key]), placeholder: ph || 'JSON value' });
  inp.addEventListener('input', () => { obj[key] = inputToVal(inp.value); touched(); });
  return inp;
}

// string list (states, enum values, accepts, tags)
function stringList(arr, { ph = 'value', onStructure } = {}) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    arr.forEach((_, i) => {
      const inp = h('input', { type: 'text', value: arr[i], placeholder: ph });
      inp.addEventListener('input', () => { arr[i] = inp.value; touched(); });
      wrap.appendChild(h('div', { class: 'list-row' }, inp,
        h('button', { class: 'tiny del', onclick: () => { arr.splice(i, 1); redraw(); onStructure ? onStructure() : touched(); } }, '✕')));
    });
    wrap.appendChild(h('button', { class: 'tiny add-btn', onclick: () => { arr.push(''); redraw(); touched(); } }, '+ add'));
  };
  redraw();
  return wrap;
}

// list of arbitrary JSON values (literals or {"$path":"…"}), e.g. check modifiers
function valueList(arr) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    arr.forEach((_, i) => {
      const inp = h('input', { type: 'text', value: valToInput(arr[i]), placeholder: 'number or {"$path":"entity.x.skill"}' });
      inp.addEventListener('input', () => { const v = inputToVal(inp.value); arr[i] = v === undefined ? 0 : v; touched(); });
      wrap.appendChild(h('div', { class: 'list-row' }, inp,
        h('button', { class: 'tiny del', onclick: () => { arr.splice(i, 1); redraw(); touched(); } }, '✕')));
    });
    wrap.appendChild(h('button', { class: 'tiny add-btn', onclick: () => { arr.push(0); redraw(); touched(); } }, '+ add modifier'));
  };
  redraw();
  return wrap;
}

// key/value map editor. valueKind: 'value' | 'int' | 'string-item' (item dropdown) | 'string'
function kvEditor(obj, { valueKind = 'value', keyPh = 'key', options = null } = {}) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    for (const k of Object.keys(obj)) {
      const keyInp = h('input', { class: 'key', type: 'text', value: k });
      keyInp.addEventListener('change', () => {
        const nk = keyInp.value.trim();
        if (nk && nk !== k && !(nk in obj)) { obj[nk] = obj[k]; delete obj[k]; redraw(); touched(); }
        else keyInp.value = k;
      });
      let valCtl;
      if (valueKind === 'int') {
        valCtl = h('input', { type: 'number', value: obj[k] });
        valCtl.addEventListener('input', () => { obj[k] = Number(valCtl.value || 0); touched(); });
      } else if (options) {
        valCtl = selectInput(obj, k, options, { allowEmpty: false });
      } else if (valueKind === 'string') {
        valCtl = h('input', { type: 'text', value: obj[k] ?? '' });
        valCtl.addEventListener('input', () => { obj[k] = valCtl.value; touched(); });
      } else {
        valCtl = h('input', { type: 'text', value: valToInput(obj[k]) });
        valCtl.addEventListener('input', () => { obj[k] = inputToVal(valCtl.value); touched(); });
      }
      wrap.appendChild(h('div', { class: 'kv-row' }, keyInp, valCtl,
        h('button', { class: 'tiny del', onclick: () => { delete obj[k]; redraw(); touched(); } }, '✕')));
    }
    wrap.appendChild(h('button', { class: 'tiny add-btn', onclick: () => {
      let i = 1, nk = 'key'; while ((nk) in obj) nk = 'key' + (++i);
      obj[nk] = valueKind === 'int' ? 0 : (options ? options[0] : '');
      redraw(); touched();
    } }, '+ add'));
  };
  redraw();
  return wrap;
}

// ---------- VarSpec editor ----------
function varSpecEditor(spec) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    const typeRow = h('div', { class: 'row' },
      field('type', selectInput(spec, 'type', S.meta.varTypes, { allowEmpty: false, onChange: () => { redraw(); touched(); } })));
    wrap.appendChild(typeRow);
    const t = spec.type;
    const extra = h('div', { class: 'row' });
    if (t === 'int' || t === 'float') {
      extra.appendChild(field('default', valueInput(spec, 'default', 'e.g. 0')));
      extra.appendChild(field('min', textInput(spec, 'min', { type: 'number' })));
      extra.appendChild(field('max', textInput(spec, 'max', { type: 'number' })));
    } else if (t === 'string') {
      extra.appendChild(field('default', textInput(spec, 'default')));
    } else if (t === 'bool') {
      extra.appendChild(field('default', selectInput(spec, 'default', [{ value: 'true', label: 'true' }, { value: 'false', label: 'false' }], { emptyLabel: '— unset —', onChange: () => { spec.default = spec.default === 'true' ? true : spec.default === 'false' ? false : undefined; touched(); } })));
    } else if (t === 'ref') {
      extra.appendChild(field('refType', selectInput(spec, 'refType', entityTypeNames())));
      extra.appendChild(field('default', textInput(spec, 'default', { ph: 'entity id' })));
    } else if (t === 'enum') {
      spec.values = spec.values || [];
      extra.appendChild(field('values', stringList(spec.values, { ph: 'option' })));
      extra.appendChild(field('default', textInput(spec, 'default')));
    } else if (t === 'set') {
      extra.appendChild(field('elem', selectInput(spec, 'elem', S.meta.setElems, { allowEmpty: false })));
    }
    if (extra.children.length) wrap.appendChild(extra);
  };
  redraw();
  return wrap;
}

// varSpec map (world vars, entity-type / rel-type attributes)
function varSpecMap(obj) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    for (const name of Object.keys(obj)) {
      wrap.appendChild(h('div', { class: 'subcard' },
        h('div', { class: 'kv-row' },
          renameInput(obj, name, redraw),
          h('span', { class: 'spacer', style: 'flex:1' }),
          h('button', { class: 'tiny del', onclick: () => { delete obj[name]; redraw(); touched(); } }, '✕ remove')),
        varSpecEditor(obj[name])));
    }
    wrap.appendChild(h('button', { class: 'add-btn', onclick: () => {
      let i = 1, nk = 'var'; while (nk in obj) nk = 'var' + (++i);
      obj[nk] = { type: 'int' }; redraw(); touched();
    } }, '+ add variable'));
  };
  redraw();
  return wrap;
}

function renameInput(obj, name, redraw) {
  const inp = h('input', { class: 'key', type: 'text', value: name, title: 'rename' });
  inp.addEventListener('change', () => {
    const nk = inp.value.trim();
    if (nk && nk !== name && !(nk in obj)) { obj[nk] = obj[name]; delete obj[name]; redraw(); touched(); }
    else inp.value = name;
  });
  return inp;
}

// ---------- Guard editor ----------
// optional guard at parent[key] (may be absent)
function optionalGuard(parent, key, label) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    const present = parent[key] && typeof parent[key] === 'object';
    const head = h('div', { class: 'inline' },
      h('label', { style: 'color:var(--muted);font-size:12px' }, label || 'guard'),
      present
        ? h('button', { class: 'tiny del', onclick: () => { delete parent[key]; redraw(); touched(); } }, '✕ remove guard')
        : h('button', { class: 'tiny', onclick: () => { parent[key] = { target: '', op: 'eq', value: true }; redraw(); touched(); } }, '+ add guard'));
    wrap.appendChild(head);
    if (present) wrap.appendChild(h('div', { class: 'guard' }, guardNode(parent[key], (g) => { parent[key] = g; redraw(); touched(); })));
  };
  redraw();
  return wrap;
}

function guardKind(g) {
  if (g.and) return 'and';
  if (g.or) return 'or';
  if (g.not) return 'not';
  return 'leaf';
}

// guardNode renders a single (non-null) guard; onChange replaces it.
function guardNode(g, onChange) {
  const wrap = h('div', {});
  const kind = guardKind(g);
  const kindSel = selectInput({ k: kind }, 'k',
    [{ value: 'leaf', label: 'condition' }, { value: 'and', label: 'all of (AND)' }, { value: 'or', label: 'any of (OR)' }, { value: 'not', label: 'NOT' }],
    { allowEmpty: false, onChange: () => {
      const nk = kindSel.value;
      let ng = {};
      if (nk === 'leaf') ng = { target: '', op: 'eq', value: true };
      else if (nk === 'and') ng = { and: [{ target: '', op: 'eq', value: true }] };
      else if (nk === 'or') ng = { or: [{ target: '', op: 'eq', value: true }] };
      else if (nk === 'not') ng = { not: { target: '', op: 'eq', value: true } };
      onChange(ng);
    } });
  wrap.appendChild(h('div', { class: 'inline', style: 'margin-bottom:6px' }, kindSel));

  if (kind === 'leaf') {
    const opSel = selectInput(g, 'op', S.meta.guardOps, { allowEmpty: false, onChange: () => { touched(); rerenderLeaf(); } });
    const tgt = h('input', { type: 'text', value: g.target || '', placeholder: 'path, e.g. world.alarm or roll.snk' });
    tgt.addEventListener('input', () => { g.target = tgt.value; touched(); });
    const leaf = h('div', { class: 'row' });
    const valWrap = h('div', { class: 'field', style: 'flex:1' });
    const rerenderLeaf = () => {
      valWrap.innerHTML = '';
      if (g.op !== 'exists') {
        valWrap.appendChild(h('label', {}, 'value'));
        valWrap.appendChild(valueInput(g, 'value', g.op === 'contains' ? 'member (string)' : 'JSON value'));
      }
    };
    leaf.appendChild(field('target', tgt));
    leaf.appendChild(field('op', opSel));
    leaf.appendChild(valWrap);
    rerenderLeaf();
    wrap.appendChild(leaf);
  } else if (kind === 'and' || kind === 'or') {
    const arr = g[kind];
    const list = h('div', { class: 'guard' });
    const redrawList = () => {
      list.innerHTML = '';
      arr.forEach((child, i) => {
        list.appendChild(h('div', { class: 'subcard' },
          h('div', { class: 'inline', style: 'justify-content:flex-end' },
            h('button', { class: 'tiny del', onclick: () => { arr.splice(i, 1); redrawList(); touched(); } }, '✕')),
          guardNode(child, (ng) => { arr[i] = ng; touched(); })));
      });
      list.appendChild(h('button', { class: 'tiny add-btn', onclick: () => { arr.push({ target: '', op: 'eq', value: true }); redrawList(); touched(); } }, '+ add condition'));
    };
    redrawList();
    wrap.appendChild(list);
  } else if (kind === 'not') {
    wrap.appendChild(h('div', { class: 'guard' }, guardNode(g.not, (ng) => { g.not = ng; touched(); })));
  }
  return wrap;
}

// ---------- Effects editor ----------
function effectOpSpec(op) { return S.meta.effectOps.find(e => e.op === op); }

function pruneEffect(eff) {
  const spec = effectOpSpec(eff.op);
  const keep = new Set(['op']);
  if (spec) for (const f of spec.fields) keep.add(f.name);
  for (const k of Object.keys(eff)) if (!keep.has(k)) delete eff[k];
}

function effectsEditor(arr) {
  const wrap = h('div', { class: 'effects' });
  const redraw = () => {
    wrap.innerHTML = '';
    if (!arr.length) wrap.appendChild(h('div', { class: 'empty' }, 'no effects'));
    arr.forEach((eff, i) => wrap.appendChild(effectRow(eff, i, arr, redraw)));
    wrap.appendChild(h('button', { class: 'tiny add-btn', onclick: () => { arr.push({ op: 'set' }); redraw(); touched(); } }, '+ add effect'));
  };
  redraw();
  return wrap;
}

function groupedOpOptions() {
  // produce <optgroup>-friendly flat options with group prefix; we build manually
  return S.meta.effectOps;
}

function effectRow(eff, i, arr, redrawParent) {
  const row = h('div', { class: 'effect-row' });
  const draw = () => {
    row.innerHTML = '';
    // op selector grouped
    const sel = h('select', {});
    let curGroup = '';
    let grp;
    for (const o of groupedOpOptions()) {
      if (o.group !== curGroup) { curGroup = o.group; grp = h('optgroup', { label: curGroup }); sel.appendChild(grp); }
      const opt = h('option', { value: o.op }, o.op);
      if (o.op === eff.op) opt.selected = true;
      grp.appendChild(opt);
    }
    sel.addEventListener('change', () => { eff.op = sel.value; pruneEffect(eff); draw(); touched(); });

    row.appendChild(h('div', { class: 'effect-head' },
      sel,
      h('span', { class: 'spacer', style: 'flex:1' }),
      i > 0 ? h('button', { class: 'tiny', title: 'move up', onclick: () => { [arr[i - 1], arr[i]] = [arr[i], arr[i - 1]]; redrawParent(); touched(); } }, '↑') : null,
      i < arr.length - 1 ? h('button', { class: 'tiny', title: 'move down', onclick: () => { [arr[i + 1], arr[i]] = [arr[i], arr[i + 1]]; redrawParent(); touched(); } }, '↓') : null,
      h('button', { class: 'tiny del', onclick: () => { arr.splice(i, 1); redrawParent(); touched(); } }, '✕')));

    const spec = effectOpSpec(eff.op);
    if (spec) {
      row.appendChild(h('div', { class: 'op-doc' }, spec.doc));
      for (const f of spec.fields) row.appendChild(effectField(eff, f));
    }
  };
  draw();
  return row;
}

function effectField(eff, f) {
  switch (f.kind) {
    case 'int': return field(f.name, textInput(eff, f.name, { type: 'number' }), f.doc);
    case 'string': {
      // helpful dropdowns for some known fields
      const opts = fieldOptions(f.name);
      if (opts) return field(f.name, selectInput(eff, f.name, opts), f.doc);
      return field(f.name, textInput(eff, f.name), f.doc);
    }
    case 'value': return field(f.name, valueInput(eff, f.name), f.doc);
    case 'values': { eff[f.name] = eff[f.name] || []; return field(f.name, valueList(eff[f.name]), f.doc); }
    case 'tags': { eff[f.name] = eff[f.name] || []; return field(f.name, stringList(eff[f.name], { ph: 'tag' }), f.doc); }
    case 'attrs': { eff[f.name] = eff[f.name] || {}; return field(f.name, kvEditor(eff[f.name], { valueKind: 'value' }), f.doc); }
    case 'guard': {
      const holder = {}; holder.g = eff[f.name];
      return field(f.name + ' (when)', (() => {
        if (!eff[f.name]) eff[f.name] = { target: '', op: 'eq', value: true };
        return guardNode(eff[f.name], (ng) => { eff[f.name] = ng; touched(); });
      })(), f.doc);
    }
    case 'effects': { eff[f.name] = eff[f.name] || []; return field(f.name, effectsEditor(eff[f.name]), f.doc); }
    default: return field(f.name, textInput(eff, f.name), f.doc);
  }
}

// soft dropdowns for common effect string fields
function fieldOptions(name) {
  switch (name) {
    case 'entity': return entityNames();
    case 'item': return itemTypeNames();
    case 'entityType': return entityTypeNames();
    case 'relType': return relTypeNames();
    case 'machine': return machineNames();
    case 'beat': return keys(S.def.beats);
    case 'lore': return keys(S.def.lore);
    default: return null;
  }
}

// ---------- card / collection helpers ----------
function mapSection(obj, { noun, renderBody, makeNew }) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    const names = Object.keys(obj);
    if (!names.length) wrap.appendChild(h('div', { class: 'empty' }, `No ${noun}s yet.`));
    for (const name of names) {
      wrap.appendChild(h('div', { class: 'card' },
        h('div', { class: 'card-head' },
          renameInput(obj, name, redraw),
          h('span', { class: 'spacer', style: 'flex:1' }),
          h('button', { class: 'tiny', title: 'duplicate', onclick: () => {
            let i = 2, nk = name + '_copy'; while (nk in obj) nk = name + '_copy' + (i++);
            obj[nk] = JSON.parse(JSON.stringify(obj[name])); redraw(); touched();
          } }, 'duplicate'),
          h('button', { class: 'tiny del', onclick: () => { delete obj[name]; redraw(); touched(); } }, '✕ delete')),
        renderBody(obj[name], name)));
    }
    wrap.appendChild(h('button', { class: 'primary add-btn', onclick: () => {
      const base = noun.replace(/\s+/g, '_'); let i = 1, nk = base; while (nk in obj) nk = base + (++i);
      obj[nk] = makeNew(); redraw(); touched();
    } }, `+ new ${noun}`));
  };
  redraw();
  return wrap;
}

function listSection(arr, { noun, renderBody, makeNew }) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    if (!arr.length) wrap.appendChild(h('div', { class: 'empty' }, `No ${noun}s yet.`));
    arr.forEach((item, i) => {
      wrap.appendChild(h('div', { class: 'card' },
        h('div', { class: 'card-head' },
          h('span', { class: 'title' }, `${noun} ${i + 1}`),
          h('span', { class: 'spacer', style: 'flex:1' }),
          h('button', { class: 'tiny del', onclick: () => { arr.splice(i, 1); redraw(); touched(); } }, '✕ delete')),
        renderBody(item, i)));
    });
    wrap.appendChild(h('button', { class: 'primary add-btn', onclick: () => { arr.push(makeNew()); redraw(); touched(); } }, `+ new ${noun}`));
  };
  redraw();
  return wrap;
}

function subTabs(sectionKey, tabs) {
  const bar = h('div', { class: 'tabs' });
  for (const [id, label] of tabs) {
    bar.appendChild(h('div', { class: 'tab' + (S.subtab[sectionKey] === id ? ' active' : ''), onclick: () => { S.subtab[sectionKey] = id; renderMain(); } }, label));
  }
  return bar;
}

// ---------- section renderers ----------
function renderGame(main) {
  const d = S.def;
  main.appendChild(h('div', { class: 'card' },
    field('id', textInput(d, 'id'), 'unique game id (also the default file name)'),
    field('name', textInput(d, 'name')),
    h('div', { class: 'row' }, field('version', textInput(d, 'version', { type: 'number' }))),
    field('description', textArea(d, 'description', 3), 'the pitch — what is this game?'),
    field('intent', textArea(d, 'intent', 2), 'a note to the narrator about tone/goal')));
}

function renderWorld(main) {
  const d = S.def;
  d.world = d.world || {};
  d.setup = d.setup || [];
  main.appendChild(h('h2', {}, 'World variables'));
  main.appendChild(h('p', { class: 'section-blurb' }, 'Global state for the whole game — flags, counters, scores.'));
  main.appendChild(varSpecMap(d.world));
  main.appendChild(h('h2', { style: 'margin-top:24px' }, 'Setup'));
  main.appendChild(h('p', { class: 'section-blurb' }, 'Effects run once when a playthrough begins, after the cast is seeded.'));
  main.appendChild(effectsEditor(d.setup));
}

function renderTypes(main) {
  const d = S.def;
  main.appendChild(subTabs('types', [['entityTypes', 'Entity types'], ['itemTypes', 'Item types'], ['relationshipTypes', 'Relationship types']]));
  const t = S.subtab.types;
  if (t === 'entityTypes') {
    d.entityTypes = d.entityTypes || {};
    main.appendChild(h('p', { class: 'section-blurb' }, 'Kinds of things in the world (characters, objects, locations). Each has typed attributes and optional equipment slots.'));
    main.appendChild(mapSection(d.entityTypes, {
      noun: 'entity type', makeNew: () => ({ description: '', attributes: {} }),
      renderBody: (et) => {
        et.attributes = et.attributes || {}; et.slots = et.slots || {};
        return h('div', {},
          field('description', textArea(et, 'description', 2)),
          field('intent', textInput(et, 'intent')),
          h('div', { class: 'pt-section-label' }, 'Attributes'),
          varSpecMap(et.attributes),
          h('div', { class: 'pt-section-label' }, 'Equipment slots'),
          slotsEditor(et.slots));
      },
    }));
  } else if (t === 'itemTypes') {
    d.itemTypes = d.itemTypes || {};
    main.appendChild(h('p', { class: 'section-blurb' }, 'Things that can sit in inventories or be equipped. Category + equippable decide which slots accept them.'));
    main.appendChild(mapSection(d.itemTypes, {
      noun: 'item type', makeNew: () => ({ description: '' }),
      renderBody: (it) => {
        it.attributes = it.attributes || {};
        return h('div', {},
          field('description', textArea(it, 'description', 2)),
          h('div', { class: 'row' },
            field('category', textInput(it, 'category'), 'e.g. weapon, torch, dress'),
            field('maxStack', textInput(it, 'maxStack', { type: 'number' }), 'optional stack cap')),
          h('div', { class: 'field' }, checkbox(it, 'equippable', 'equippable')),
          h('div', { class: 'pt-section-label' }, 'Attributes (free-form)'),
          kvEditor(it.attributes, { valueKind: 'value' }));
      },
    }));
  } else {
    d.relationshipTypes = d.relationshipTypes || {};
    main.appendChild(h('p', { class: 'section-blurb' }, 'Edges between entities — friendships, trust, exits between rooms. Carry their own typed attributes (the axes).'));
    main.appendChild(mapSection(d.relationshipTypes, {
      noun: 'relationship type', makeNew: () => ({ from: entityTypeNames()[0] || '', to: entityTypeNames()[0] || '', directed: true, attributes: {} }),
      renderBody: (rt) => {
        rt.attributes = rt.attributes || {};
        return h('div', {},
          field('description', textArea(rt, 'description', 2)),
          h('div', { class: 'row' },
            field('from (entity type)', selectInput(rt, 'from', entityTypeNames(), { allowEmpty: false })),
            field('to (entity type)', selectInput(rt, 'to', entityTypeNames(), { allowEmpty: false }))),
          h('div', { class: 'field' }, checkbox(rt, 'directed', 'directed')),
          h('div', { class: 'pt-section-label' }, 'Attributes (axes)'),
          varSpecMap(rt.attributes));
      },
    }));
  }
}

function slotsEditor(slots) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    for (const name of Object.keys(slots)) {
      const sl = slots[name]; sl.accepts = sl.accepts || [];
      wrap.appendChild(h('div', { class: 'subcard' },
        h('div', { class: 'kv-row' }, renameInput(slots, name, redraw), h('span', { style: 'flex:1' }),
          h('button', { class: 'tiny del', onclick: () => { delete slots[name]; redraw(); touched(); } }, '✕')),
        field('description', textInput(sl, 'description')),
        h('label', { style: 'font-size:12px;color:var(--muted)' }, 'accepts categories'),
        stringList(sl.accepts, { ph: 'category' })));
    }
    wrap.appendChild(h('button', { class: 'tiny add-btn', onclick: () => {
      let i = 1, nk = 'slot'; while (nk in slots) nk = 'slot' + (++i);
      slots[nk] = { accepts: [] }; redraw(); touched();
    } }, '+ add slot'));
  };
  redraw();
  return wrap;
}

function renderCast(main) {
  const d = S.def;
  main.appendChild(subTabs('cast', [['entities', 'Entities (cast)'], ['relationships', 'Relationships']]));
  if (S.subtab.cast === 'entities') {
    d.entities = d.entities || {};
    main.appendChild(h('p', { class: 'section-blurb' }, 'The specific characters, objects, and places that exist when the story starts.'));
    main.appendChild(mapSection(d.entities, {
      noun: 'entity', makeNew: () => ({ type: entityTypeNames()[0] || '', attrs: {} }),
      renderBody: (e) => {
        e.attrs = e.attrs || {}; e.inventory = e.inventory || {}; e.equipped = e.equipped || {};
        return h('div', {},
          h('div', { class: 'row' }, field('type', selectInput(e, 'type', entityTypeNames(), { allowEmpty: false }))),
          field('description', textArea(e, 'description', 2), 'authored prose for this specific entity'),
          h('div', { class: 'pt-section-label' }, 'Attributes'),
          kvEditor(e.attrs, { valueKind: 'value', keyPh: 'attr' }),
          h('div', { class: 'pt-section-label' }, 'Inventory (item → count)'),
          kvEditor(e.inventory, { valueKind: 'int' }),
          h('div', { class: 'pt-section-label' }, 'Equipped (slot → item)'),
          kvEditor(e.equipped, { valueKind: 'string' }));
      },
    }));
  } else {
    d.relationships = d.relationships || [];
    main.appendChild(h('p', { class: 'section-blurb' }, 'Starting edges between cast members (and map exits between locations).'));
    main.appendChild(listSection(d.relationships, {
      noun: 'relationship', makeNew: () => ({ type: relTypeNames()[0] || '', from: entityNames()[0] || '', to: entityNames()[0] || '', attrs: {} }),
      renderBody: (r) => {
        r.attrs = r.attrs || {};
        return h('div', {},
          h('div', { class: 'row' },
            field('type', selectInput(r, 'type', relTypeNames(), { allowEmpty: false })),
            field('from', selectInput(r, 'from', entityNames(), { allowEmpty: false })),
            field('to', selectInput(r, 'to', entityNames(), { allowEmpty: false }))),
          h('div', { class: 'pt-section-label' }, 'Attributes'),
          kvEditor(r.attrs, { valueKind: 'value' }));
      },
    }));
  }
}

function renderStory(main) {
  const d = S.def; d.machines = d.machines || {};
  main.appendChild(h('p', { class: 'section-blurb' }, 'State machines drive the story: scenes/arcs and per-character or per-relationship machines. Endings are terminal states.'));
  main.appendChild(mapSection(d.machines, {
    noun: 'machine', makeNew: () => ({ initial: 'start', states: ['start', 'end'], stateMeta: {}, transitions: [] }),
    renderBody: (m) => machineEditor(m),
  }));
}

function machineEditor(m) {
  m.states = m.states || []; m.stateMeta = m.stateMeta || {}; m.transitions = m.transitions || [];
  const wrap = h('div', {});
  wrap.appendChild(field('description', textArea(m, 'description', 2)));
  wrap.appendChild(field('intent', textInput(m, 'intent')));

  // attach
  const attach = m.attach || {};
  const [akind, aname] = (attach.to || '').includes(':') ? attach.to.split(':') : ['', ''];
  const attachState = { kind: akind, name: aname };
  const applyAttach = () => {
    if (!attachState.kind) delete m.attach;
    else m.attach = { to: attachState.kind + ':' + (attachState.name || '') };
    touched();
  };
  const kindSel = selectInput(attachState, 'kind', [{ value: 'entityType', label: 'entityType' }, { value: 'relationshipType', label: 'relationshipType' }], { emptyLabel: '— global (none) —', onChange: applyAttach });
  const nameSel = (() => {
    const opts = attachState.kind === 'relationshipType' ? relTypeNames() : entityTypeNames();
    const s = selectInput(attachState, 'name', opts, { allowEmpty: false });
    s.addEventListener('change', applyAttach);
    return s;
  })();
  wrap.appendChild(h('div', { class: 'row' },
    field('attach to', kindSel, 'leave global for the main arc; attach to make a per-host machine'),
    attachState.kind ? field('host type', nameSel) : null));

  // states + their meta (endings)
  wrap.appendChild(h('div', { class: 'pt-section-label' }, 'States & endings'));
  wrap.appendChild(field('initial state', selectInput(m, 'initial', m.states, { allowEmpty: false })));
  const statesWrap = h('div', {});
  const drawStates = () => {
    statesWrap.innerHTML = '';
    m.states.forEach((st, i) => {
      const meta = m.stateMeta[st] || {};
      const nameInp = h('input', { class: 'key', type: 'text', value: st });
      nameInp.addEventListener('change', () => {
        const nn = nameInp.value.trim();
        if (nn && nn !== st) {
          m.states[i] = nn;
          if (m.stateMeta[st]) { m.stateMeta[nn] = m.stateMeta[st]; delete m.stateMeta[st]; }
          if (m.initial === st) m.initial = nn;
          drawStates(); touched();
        }
      });
      const ensureMeta = () => (m.stateMeta[st] = m.stateMeta[st] || {});
      const term = h('input', { type: 'checkbox' }); term.checked = !!meta.terminal;
      term.addEventListener('change', () => { ensureMeta().terminal = term.checked || undefined; touched(); });
      const end = h('input', { type: 'checkbox' }); end.checked = !!meta.ending;
      end.addEventListener('change', () => { ensureMeta().ending = end.checked || undefined; touched(); });
      const descTa = h('textarea', { rows: 2, placeholder: 'state description (shown to narrator; required-ish for endings)' });
      descTa.value = meta.description || '';
      descTa.addEventListener('input', () => { ensureMeta().description = descTa.value || undefined; touched(); });
      statesWrap.appendChild(h('div', { class: 'subcard' },
        h('div', { class: 'kv-row' }, nameInp,
          h('label', { class: 'checkbox' }, term, 'terminal'),
          h('label', { class: 'checkbox' }, end, 'ending'),
          h('button', { class: 'tiny del', onclick: () => { m.states.splice(i, 1); delete m.stateMeta[st]; drawStates(); touched(); } }, '✕')),
        descTa));
    });
    statesWrap.appendChild(h('button', { class: 'tiny add-btn', onclick: () => {
      let i = 1, nk = 'state'; while (m.states.includes(nk)) nk = 'state' + (++i);
      m.states.push(nk); drawStates(); touched();
    } }, '+ add state'));
  };
  drawStates();
  wrap.appendChild(statesWrap);

  // transitions
  wrap.appendChild(h('div', { class: 'pt-section-label' }, 'Transitions (actions)'));
  wrap.appendChild(listSection(m.transitions, {
    noun: 'transition', makeNew: () => ({ id: 'action', from: m.initial || (m.states[0] || ''), to: m.states[0] || '' }),
    renderBody: (tr) => transitionEditor(tr, m),
  }));
  return wrap;
}

function transitionEditor(tr, m) {
  const wrap = h('div', {});
  wrap.appendChild(h('div', { class: 'row' },
    field('id (action name)', textInput(tr, 'id')),
    field('to state', selectInput(tr, 'to', m.states, { allowEmpty: false }))));
  wrap.appendChild(field('description', textInput(tr, 'description')));
  // from: support single, list, or "*"
  const fromArr = Array.isArray(tr.from) ? tr.from.slice() : (tr.from ? [tr.from] : []);
  const fromHolder = { arr: fromArr };
  const syncFrom = () => { tr.from = fromHolder.arr.length === 1 ? fromHolder.arr[0] : fromHolder.arr.slice(); touched(); };
  wrap.appendChild(field('from state(s)', (() => {
    const box = h('div', {});
    const draw = () => {
      box.innerHTML = '';
      fromHolder.arr.forEach((fs, i) => {
        const sel = selectInput({ v: fs }, 'v', [...m.states, { value: '*', label: '* (any state)' }], { allowEmpty: false });
        sel.addEventListener('change', () => { fromHolder.arr[i] = sel.value; syncFrom(); });
        box.appendChild(h('div', { class: 'list-row' }, sel, h('button', { class: 'tiny del', onclick: () => { fromHolder.arr.splice(i, 1); draw(); syncFrom(); } }, '✕')));
      });
      box.appendChild(h('button', { class: 'tiny add-btn', onclick: () => { fromHolder.arr.push(m.states[0] || ''); draw(); syncFrom(); } }, '+ add from-state'));
    };
    draw();
    return box;
  })(), 'which state(s) this action is available from'));

  wrap.appendChild(h('div', { class: 'pt-section-label' }, 'Guard (must hold for the action to be allowed)'));
  wrap.appendChild(optionalGuard(tr, 'guard', 'guard'));

  wrap.appendChild(h('div', { class: 'pt-section-label' }, 'Parameters'));
  tr.params = tr.params || {};
  wrap.appendChild(varSpecMap(tr.params));

  wrap.appendChild(h('div', { class: 'pt-section-label' }, 'Effects (applied when the action fires)'));
  tr.effects = tr.effects || [];
  wrap.appendChild(effectsEditor(tr.effects));
  return wrap;
}

function renderBeats(main) {
  const d = S.def; d.beats = d.beats || {};
  main.appendChild(h('p', { class: 'section-blurb' }, 'Authored narrative units the engine surfaces when their machine-state and/or guard match. One-shot by default.'));
  main.appendChild(mapSection(d.beats, {
    noun: 'beat', makeNew: () => ({ text: '', once: true }),
    renderBody: (b) => {
      const wrap = h('div', {});
      wrap.appendChild(field('text', textArea(b, 'text', 2)));
      wrap.appendChild(field('intent', textInput(b, 'intent')));
      // machineState binding (optional)
      const hasMS = !!b.machineState;
      const msToggle = h('label', { class: 'checkbox' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = hasMS; c.addEventListener('change', () => { if (c.checked) b.machineState = { machine: machineNames()[0] || '', state: '' }; else delete b.machineState; renderMain(); touched(); }); return c; })(), 'bind to a machine state');
      wrap.appendChild(h('div', { class: 'field' }, msToggle));
      if (b.machineState) {
        const ms = b.machineState;
        wrap.appendChild(h('div', { class: 'row' },
          field('machine', selectInput(ms, 'machine', machineNames(), { allowEmpty: false, onChange: () => { renderMain(); touched(); } })),
          field('state', selectInput(ms, 'state', stateNames(ms.machine), { allowEmpty: false }))));
      }
      wrap.appendChild(h('div', { class: 'field' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = b.once !== false; c.addEventListener('change', () => { b.once = c.checked; touched(); }); return h('label', { class: 'checkbox' }, c, 'one-shot (deliver once)'); })()));
      wrap.appendChild(h('div', { class: 'pt-section-label' }, 'Guard (optional)'));
      wrap.appendChild(optionalGuard(b, 'guard', 'guard'));
      return wrap;
    },
  }));
}

function renderSystems(main) {
  const d = S.def;
  main.appendChild(subTabs('systems', [['triggers', 'Triggers'], ['derived', 'Derived queries']]));
  if (S.subtab.systems === 'triggers') {
    d.triggers = d.triggers || {};
    main.appendChild(h('p', { class: 'section-blurb' }, 'Reactive rules. `when` fires on a condition (edge-triggered); `every` fires periodically on advance. Effects apply automatically.'));
    if (Object.keys(d.triggers).some(k => k.startsWith('scene_')))
      main.appendChild(h('p', { class: 'hint', style: 'margin-top:-10px' }, 'Triggers named scene_… are generated by Map → Scenes; edit them there.'));
    main.appendChild(mapSection(d.triggers, {
      noun: 'trigger', makeNew: () => ({ effects: [], once: true }),
      renderBody: (t) => {
        const wrap = h('div', {});
        wrap.appendChild(field('intent', textInput(t, 'intent')));
        wrap.appendChild(h('div', { class: 'row' }, field('every (ticks)', textInput(t, 'every', { type: 'number' }), 'periodic firing on advance; leave blank for reactive-only')));
        wrap.appendChild(h('div', { class: 'field' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = t.once !== false; c.addEventListener('change', () => { t.once = c.checked; touched(); }); return h('label', { class: 'checkbox' }, c, 'fire at most once'); })()));
        wrap.appendChild(h('div', { class: 'pt-section-label' }, 'When (reactive condition)'));
        wrap.appendChild(optionalGuard(t, 'when', 'when'));
        wrap.appendChild(h('div', { class: 'pt-section-label' }, 'Effects'));
        t.effects = t.effects || [];
        wrap.appendChild(effectsEditor(t.effects));
        return wrap;
      },
    }));
  } else {
    d.derived = d.derived || {};
    main.appendChild(h('p', { class: 'section-blurb' }, 'Named aggregate queries over the entity/relationship graph, recomputed on read. Great for social-graph and spatial summaries.'));
    main.appendChild(mapSection(d.derived, {
      noun: 'derived', makeNew: () => ({ over: 'entities', where: {}, reduce: 'count' }),
      renderBody: (dv) => {
        dv.where = dv.where || {}; dv.where.attrs = dv.where.attrs || [];
        const wrap = h('div', {});
        wrap.appendChild(field('intent', textInput(dv, 'intent')));
        wrap.appendChild(h('div', { class: 'row' },
          field('over', selectInput(dv, 'over', S.meta.derivedOver, { allowEmpty: false, onChange: renderMain })),
          field('reduce', (() => { const i = textInput(dv, 'reduce'); i.setAttribute('list', 'reduce-verbs'); return i; })(), 'count | any | list | sum:<attr> | argmax:<attr> …')));
        const typeOpts = dv.over === 'relationships' ? relTypeNames() : entityTypeNames();
        wrap.appendChild(h('div', { class: 'row' }, field('where.type', selectInput(dv.where, 'type', typeOpts))));
        if (dv.over === 'relationships') {
          wrap.appendChild(h('div', { class: 'row' },
            field('where.from', (() => { const i = h('input', { type: 'text', value: valToInput(dv.where.from) }); i.addEventListener('input', () => { dv.where.from = inputToVal(i.value); touched(); }); return i; })(), 'id, $self, or {"$path":"…"}'),
            field('where.to', (() => { const i = h('input', { type: 'text', value: valToInput(dv.where.to) }); i.addEventListener('input', () => { dv.where.to = inputToVal(i.value); touched(); }); return i; })(), 'id, $self, or {"$path":"…"}')));
        }
        wrap.appendChild(h('div', { class: 'pt-section-label' }, 'Attribute predicates'));
        wrap.appendChild(attrPredEditor(dv.where.attrs));
        return wrap;
      },
    }));
  }
}

function attrPredEditor(arr) {
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    arr.forEach((p, i) => {
      wrap.appendChild(h('div', { class: 'kv-row' },
        (() => { const a = h('input', { class: 'key', type: 'text', value: p.attr || '', placeholder: 'attr' }); a.addEventListener('input', () => { p.attr = a.value; touched(); }); return a; })(),
        selectInput(p, 'op', ['eq', 'ne', 'gt', 'gte', 'lt', 'lte', 'in'], { allowEmpty: false }),
        (() => { const v = h('input', { type: 'text', value: valToInput(p.value), placeholder: 'value' }); v.addEventListener('input', () => { p.value = inputToVal(v.value); touched(); }); return v; })(),
        h('button', { class: 'tiny del', onclick: () => { arr.splice(i, 1); redraw(); touched(); } }, '✕')));
    });
    wrap.appendChild(h('button', { class: 'tiny add-btn', onclick: () => { arr.push({ attr: '', op: 'eq' }); redraw(); touched(); } }, '+ add predicate'));
  };
  redraw();
  return wrap;
}

function renderLore(main) {
  const d = S.def; d.lore = d.lore || {};
  main.appendChild(h('p', { class: 'section-blurb' }, 'The world bible: authored, static reference entries. Reveal them at runtime with the discover effect.'));
  main.appendChild(mapSection(d.lore, {
    noun: 'lore', makeNew: () => ({ title: '', text: '' }),
    renderBody: (e) => {
      e.tags = e.tags || [];
      const wrap = h('div', {});
      wrap.appendChild(field('title', textInput(e, 'title')));
      wrap.appendChild(field('text', textArea(e, 'text', 4)));
      wrap.appendChild(h('div', { class: 'row' },
        field('subject', textInput(e, 'subject'), 'optional id this is about'),
        field('when', textInput(e, 'when'), 'optional timeline marker')));
      wrap.appendChild(field('intent', textInput(e, 'intent')));
      wrap.appendChild(h('label', { style: 'font-size:12px;color:var(--muted)' }, 'tags'));
      wrap.appendChild(stringList(e.tags, { ph: 'tag' }));
      return wrap;
    },
  }));
}

// ============================ MAP & SCENES ============================
// The map is location entities connected by exit relationships; movers carry a
// `ref` attribute pointing at their place. Node positions and scene authoring
// live under the definition's `_editor` key, which the engine ignores.
const SVGNS = 'http://www.w3.org/2000/svg';
function s(tag, attrs, ...kids) {
  const e = document.createElementNS(SVGNS, tag);
  if (attrs) for (const [k, v] of Object.entries(attrs)) {
    if (v == null || v === false) continue;
    if (k.startsWith('on') && typeof v === 'function') e.addEventListener(k.slice(2), v);
    else e.setAttribute(k, v);
  }
  for (const kid of kids.flat()) { if (kid == null) continue; e.appendChild(typeof kid === 'object' ? kid : document.createTextNode(String(kid))); }
  return e;
}

function ed() { S.def._editor = S.def._editor || {}; return S.def._editor; }

function mapCfg() {
  const e = ed(); e.map = e.map || {};
  const m = e.map;
  const det = detectMapConfig();
  if (!m.placeType) m.placeType = det.placeType;
  if (!m.exitType) m.exitType = det.exitType;
  if (!m.moverAttr) m.moverAttr = det.moverAttr || 'location';
  m.rooms = m.rooms || {};
  m.tokens = m.tokens || {};
  if (!m.cell) m.cell = 28;
  return m;
}

// refTargets: entity types that something points at via a `ref` attribute — i.e.
// types used as a *location* (what makes a place a place, not a self-rel like
// romance/trust between characters).
function refTargets(d) {
  const set = new Set();
  for (const et of Object.values(d.entityTypes || {}))
    for (const spec of Object.values(et.attributes || {}))
      if (spec && spec.type === 'ref' && spec.refType) set.add(spec.refType);
  return set;
}
// selfRelMap: endpoint entity type -> [relationship type names] for relationship
// types that connect a type to itself (candidate "exit" graphs).
function selfRelMap(d) {
  const m = {};
  for (const [name, rt] of Object.entries(d.relationshipTypes || {}))
    if (rt.from && rt.from === rt.to) (m[rt.from] = m[rt.from] || []).push(name);
  return m;
}
// detectPlaceType: a place is an entity type that is BOTH referenced as a location
// AND connected to itself by some relationship (the exits). This excludes
// character↔character relationships (romance, trust) from being read as a map.
function detectPlaceType(d) {
  const refs = refTargets(d), self = selfRelMap(d);
  const candidates = Object.keys(self).filter(t => refs.has(t));
  if (!candidates.length) return '';
  return candidates.includes('location') ? 'location' : candidates[0];
}

function detectMapConfig() {
  const placeType = detectPlaceType(S.def);
  let exitType = '';
  if (placeType) {
    const rels = selfRelMap(S.def)[placeType] || [];
    exitType = rels.includes('exit') ? 'exit' : (rels[0] || '');
  }
  let moverAttr = '';
  for (const et of Object.values(S.def.entityTypes || {})) {
    for (const [an, spec] of Object.entries(et.attributes || {})) {
      if (spec && spec.type === 'ref' && spec.refType === placeType) { moverAttr = an; break; }
    }
    if (moverAttr) break;
  }
  return { placeType, exitType, moverAttr: moverAttr || 'location' };
}

function placesCount(d) {
  let pt = d._editor && d._editor.map && d._editor.map.placeType;
  if (!pt) pt = detectPlaceType(d);
  if (!pt) return 0;
  return Object.values(d.entities || {}).filter(x => x.type === pt).length;
}

function getPlaces(m) { return Object.keys(S.def.entities || {}).filter(id => S.def.entities[id].type === m.placeType).map(id => ({ id, e: S.def.entities[id] })); }
function getExits(m) { return (S.def.relationships || []).filter(r => r.type === m.exitType); }
function moverTypes(m) { return Object.keys(S.def.entityTypes || {}).filter(t => { const a = (S.def.entityTypes[t].attributes || {})[m.moverAttr]; return a && a.type === 'ref'; }); }
function movers(m) { const mt = new Set(moverTypes(m)); return Object.keys(S.def.entities || {}).filter(id => mt.has(S.def.entities[id].type)); }
function occupantsOf(place, m) { return movers(m).filter(id => (S.def.entities[id].attrs || {})[m.moverAttr] === place); }
function placeName(p) { return (p.e && p.e.attrs && p.e.attrs.name) || p.id; }
function nameOf(id) { const e = S.def.entities[id]; return (e && e.attrs && e.attrs.name) || id; }

function slugify(str) {
  return (str || '').toString().toLowerCase().trim().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '') || 'place';
}
function uniqueId(base, taken) { let id = base, i = 1; while (taken(id)) id = base + '_' + (++i); return id; }

// renameEntity changes an entity's id (the JSON key) everywhere it is referenced:
// ref attributes, relationship endpoints, map positions, and scene placements.
function renameEntity(oldId, newId) {
  const d = S.def;
  if (oldId === newId || !d.entities[oldId] || d.entities[newId]) return;
  d.entities[newId] = d.entities[oldId];
  delete d.entities[oldId];
  for (const e of Object.values(d.entities)) {
    if (!e.attrs) continue;
    for (const [k, v] of Object.entries(e.attrs)) if (v === oldId) e.attrs[k] = newId;
  }
  for (const r of (d.relationships || [])) { if (r.from === oldId) r.from = newId; if (r.to === oldId) r.to = newId; }
  const e2 = d._editor || {};
  const mm = e2.map || {};
  for (const key of ['rooms', 'tokens']) {
    if (mm[key] && mm[key][oldId] !== undefined) { mm[key][newId] = mm[key][oldId]; delete mm[key][oldId]; }
  }
  for (const sc of Object.values(e2.scenes || {})) {
    for (const coll of ['placements', 'tokens']) {
      const c = sc[coll]; if (!c) continue;
      if (coll === 'placements') for (const [ent, place] of Object.entries(c)) if (place === oldId) c[ent] = newId;
      if (c[oldId] !== undefined) { c[newId] = c[oldId]; delete c[oldId]; }
    }
  }
  if (S.mapSel === oldId) S.mapSel = newId;
}

function renderMap(main) {
  // Detect read-only first so a game with no map doesn't get an empty _editor.map.
  const stored = (S.def._editor && S.def._editor.map) || null;
  const det = detectMapConfig();
  const placeType = (stored && stored.placeType) || det.placeType;
  const exitType = (stored && stored.exitType) || det.exitType;
  const haveType = placeType && (placeType in (S.def.entityTypes || {}));
  const haveExit = exitType && (exitType in (S.def.relationshipTypes || {}));
  if (!haveType || !haveExit) {
    main.appendChild(h('p', { class: 'section-blurb' },
      'A map is location entities connected by exit relationships; characters and objects are placed via a location reference. This game has no map structure yet.'));
    main.appendChild(h('button', { class: 'primary', onclick: scaffoldMap }, '+ Create map structure (location + exit)'));
    return;
  }
  const m = mapCfg(); // persists config only now that a real map exists
  ensureRooms(m);
  const scenes = (ed().scenes = ed().scenes || {});
  if (S.mapTab !== 'layout' && !(S.mapTab in scenes)) S.mapTab = 'layout';

  // tab bar: shared Layout + one tab per scene (objects restaged) + add
  const bar = h('div', { class: 'tabs' });
  bar.appendChild(h('div', { class: 'tab' + (S.mapTab === 'layout' ? ' active' : ''), onclick: () => { S.mapTab = 'layout'; renderMain(); } }, 'Layout'));
  for (const id of Object.keys(scenes)) bar.appendChild(h('div', { class: 'tab' + (S.mapTab === id ? ' active' : ''), onclick: () => { S.mapTab = id; renderMain(); } }, '▶ ' + (scenes[id].name || id)));
  bar.appendChild(h('div', { class: 'tab', style: 'color:var(--accent)', onclick: () => addScene(m) }, '+ scene'));
  main.appendChild(bar);

  if (S.mapTab === 'layout') renderGrid(main, m, { mode: 'layout' });
  else renderGrid(main, m, { mode: 'scene', sc: scenes[S.mapTab] });
}

function scaffoldMap() {
  S.def.entityTypes = S.def.entityTypes || {};
  S.def.relationshipTypes = S.def.relationshipTypes || {};
  if (!S.def.entityTypes.location) S.def.entityTypes.location = { description: 'A place in the world', attributes: { name: { type: 'string' } } };
  if (!S.def.relationshipTypes.exit) S.def.relationshipTypes.exit = { description: 'A passage between places', from: 'location', to: 'location', directed: true, attributes: { direction: { type: 'string' }, locked: { type: 'bool' } } };
  ed().map = { placeType: 'location', exitType: 'exit', moverAttr: 'location', rooms: {}, tokens: {}, cell: 28 };
  S.mapScroll = null; // re-center the view on the fresh (origin-centered) map
  touched(); refresh();
}

function mapSettings(m) {
  const selfRel = Object.keys(S.def.relationshipTypes || {}).filter(n => { const rt = S.def.relationshipTypes[n]; return rt.from && rt.from === rt.to; });
  return h('details', { class: 'subcard' },
    h('summary', { style: 'cursor:pointer;color:var(--muted)' }, 'map settings'),
    h('div', { class: 'row', style: 'margin-top:8px' },
      field('place type', selectInput(m, 'placeType', Object.keys(S.def.entityTypes || {}), { allowEmpty: false, onChange: () => { touched(); renderMain(); } })),
      field('exit relationship', selectInput(m, 'exitType', selfRel.length ? selfRel : Object.keys(S.def.relationshipTypes || {}), { allowEmpty: false, onChange: () => { touched(); renderMain(); } })),
      field('mover location attr', textInput(m, 'moverAttr', { onChange: () => { touched(); renderMain(); } }), 'the ref attr that points a character/object at a place')));
}

// ensureRooms gives every place a grid rectangle {x,y,w,h} (in cells) if it lacks
// one, packed into a block CENTERED on the origin (0,0) so the map can grow in any
// direction. Coordinates may be negative. Rooms are shared across all scenes.
function ensureRooms(m) {
  const places = getPlaces(m);
  if (!places.some(p => !m.rooms[p.id])) return;
  const RW = 4, RH = 3, PITCHX = 6, PITCHY = 5;
  const n = places.length;
  const gcols = Math.max(1, Math.ceil(Math.sqrt(n)));
  const grows = Math.ceil(n / gcols);
  const ox = -Math.round((gcols * PITCHX) / 2), oy = -Math.round((grows * PITCHY) / 2);
  let i = 0;
  for (const p of places) {
    if (!m.rooms[p.id]) m.rooms[p.id] = { x: ox + (i % gcols) * PITCHX, y: oy + Math.floor(i / gcols) * PITCHY, w: RW, h: RH };
    i++;
  }
}

function clampZoom(z) { return Math.max(0.35, Math.min(2.6, Math.round(z * 100) / 100)); }
function zoomControls() {
  const setZ = (z) => { S.mapZoom = clampZoom(z); renderMain(); };
  return h('span', { class: 'inline', style: 'gap:4px' },
    h('button', { class: 'tiny', onclick: () => setZ((S.mapZoom || 1) - 0.2) }, '−'),
    h('span', { class: 'hint', style: 'min-width:40px;text-align:center' }, Math.round((S.mapZoom || 1) * 100) + '%'),
    h('button', { class: 'tiny', onclick: () => setZ((S.mapZoom || 1) + 0.2) }, '+'),
    h('span', { class: 'hint' }, 'scroll to zoom · drag empty space to pan'));
}

function renderGrid(main, m, opts) {
  const { mode, sc } = opts;
  if (mode === 'layout') main.appendChild(mapSettings(m));
  const tb = h('div', { class: 'inline', style: 'margin:10px 0;gap:12px' });
  if (mode === 'layout') tb.appendChild(h('button', { class: 'primary tiny', onclick: () => addPlace(m) }, '+ add room'));
  tb.appendChild(zoomControls());
  tb.appendChild(h('span', { class: 'hint' }, mode === 'layout'
    ? 'drag rooms to arrange · drag the corner to resize · drag a token to set its starting room & spot · click a room to edit it'
    : 'drag people & props to restage them for this scene — rooms are shared across all scenes'));
  main.appendChild(tb);

  const wrap = h('div', { style: 'display:flex;gap:14px;align-items:stretch;flex:1;min-height:0' });
  wrap.appendChild(buildCanvas(m, mode, sc));
  const panel = h('div', { style: 'flex:0 0 300px;overflow:auto' });
  panel.appendChild(mode === 'layout' ? roomInspector(m) : sceneInspector(m, sc));
  wrap.appendChild(panel);
  main.appendChild(wrap);
}

function tokenColor(type) {
  const pal = ['#7c9cff', '#4caf7d', '#d8a657', '#e2615a', '#b07cff', '#48b4c0', '#d98cae'];
  let hns = 0; for (const c of (type || '')) hns = (hns * 31 + c.charCodeAt(0)) >>> 0;
  return pal[hns % pal.length];
}
function shortLabel(s) {
  s = s || '';
  if (s.length <= 8) return s;
  const parts = s.split(/\s+/);
  if (parts.length > 1) return parts.map(p => p[0]).join('').slice(0, 4).toUpperCase();
  return s.slice(0, 7);
}
function roomAt(m, cx, cy) {
  for (const p of getPlaces(m)) { const r = m.rooms[p.id]; if (r && cx >= r.x && cx < r.x + r.w && cy >= r.y && cy < r.y + r.h) return p.id; }
  return null;
}

// tokensForView resolves which room each character/prop is in for this view and
// their (room-relative) cell coordinates, defaulting any that aren't placed yet.
function tokensForView(m, mode, sc) {
  const ids = new Set(movers(m));
  if (mode === 'scene' && sc && sc.placements) for (const k of Object.keys(sc.placements)) ids.add(k);
  const out = [];
  for (const ent of ids) {
    const e = S.def.entities[ent]; if (!e) continue;
    const room = (mode === 'scene' && sc && sc.placements && sc.placements[ent]) || (e.attrs || {})[m.moverAttr];
    if (!room || !m.rooms[room]) continue;
    const coord = (mode === 'scene' && sc && sc.tokens && sc.tokens[ent]) || m.tokens[ent];
    out.push({ ent, room, coord });
  }
  const counts = {};
  for (const t of out) {
    if (t.coord && typeof t.coord.x === 'number') { t.x = t.coord.x; t.y = t.coord.y; }
    else { const r = m.rooms[t.room], w = Math.max(1, r.w), n = counts[t.room] || 0; t.x = n % w; t.y = 1 + Math.floor(n / w); counts[t.room] = n + 1; }
  }
  return out;
}

// setToken records a token's room (graph relationship) and its room-relative
// coordinates. In a scene this writes the per-scene override; on Layout it sets
// the entity's starting location and base coordinate.
function setToken(ent, room, x, y, m, mode, sc) {
  ensureMoverAttr(ent, m);
  if (mode === 'scene') {
    sc.placements = sc.placements || {}; sc.placements[ent] = room;
    sc.tokens = sc.tokens || {}; sc.tokens[ent] = { x, y };
    syncScenes(m);
  } else {
    const e = S.def.entities[ent]; e.attrs = e.attrs || {}; e.attrs[m.moverAttr] = room;
    m.tokens[ent] = { x, y };
  }
  touched();
}

function buildCanvas(m, mode, sc) {
  const cell = (m.cell || 28) * (S.mapZoom || 1);
  const places = getPlaces(m);
  // Origin-centered grid: (0,0) is the centre cell; coordinates may be negative,
  // so the map can grow in every direction. We render a square [-E,+E] window
  // that grows to keep margin around the furthest room.
  let reach = 0, minX = 0, minY = 0, maxX = 0, maxY = 0, any = false;
  for (const p of places) {
    const r = m.rooms[p.id]; if (!r) continue; any = true;
    minX = Math.min(minX, r.x); minY = Math.min(minY, r.y);
    maxX = Math.max(maxX, r.x + r.w); maxY = Math.max(maxY, r.y + r.h);
    reach = Math.max(reach, Math.abs(r.x), Math.abs(r.x + r.w), Math.abs(r.y), Math.abs(r.y + r.h));
  }
  const E = Math.max(70, Math.ceil(reach) + 30); // half-extent in cells
  const span = 2 * E, W = span * cell, H = span * cell;
  const PX = (g) => (g + E) * cell;              // grid cell -> pixel

  const box = h('div', { style: 'flex:1;min-width:0;align-self:stretch;overflow:auto;background:var(--bg);border:1px solid var(--border);border-radius:8px;cursor:grab' });
  const svg = s('svg', { width: W, height: H, style: 'display:block' });
  svg.appendChild(s('defs', {}, s('marker', { id: 'exitarrow', viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 7, markerHeight: 7, orient: 'auto-start-reverse' }, s('path', { d: 'M0,0 L10,5 L0,10 z', fill: '#5a6180' }))));

  // grid, with emphasised centre axes and a marked (0,0) cell
  const grid = s('g', {});
  for (let i = 0; i <= span; i++) {
    const onAxis = i === E, c = onAxis ? '#33405e' : '#1d212b', sw = onAxis ? 2 : 1;
    grid.appendChild(s('line', { x1: i * cell, y1: 0, x2: i * cell, y2: H, stroke: c, 'stroke-width': sw }));
    grid.appendChild(s('line', { x1: 0, y1: i * cell, x2: W, y2: i * cell, stroke: c, 'stroke-width': sw }));
  }
  grid.appendChild(s('rect', { x: PX(0), y: PX(0), width: cell, height: cell, fill: 'rgba(124,156,255,0.10)' }));
  grid.appendChild(s('text', { x: PX(0) + 3, y: PX(0) + 11, fill: '#5a6180', 'font-size': 9, style: 'pointer-events:none' }, '0,0'));
  svg.appendChild(grid);

  // transparent background: drag to pan, click to deselect
  const bg = s('rect', { x: 0, y: 0, width: W, height: H, fill: 'transparent' });
  bg.addEventListener('mousedown', (ev) => {
    ev.preventDefault();
    const sx = ev.clientX, sy = ev.clientY, sl = box.scrollLeft, st = box.scrollTop; let moved = false;
    const mm = (e2) => { moved = true; box.scrollLeft = sl - (e2.clientX - sx); box.scrollTop = st - (e2.clientY - sy); };
    const mu = () => { document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', mu); if (!moved && S.mapSel) { S.mapSel = null; renderMain(); } };
    document.addEventListener('mousemove', mm); document.addEventListener('mouseup', mu);
  });
  svg.appendChild(bg);

  // scroll wheel zooms toward the cursor (raw pixel-cell space)
  box.addEventListener('wheel', (e) => {
    e.preventDefault();
    const rect = box.getBoundingClientRect();
    const offX = e.clientX - rect.left, offY = e.clientY - rect.top;
    const cellX = (offX + box.scrollLeft) / cell, cellY = (offY + box.scrollTop) / cell;
    const nz = clampZoom((S.mapZoom || 1) * (e.deltaY < 0 ? 1.12 : 1 / 1.12));
    if (nz === (S.mapZoom || 1)) return;
    S.mapZoom = nz; S.mapAnchor = { cellX, cellY, offX, offY };
    renderMain();
  }, { passive: false });

  // exits: each direction offset to its own side, label near its source, so a
  // bidirectional pair never overlaps.
  const edgeG = s('g', {}); svg.appendChild(edgeG);
  const drawEdges = () => {
    edgeG.innerHTML = '';
    for (const r of getExits(m)) {
      const a = m.rooms[r.from], b = m.rooms[r.to]; if (!a || !b) continue;
      const ax = PX(a.x + a.w / 2), ay = PX(a.y + a.h / 2), bx = PX(b.x + b.w / 2), by = PX(b.y + b.h / 2);
      const dx = bx - ax, dy = by - ay, len = Math.hypot(dx, dy) || 1, nx = -dy / len, ny = dx / len;
      const off = (r.from < r.to ? 1 : -1) * 5;
      const x1 = ax + nx * off, y1 = ay + ny * off, x2 = bx + nx * off, y2 = by + ny * off;
      const locked = r.attrs && r.attrs.locked;
      edgeG.appendChild(s('line', { x1, y1, x2, y2, stroke: locked ? '#e2615a' : '#5a6180', 'stroke-width': 2, 'stroke-dasharray': locked ? '6 4' : '', 'marker-end': 'url(#exitarrow)' }));
      const dir = (r.attrs && r.attrs.direction) || '';
      if (dir) { const t = 0.3, lx = x1 + (x2 - x1) * t, ly = y1 + (y2 - y1) * t; edgeG.appendChild(s('text', { x: lx + nx * 8, y: ly + ny * 8 + 3, 'text-anchor': 'middle', fill: '#9aa3b2', 'font-size': 10 }, dir)); }
    }
  };
  drawEdges();

  const tokens = tokensForView(m, mode, sc);
  const byRoom = {};
  for (const t of tokens) (byRoom[t.room] = byRoom[t.room] || []).push(t);

  for (const p of places) {
    const r = m.rooms[p.id]; if (!r) continue;
    const seld = mode === 'layout' && S.mapSel === p.id;
    const g = s('g', { transform: `translate(${PX(r.x)},${PX(r.y)})` });
    const rect = s('rect', { x: 0, y: 0, width: r.w * cell, height: r.h * cell, rx: 6, fill: seld ? '#222b44' : '#171a22', stroke: seld ? '#7c9cff' : '#2e3342', 'stroke-width': seld ? 2 : 1, style: mode === 'layout' ? 'cursor:move' : 'cursor:pointer' });
    g.appendChild(rect);
    g.appendChild(s('text', { x: 7, y: 16, fill: '#e6e8ee', 'font-size': 12, 'font-weight': 600, style: 'pointer-events:none' }, placeName(p)));
    for (const t of (byRoom[p.id] || [])) g.appendChild(drawToken(t, m, mode, sc, cell));
    if (mode === 'layout') {
      const hd = s('rect', { x: r.w * cell - 11, y: r.h * cell - 11, width: 11, height: 11, fill: '#7c9cff', style: 'cursor:nwse-resize' });
      hd.addEventListener('mousedown', (ev) => {
        ev.stopPropagation(); ev.preventDefault();
        const sx = ev.clientX, sy = ev.clientY, ow = r.w, oh = r.h;
        const mm = (e2) => {
          r.w = Math.max(2, ow + Math.round((e2.clientX - sx) / cell)); r.h = Math.max(2, oh + Math.round((e2.clientY - sy) / cell));
          rect.setAttribute('width', r.w * cell); rect.setAttribute('height', r.h * cell);
          hd.setAttribute('x', r.w * cell - 11); hd.setAttribute('y', r.h * cell - 11); drawEdges();
        };
        const mu = () => { document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', mu); touched(); renderMain(); };
        document.addEventListener('mousemove', mm); document.addEventListener('mouseup', mu);
      });
      g.appendChild(hd);
      rect.addEventListener('mousedown', (ev) => {
        ev.stopPropagation(); ev.preventDefault();
        const sx = ev.clientX, sy = ev.clientY, ox = r.x, oy = r.y; let moved = false;
        const mm = (e2) => {
          if (Math.abs(e2.clientX - sx) + Math.abs(e2.clientY - sy) > 3) moved = true;
          r.x = ox + Math.round((e2.clientX - sx) / cell); r.y = oy + Math.round((e2.clientY - sy) / cell); // negatives allowed
          g.setAttribute('transform', `translate(${PX(r.x)},${PX(r.y)})`); drawEdges();
        };
        const mu = () => { document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', mu); if (moved) { touched(); renderMain(); } else { S.mapSel = p.id; renderMain(); } };
        document.addEventListener('mousemove', mm); document.addEventListener('mouseup', mu);
      });
    }
    svg.appendChild(g);
  }
  box.appendChild(svg);

  box.addEventListener('scroll', () => { S.mapScroll = { left: box.scrollLeft, top: box.scrollTop }; });
  requestAnimationFrame(() => {
    if (S.mapAnchor) { box.scrollLeft = S.mapAnchor.cellX * cell - S.mapAnchor.offX; box.scrollTop = S.mapAnchor.cellY * cell - S.mapAnchor.offY; S.mapAnchor = null; }
    else if (S.mapScroll) { box.scrollLeft = S.mapScroll.left; box.scrollTop = S.mapScroll.top; }
    else { const cgx = any ? (minX + maxX) / 2 : 0, cgy = any ? (minY + maxY) / 2 : 0; box.scrollLeft = PX(cgx) - box.clientWidth / 2; box.scrollTop = PX(cgy) - box.clientHeight / 2; }
    S.mapScroll = { left: box.scrollLeft, top: box.scrollTop };
  });
  return box;
}

function drawToken(t, m, mode, sc, cell) {
  const e = S.def.entities[t.ent];
  const g = s('g', { transform: `translate(${t.x * cell},${t.y * cell})`, style: 'cursor:grab' });
  const pad = cell * 0.06, w = cell * 0.88, hgt = cell * 0.66;
  g.appendChild(s('rect', { x: pad, y: cell * 0.16, width: w, height: hgt, rx: 4, fill: tokenColor(e.type), stroke: '#0d0f14', 'stroke-width': 1 }));
  g.appendChild(s('text', { x: pad + w / 2, y: cell * 0.16 + hgt / 2 + 3, 'text-anchor': 'middle', fill: '#0d0f14', 'font-size': Math.max(8, cell * 0.26), 'font-weight': 700, style: 'pointer-events:none' }, shortLabel(nameOf(t.ent))));
  g.appendChild(s('title', {}, `${nameOf(t.ent)} (${e.type})`));
  g.addEventListener('mousedown', (ev) => {
    ev.stopPropagation(); ev.preventDefault();
    const sx = ev.clientX, sy = ev.clientY, bx = t.x, by = t.y; let cx = bx, cy = by, moved = false;
    const mm = (e2) => {
      cx = bx + (e2.clientX - sx) / cell; cy = by + (e2.clientY - sy) / cell;
      if (Math.abs(e2.clientX - sx) + Math.abs(e2.clientY - sy) > 3) moved = true;
      g.setAttribute('transform', `translate(${cx * cell},${cy * cell})`);
    };
    const mu = () => {
      document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', mu);
      if (!moved) return;
      const room0 = m.rooms[t.room]; const absX = room0.x + cx, absY = room0.y + cy;
      const target = roomAt(m, absX + 0.4, absY + 0.3) || t.room; const tr = m.rooms[target];
      let rx = Math.round(absX - tr.x), ry = Math.round(absY - tr.y);
      rx = Math.max(0, Math.min(tr.w - 1, rx)); ry = Math.max(0, Math.min(tr.h - 1, ry));
      setToken(t.ent, target, rx, ry, m, mode, sc); renderMain();
    };
    document.addEventListener('mousemove', mm); document.addEventListener('mouseup', mu);
  });
  return g;
}

function addPlace(m) {
  const et = S.def.entityTypes[m.placeType];
  const hasName = et && et.attributes && et.attributes.name;
  S.def.entities = S.def.entities || {};
  const id = uniqueId(slugify('New place'), x => x in S.def.entities);
  S.def.entities[id] = { type: m.placeType, attrs: hasName ? { name: 'New place' } : {} };
  // place near the origin, just below the existing cluster (coords may be negative)
  const rs = Object.values(m.rooms || {});
  let nx = -2, ny = -1;
  if (rs.length) {
    const lo = Math.min(...rs.map(r => r.x)), hi = Math.max(...rs.map(r => r.x + r.w)), bot = Math.max(...rs.map(r => r.y + r.h));
    nx = Math.round((lo + hi) / 2 - 2); ny = bot + 2;
  }
  m.rooms[id] = { x: nx, y: ny, w: 4, h: 3 };
  S.mapSel = id;
  touched(); refresh();
}

function roomInspector(m) {
  const places = getPlaces(m);
  const exits = getExits(m);
  const box = h('div', {});
  const sel = S.mapSel;
  if (!sel || !S.def.entities[sel] || S.def.entities[sel].type !== m.placeType) {
    box.appendChild(h('div', { class: 'pt-section-label' }, 'Places'));
    if (!places.length) box.appendChild(h('div', { class: 'empty' }, 'none'));
    for (const p of places) box.appendChild(h('button', { class: 'action-btn', onclick: () => { S.mapSel = p.id; renderMain(); } }, `${placeName(p)} — ${occupantsOf(p.id, m).length} here`));
    const un = movers(m).filter(id => { const loc = (S.def.entities[id].attrs || {})[m.moverAttr]; return !loc || !S.def.entities[loc]; });
    if (un.length) { box.appendChild(h('div', { class: 'pt-section-label' }, 'Unplaced')); box.appendChild(h('div', { class: 'hint' }, un.join(', '))); }
    return box;
  }
  const e = S.def.entities[sel];
  e.attrs = e.attrs || {};
  box.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, placeName({ id: sel, e })),
    h('span', { class: 'hint', style: 'margin-left:6px' }, sel),
    h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => deletePlace(sel, m) }, '✕ delete place')));
  const hasNameAttr = S.def.entityTypes[m.placeType].attributes && S.def.entityTypes[m.placeType].attributes.name;
  if (hasNameAttr) {
    const nameInp = h('input', { type: 'text', value: e.attrs.name || '' });
    nameInp.addEventListener('input', () => { e.attrs.name = nameInp.value || undefined; touched(); });
    nameInp.addEventListener('change', () => {
      const newId = uniqueId(slugify(e.attrs.name), x => x !== sel && x in S.def.entities);
      if (newId !== sel) { renameEntity(sel, newId); syncScenes(m); touched(); renderMain(); }
    });
    box.appendChild(field('name', nameInp, 'the JSON key follows this name (slugified) — e.g. “The Foyer” → the_foyer'));
  }
  box.appendChild(field('description', textArea(e, 'description', 3)));

  // occupants
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Here (characters & objects)'));
  const occ = occupantsOf(sel, m);
  if (!occ.length) box.appendChild(h('div', { class: 'empty' }, 'nobody here yet'));
  for (const id of occ) box.appendChild(h('div', { class: 'kv-row' }, h('span', { style: 'flex:1' }, `${id} (${S.def.entities[id].type})`),
    h('button', { class: 'tiny del', onclick: () => { delete S.def.entities[id].attrs[m.moverAttr]; touched(); renderMain(); } }, 'remove')));
  const candidates = movers(m).filter(id => id !== sel && (S.def.entities[id].attrs || {})[m.moverAttr] !== sel)
    .concat(Object.keys(S.def.entities).filter(id => id !== sel && S.def.entities[id].type !== m.placeType && !moverTypes(m).includes(S.def.entities[id].type)));
  const uniq = [...new Set(candidates)];
  if (uniq.length) {
    const selBox = selectInput({ v: '' }, 'v', uniq.map(id => ({ value: id, label: nameOf(id) })), { emptyLabel: '+ place a character or prop here…' });
    selBox.addEventListener('change', () => { if (selBox.value) { placeEntity(selBox.value, sel, m); } });
    box.appendChild(h('div', { class: 'field', style: 'margin-top:6px' }, selBox));
  }

  // items sitting in this place (the place entity's starting inventory)
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Items in this place (starting)'));
  e.inventory = e.inventory || {};
  if (!itemTypeNames().length) box.appendChild(h('div', { class: 'hint' }, 'define item types (Types → Item types) to stock this place'));
  else box.appendChild(invEditor(e.inventory));

  // exits
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Exits from here'));
  const fromHere = exits.filter(r => r.from === sel);
  if (!fromHere.length) box.appendChild(h('div', { class: 'empty' }, 'no exits'));
  for (const r of fromHere) {
    r.attrs = r.attrs || {};
    const sub = h('div', { class: 'subcard' });
    sub.appendChild(h('div', { class: 'kv-row' }, h('span', { style: 'flex:1' }, `→ ${nameOf(r.to)}`),
      h('button', { class: 'tiny', onclick: () => { S.mapSel = r.to; renderMain(); } }, 'go'),
      h('button', { class: 'tiny del', onclick: () => { const i = S.def.relationships.indexOf(r); if (i >= 0) S.def.relationships.splice(i, 1); touched(); renderMain(); } }, '✕')));
    const rtAttrs = (S.def.relationshipTypes[m.exitType].attributes) || {};
    if ('direction' in rtAttrs) sub.appendChild(field('direction', textInput(r.attrs, 'direction', { ph: 'e.g. north' })));
    if ('locked' in rtAttrs) sub.appendChild(h('div', { class: 'field' }, checkbox(r.attrs, 'locked', 'locked')));
    box.appendChild(sub);
  }
  const others = places.filter(p => p.id !== sel && !fromHere.some(r => r.to === p.id));
  if (others.length) {
    const selBox = selectInput({ v: '' }, 'v', others.map(p => ({ value: p.id, label: placeName(p) })), { emptyLabel: '+ add exit to…' });
    selBox.addEventListener('change', () => { if (selBox.value) addExit(sel, selBox.value, m); });
    box.appendChild(h('div', { class: 'field', style: 'margin-top:6px' }, selBox));
  }
  return box;
}

// ensureMoverAttr makes sure an entity's type can hold a location reference, so a
// character or prop that wasn't authored with one can still be positioned.
function ensureMoverAttr(id, m) {
  const et = S.def.entityTypes[S.def.entities[id].type];
  et.attributes = et.attributes || {};
  if (!et.attributes[m.moverAttr] || et.attributes[m.moverAttr].type !== 'ref')
    et.attributes[m.moverAttr] = { type: 'ref', refType: m.placeType };
}

function placeEntity(id, place, m) {
  ensureMoverAttr(id, m);
  const e = S.def.entities[id];
  e.attrs = e.attrs || {};
  e.attrs[m.moverAttr] = place;
  touched(); renderMain();
}

// invEditor edits an entity's starting inventory (item type -> count), used to
// stock a place with props/items.
function invEditor(inv) {
  const items = itemTypeNames();
  const wrap = h('div', {});
  const redraw = () => {
    wrap.innerHTML = '';
    for (const it of Object.keys(inv)) {
      const n = h('input', { type: 'number', value: inv[it], style: 'max-width:90px' });
      n.addEventListener('input', () => { inv[it] = Number(n.value || 0); touched(); });
      wrap.appendChild(h('div', { class: 'kv-row' }, h('span', { class: 'key', style: 'align-self:center' }, it), n,
        h('button', { class: 'tiny del', onclick: () => { delete inv[it]; redraw(); touched(); } }, '✕')));
    }
    const free = items.filter(x => !(x in inv));
    if (free.length) {
      const sb = selectInput({ v: '' }, 'v', free, { emptyLabel: '+ add item…' });
      sb.addEventListener('change', () => { if (sb.value) { inv[sb.value] = 1; redraw(); touched(); } });
      wrap.appendChild(h('div', { class: 'field', style: 'margin-top:6px' }, sb));
    }
  };
  redraw();
  return wrap;
}

function addExit(from, to, m) {
  S.def.relationships = S.def.relationships || [];
  S.def.relationships.push({ type: m.exitType, from, to, attrs: {} });
  touched(); renderMain();
}

function deletePlace(id, m) {
  if (!confirm(`Delete place "${id}"? Exits to/from it and anyone standing there will be cleared.`)) return;
  delete S.def.entities[id];
  S.def.relationships = (S.def.relationships || []).filter(r => !(r.type === m.exitType && (r.from === id || r.to === id)));
  for (const eid of Object.keys(S.def.entities)) { const a = S.def.entities[eid].attrs || {}; if (a[m.moverAttr] === id) delete a[m.moverAttr]; }
  if (m.rooms) delete m.rooms[id];
  if (m.tokens) delete m.tokens[id];
  for (const sc of Object.values(ed().scenes || {})) {
    if (sc.placements) for (const [ent, place] of Object.entries(sc.placements)) if (place === id) delete sc.placements[ent];
  }
  syncScenes(m);
  if (S.mapSel === id) S.mapSel = null;
  touched(); refresh();
}

// ---- Scenes ----
function addScene(m) {
  const scenes = (ed().scenes = ed().scenes || {});
  let i = 1, nk = 'scene'; while (nk in scenes) nk = 'scene' + (++i);
  scenes[nk] = { name: 'New scene', placements: {}, tokens: {}, once: true };
  syncScenes(m); touched(); S.mapTab = nk; renderMain();
}

// sceneInspector is the side panel for a scene tab: when it fires, plus the
// non-spatial staging (items, lore, beat, journal). Token positions are set by
// dragging on the shared map; this panel covers everything else.
function sceneInspector(m, sc) {
  const scenes = ed().scenes; const id = S.mapTab;
  sc.placements = sc.placements || {};
  const box = h('div', {});
  box.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Scene'),
    h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => { delete scenes[id]; syncScenes(m); touched(); S.mapTab = 'layout'; renderMain(); } }, '✕ delete scene')));
  const nameInp = h('input', { type: 'text', value: sc.name || '' });
  nameInp.addEventListener('input', () => { sc.name = nameInp.value; syncScenes(m); touched(); });
  nameInp.addEventListener('change', () => renderMain());
  box.appendChild(field('name', nameInp));

  box.appendChild(h('div', { class: 'pt-section-label' }, 'Fires when'));
  box.appendChild(h('div', { class: 'row' },
    field('machine', selectInput(sc, 'machine', machineNames(), { onChange: () => { syncScenes(m); touched(); renderMain(); } })),
    field('enters state', selectInput(sc, 'state', stateNames(sc.machine), { onChange: () => { syncScenes(m); touched(); } }))));
  box.appendChild(h('div', { class: 'field' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = sc.once !== false; c.addEventListener('change', () => { sc.once = c.checked; syncScenes(m); touched(); }); return h('label', { class: 'checkbox' }, c, 'fire once'); })()));
  if (!sc.machine || !sc.state) box.appendChild(h('div', { class: 'hint' }, 'pick a machine + state so the scene fires'));

  // cast & props — positioned by dragging; offer to bring in anyone not shown
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Cast & props'));
  box.appendChild(h('div', { class: 'hint' }, 'drag tokens on the map to restage them for this scene. Everyone starts where the Layout places them.'));
  const placesIds = getPlaces(m).map(p => p.id);
  const shown = new Set(tokensForView(m, 'scene', sc).map(t => t.ent));
  const cand = Object.keys(S.def.entities).filter(eid => S.def.entities[eid].type !== m.placeType && !shown.has(eid));
  if (cand.length && placesIds.length) {
    const selBox = selectInput({ v: '' }, 'v', cand.map(eid => ({ value: eid, label: nameOf(eid) })), { emptyLabel: '+ bring someone/something into frame…' });
    selBox.addEventListener('change', () => { if (selBox.value) { ensureMoverAttr(selBox.value, m); sc.placements[selBox.value] = placesIds[0]; syncScenes(m); touched(); renderMain(); } });
    box.appendChild(h('div', { class: 'field', style: 'margin-top:6px' }, selBox));
  }

  // items: add an item type into an entity's (or place's) inventory
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Place items'));
  sc.items = sc.items || [];
  const itemIds = itemTypeNames();
  const holderOpts = Object.keys(S.def.entities).map(eid => ({ value: eid, label: nameOf(eid) }));
  if (!itemIds.length) box.appendChild(h('div', { class: 'hint' }, 'define item types (Types → Item types) to place items'));
  else {
    sc.items.forEach((it, i) => {
      const n = h('input', { type: 'number', value: it.count || 1, style: 'max-width:64px' });
      n.addEventListener('input', () => { it.count = Number(n.value || 1); syncScenes(m); touched(); });
      box.appendChild(h('div', { class: 'kv-row' },
        selectInput(it, 'item', itemIds, { allowEmpty: false, onChange: () => { syncScenes(m); touched(); } }),
        h('span', { style: 'align-self:center;color:var(--muted)' }, '→'),
        selectInput(it, 'holder', holderOpts, { allowEmpty: false, onChange: () => { syncScenes(m); touched(); } }),
        n,
        h('button', { class: 'tiny del', onclick: () => { sc.items.splice(i, 1); syncScenes(m); touched(); renderMain(); } }, '✕')));
    });
    box.appendChild(h('button', { class: 'tiny add-btn', onclick: () => { sc.items.push({ item: itemIds[0], holder: Object.keys(S.def.entities)[0], count: 1 }); syncScenes(m); touched(); renderMain(); } }, '+ place an item'));
  }

  // lore trigger: reveal lore entries when the scene fires
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Reveal lore (lore trigger)'));
  sc.lore = sc.lore || [];
  const loreIds = keys(S.def.lore);
  if (!loreIds.length) box.appendChild(h('div', { class: 'hint' }, 'no lore yet — add entries in the Lore section'));
  for (const lid of loreIds) {
    const cb = h('input', { type: 'checkbox' }); cb.checked = sc.lore.includes(lid);
    cb.addEventListener('change', () => { if (cb.checked) { if (!sc.lore.includes(lid)) sc.lore.push(lid); } else sc.lore = sc.lore.filter(x => x !== lid); syncScenes(m); touched(); });
    box.appendChild(h('div', {}, h('label', { class: 'checkbox' }, cb, (S.def.lore[lid].title || lid))));
  }

  box.appendChild(h('div', { class: 'pt-section-label' }, 'On entry (optional)'));
  box.appendChild(field('mark beat', selectInput(sc, 'markBeat', keys(S.def.beats), { onChange: () => { syncScenes(m); touched(); } })));
  box.appendChild(field('journal note', textInput(sc, 'record', { onChange: () => { syncScenes(m); touched(); } })));
  return box;
}

function compileScene(id, sc, m) {
  const effects = [];
  for (const [ent, place] of Object.entries(sc.placements || {})) if (ent && place) effects.push({ op: 'set', target: `entity.${ent}.${m.moverAttr}`, value: place });
  for (const it of (sc.items || [])) if (it && it.holder && it.item) effects.push({ op: 'add_item', entity: it.holder, item: it.item, count: it.count || 1 });
  for (const lore of (sc.lore || [])) if (lore) effects.push({ op: 'discover', lore });
  if (sc.markBeat) effects.push({ op: 'mark_beat', beat: sc.markBeat });
  if (sc.record) effects.push({ op: 'record', text: sc.record });
  const when = (sc.machine && sc.state) ? { target: `machine.${sc.machine}.state`, op: 'eq', value: sc.state } : null;
  return { when, once: sc.once !== false, effects, intent: `scene: ${sc.name || id}` };
}

// syncScenes regenerates all scene_* triggers from _editor.scenes (the source of
// truth for the Scenes UI), removing any that no longer apply.
function syncScenes(m) {
  S.def.triggers = S.def.triggers || {};
  for (const k of Object.keys(S.def.triggers)) if (k.startsWith('scene_')) delete S.def.triggers[k];
  for (const [id, sc] of Object.entries(ed().scenes || {})) {
    const t = compileScene(id, sc, m);
    if (t.when && t.effects.length) S.def.triggers['scene_' + id] = t;
  }
  if (!Object.keys(S.def.triggers).length) delete S.def.triggers;
}

// ======================= CHARACTERS =======================
// Lore entries about a subject (character / item / place), edited inline.
function loreAbout(subjectId) {
  S.def.lore = S.def.lore || {};
  const box = h('div', {});
  const entries = Object.keys(S.def.lore).filter(k => S.def.lore[k].subject === subjectId);
  if (!entries.length) box.appendChild(h('div', { class: 'empty' }, 'no lore yet'));
  for (const k of entries) {
    const lo = S.def.lore[k]; lo.tags = lo.tags || [];
    const sub = h('div', { class: 'subcard' });
    sub.appendChild(h('div', { class: 'kv-row' }, renameInput(S.def.lore, k, () => renderMain()),
      h('span', { style: 'flex:1' }), h('button', { class: 'tiny del', onclick: () => { delete S.def.lore[k]; touched(); renderMain(); } }, '✕')));
    sub.appendChild(field('title', textInput(lo, 'title')));
    sub.appendChild(field('text', textArea(lo, 'text', 3)));
    sub.appendChild(field('tags', stringList(lo.tags, { ph: 'tag' })));
    box.appendChild(sub);
  }
  box.appendChild(h('button', { class: 'tiny add-btn', onclick: () => {
    let i = 1, nk = subjectId + '_lore'; while (nk in S.def.lore) nk = subjectId + '_lore' + (++i);
    S.def.lore[nk] = { title: '', text: '', subject: subjectId }; touched(); renderMain();
  } }, '+ add lore about ' + nameOf(subjectId)));
  return box;
}

function newCharacter() {
  S.def.entityTypes = S.def.entityTypes || {};
  if (!S.def.entityTypes.character) S.def.entityTypes.character = { description: 'A person in the story', attributes: { name: { type: 'string' } } };
  S.def.entities = S.def.entities || {};
  const id = uniqueId('character', x => x in S.def.entities);
  S.def.entities[id] = { type: 'character', attrs: { name: 'New character' } };
  S.charSel = id; touched(); refresh();
}

function ensureCharLocationAttr(e, pt, moverAttr) {
  const et = S.def.entityTypes[e.type]; et.attributes = et.attributes || {};
  if (!et.attributes[moverAttr] || et.attributes[moverAttr].type !== 'ref') et.attributes[moverAttr] = { type: 'ref', refType: pt };
}

function archetypeSelect(e, pt) {
  const types = Object.keys(S.def.entityTypes || {}).filter(t => t !== pt);
  return selectInput(e, 'type', types, { allowEmpty: false, onChange: () => { touched(); renderMain(); } });
}

// statBlock edits the character's stats/skills. These live on the archetype
// (entity type) so they're shared by every character of that archetype; values
// are per-character. Adding/removing a stat updates the archetype.
function statBlock(id, pt) {
  const e = S.def.entities[id]; e.attrs = e.attrs || {};
  const et = S.def.entityTypes[e.type]; et.attributes = et.attributes || {};
  const moverAttr = (ed().map && ed().map.moverAttr) || 'location';
  const stats = Object.keys(et.attributes).filter(a => a !== 'name' && a !== moverAttr && !(et.attributes[a].type === 'ref' && et.attributes[a].refType === pt));
  const box = h('div', {});
  if (!stats.length) box.appendChild(h('div', { class: 'empty' }, 'no stats — add Strength, Charm, Lockpicking… whatever your game needs'));
  for (const a of stats) {
    const spec = et.attributes[a];
    let valCtl;
    if (spec.type === 'int' || spec.type === 'float') {
      valCtl = h('input', { type: 'number', value: e.attrs[a] ?? '', style: 'max-width:110px' });
      valCtl.addEventListener('input', () => { e.attrs[a] = valCtl.value === '' ? undefined : Number(valCtl.value); touched(); });
    } else if (spec.type === 'bool') {
      valCtl = checkbox(e.attrs, a, '');
    } else {
      valCtl = h('input', { type: 'text', value: e.attrs[a] ?? '' });
      valCtl.addEventListener('input', () => { e.attrs[a] = valCtl.value || undefined; touched(); });
    }
    box.appendChild(h('div', { class: 'kv-row' },
      h('span', { class: 'key', style: 'align-self:center' }, a + (spec.type !== 'int' ? ` (${spec.type})` : '')),
      valCtl,
      h('button', { class: 'tiny del', title: 'remove this stat from the archetype', onclick: () => {
        delete et.attributes[a];
        for (const c of Object.values(S.def.entities)) if (c.type === e.type && c.attrs) delete c.attrs[a];
        touched(); renderMain();
      } }, '✕')));
  }
  box.appendChild(h('button', { class: 'tiny add-btn', onclick: () => {
    let i = 1, nm = 'stat'; while (nm in et.attributes) nm = 'stat' + (++i);
    et.attributes[nm] = { type: 'int', default: 0 }; e.attrs[nm] = 0; touched(); renderMain();
  } }, '+ add stat'));
  return box;
}

function charRelationships(id, pt) {
  S.def.relationships = S.def.relationships || [];
  const box = h('div', {});
  const relTypes = relTypeNames().filter(t => { const rt = S.def.relationshipTypes[t]; return rt.from !== pt && rt.to !== pt; });
  const rels = S.def.relationships.filter(r => (r.from === id || r.to === id) && r.type !== ((ed().map && ed().map.exitType) || 'exit'));
  if (!rels.length) box.appendChild(h('div', { class: 'empty' }, 'no relationships yet'));
  for (const r of rels) {
    const other = r.from === id ? r.to : r.from;
    const arrow = r.from === id ? `→ ${nameOf(other)}` : `← ${nameOf(other)}`;
    r.attrs = r.attrs || {};
    const sub = h('div', { class: 'subcard' });
    sub.appendChild(h('div', { class: 'kv-row' }, h('span', { style: 'flex:1' }, `${r.type} ${arrow}`),
      h('button', { class: 'tiny del', onclick: () => { const i = S.def.relationships.indexOf(r); if (i >= 0) S.def.relationships.splice(i, 1); touched(); renderMain(); } }, '✕')));
    sub.appendChild(kvEditor(r.attrs, { valueKind: 'value' }));
    box.appendChild(sub);
  }
  if (!relTypes.length) { box.appendChild(h('div', { class: 'hint' }, 'define a relationship type (Advanced → Types) to connect characters')); return box; }
  const others = Object.keys(S.def.entities).filter(x => x !== id && S.def.entities[x].type !== pt);
  const pick = { type: relTypes[0], to: '' };
  const relSel = selectInput(pick, 'type', relTypes, { allowEmpty: false });
  const toSel = selectInput(pick, 'to', others.map(o => ({ value: o, label: nameOf(o) })), { emptyLabel: 'with…' });
  box.appendChild(h('div', { class: 'inline', style: 'margin-top:6px;gap:6px' }, relSel, toSel,
    h('button', { class: 'tiny', onclick: () => { if (!pick.to) return; S.def.relationships.push({ type: pick.type, from: id, to: pick.to, attrs: {} }); touched(); renderMain(); } }, '+ add')));
  return box;
}

function characterDetail(id, pt) {
  const e = S.def.entities[id]; e.attrs = e.attrs || {}; e.inventory = e.inventory || {}; e.equipped = e.equipped || {};
  const root = h('div', {});

  const idc = h('div', { class: 'card' });
  const nameInp = h('input', { type: 'text', value: e.attrs.name || '' });
  nameInp.addEventListener('input', () => { e.attrs.name = nameInp.value || undefined; touched(); });
  nameInp.addEventListener('change', () => { const nid = uniqueId(slugify(e.attrs.name || 'character'), x => x !== id && x in S.def.entities); if (nid !== id) { renameEntity(id, nid); S.charSel = nid; touched(); renderMain(); } });
  idc.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, nameOf(id)),
    h('span', { class: 'hint', style: 'margin-left:6px' }, id), h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => { if (!confirm('Delete ' + nameOf(id) + '?')) return; delete S.def.entities[id]; S.def.relationships = (S.def.relationships || []).filter(r => r.from !== id && r.to !== id); S.charSel = null; touched(); refresh(); } }, '✕ delete')));
  idc.appendChild(h('div', { class: 'row' }, field('name', nameInp), field('archetype', archetypeSelect(e, pt), 'characters of one archetype share a stat sheet')));
  idc.appendChild(field('bio', textArea(e, 'description', 3), 'a short description the narrator can draw on'));
  root.appendChild(idc);

  const sc = h('div', { class: 'card' });
  sc.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Stats & skills'), h('span', { class: 'hint', style: 'margin-left:6px' }, 'shared by archetype “' + e.type + '” · used as check modifiers')));
  sc.appendChild(statBlock(id, pt));
  root.appendChild(sc);

  const ic = h('div', { class: 'card' });
  ic.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Inventory & equipment')));
  ic.appendChild(h('div', { class: 'pt-section-label' }, 'Carries (item → count)'));
  ic.appendChild(itemTypeNames().length ? kvEditor(e.inventory, { valueKind: 'int' }) : h('div', { class: 'hint' }, 'define items on the Items page first'));
  ic.appendChild(h('div', { class: 'pt-section-label' }, 'Wears (slot → item)'));
  ic.appendChild(kvEditor(e.equipped, { valueKind: 'string' }));
  root.appendChild(ic);

  const rc = h('div', { class: 'card' });
  rc.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Relationships')));
  rc.appendChild(charRelationships(id, pt));
  root.appendChild(rc);

  if (pt && Object.values(S.def.entities).some(x => x.type === pt)) {
    const lc = h('div', { class: 'card' });
    lc.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Starting location')));
    const places = Object.keys(S.def.entities).filter(x => S.def.entities[x].type === pt);
    const moverAttr = (ed().map && ed().map.moverAttr) || 'location';
    lc.appendChild(selectInput(e.attrs, moverAttr, places.map(p => ({ value: p, label: nameOf(p) })), { onChange: () => { ensureCharLocationAttr(e, pt, moverAttr); touched(); } }));
    lc.appendChild(h('div', { class: 'hint', style: 'margin-top:4px' }, 'or place them visually on the Map page'));
    root.appendChild(lc);
  }

  const loc = h('div', { class: 'card' });
  loc.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Lore about ' + nameOf(id))));
  loc.appendChild(loreAbout(id));
  root.appendChild(loc);
  return root;
}

function renderCharacters(main) {
  S.def.entities = S.def.entities || {}; S.def.entityTypes = S.def.entityTypes || {};
  const pt = detectPlaceType(S.def);
  const chars = Object.keys(S.def.entities).filter(id => S.def.entities[id].type !== pt);
  main.appendChild(h('p', { class: 'section-blurb' }, 'Your cast. Each character has a name, an archetype, stats/skills (which become check modifiers), what they carry, who they know, where they start, and the lore about them.'));
  const wrap = h('div', { style: 'display:flex;gap:16px;align-items:flex-start' });
  const rail = h('div', { style: 'flex:0 0 220px' });
  rail.appendChild(h('button', { class: 'primary add-btn', style: 'width:100%;margin-bottom:8px', onclick: newCharacter }, '+ new character'));
  if (!chars.length) rail.appendChild(h('div', { class: 'empty' }, 'No characters yet.'));
  for (const id of chars) {
    const sel = S.charSel === id;
    rail.appendChild(h('div', { class: 'nav-item' + (sel ? ' active' : ''), style: 'border:1px solid var(--border);margin-bottom:4px', onclick: () => { S.charSel = id; renderMain(); } },
      h('span', {}, nameOf(id)), h('span', { class: 'count' }, S.def.entities[id].type)));
  }
  wrap.appendChild(rail);
  const detail = h('div', { style: 'flex:1;min-width:0' });
  if (!S.charSel || !S.def.entities[S.charSel] || S.def.entities[S.charSel].type === pt) detail.appendChild(h('div', { class: 'empty' }, 'Select a character on the left, or create one.'));
  else detail.appendChild(characterDetail(S.charSel, pt));
  wrap.appendChild(detail);
  main.appendChild(wrap);
}

// ======================= ITEMS =======================
function itemDetail(id) {
  const it = S.def.itemTypes[id]; it.attributes = it.attributes || {};
  const root = h('div', {});
  const c = h('div', { class: 'card' });
  const nameInp = h('input', { class: 'key', type: 'text', value: id });
  nameInp.addEventListener('change', () => {
    const nid = slugify(nameInp.value);
    if (nid && nid !== id && !(nid in S.def.itemTypes)) {
      S.def.itemTypes[nid] = S.def.itemTypes[id]; delete S.def.itemTypes[id];
      for (const e of Object.values(S.def.entities || {})) {
        if (e.inventory && e.inventory[id] !== undefined) { e.inventory[nid] = e.inventory[id]; delete e.inventory[id]; }
        if (e.equipped) for (const sl in e.equipped) if (e.equipped[sl] === id) e.equipped[sl] = nid;
      }
      S.itemSel = nid; touched(); renderMain();
    } else nameInp.value = id;
  });
  c.appendChild(h('div', { class: 'card-head' }, nameInp, h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => { delete S.def.itemTypes[id]; S.itemSel = null; touched(); refresh(); } }, '✕ delete')));
  c.appendChild(field('description', textArea(it, 'description', 3)));
  c.appendChild(h('div', { class: 'row' }, field('category', textInput(it, 'category'), 'groups items for equip slots'), field('max stack', textInput(it, 'maxStack', { type: 'number' }))));
  c.appendChild(h('div', { class: 'field' }, checkbox(it, 'equippable', 'can be equipped (worn/wielded)')));
  c.appendChild(h('div', { class: 'pt-section-label' }, 'Attributes (free-form, e.g. damage, brightness)'));
  c.appendChild(kvEditor(it.attributes, { valueKind: 'value' }));
  const accepts = [];
  for (const [tn, et] of Object.entries(S.def.entityTypes || {})) for (const [sn, sl] of Object.entries(et.slots || {})) if ((sl.accepts || []).includes(it.category)) accepts.push(`${tn}.${sn}`);
  c.appendChild(h('div', { class: 'pt-section-label' }, 'Fits equipment slots'));
  c.appendChild(h('div', { class: 'hint' }, it.category ? (accepts.length ? accepts.join(', ') : `no slots accept category “${it.category}” yet — add a slot on a character archetype`) : 'set a category for this item to fit equip slots'));
  root.appendChild(c);
  const lc = h('div', { class: 'card' });
  lc.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Lore about ' + id)));
  lc.appendChild(loreAbout(id));
  root.appendChild(lc);
  return root;
}

function renderItems(main) {
  S.def.itemTypes = S.def.itemTypes || {};
  main.appendChild(h('p', { class: 'section-blurb' }, 'Things characters carry, wear, or find. Define each once, then place them via characters, the map, or story effects.'));
  const wrap = h('div', { style: 'display:flex;gap:16px;align-items:flex-start' });
  const rail = h('div', { style: 'flex:0 0 220px' });
  rail.appendChild(h('button', { class: 'primary add-btn', style: 'width:100%;margin-bottom:8px', onclick: () => { const id = uniqueId('item', x => x in S.def.itemTypes); S.def.itemTypes[id] = { description: '' }; S.itemSel = id; touched(); refresh(); } }, '+ new item'));
  const ids = Object.keys(S.def.itemTypes);
  if (!ids.length) rail.appendChild(h('div', { class: 'empty' }, 'No items yet.'));
  for (const id of ids) {
    const sel = S.itemSel === id;
    rail.appendChild(h('div', { class: 'nav-item' + (sel ? ' active' : ''), style: 'border:1px solid var(--border);margin-bottom:4px', onclick: () => { S.itemSel = id; renderMain(); } },
      h('span', {}, id), S.def.itemTypes[id].equippable ? h('span', { class: 'count' }, 'wear') : null));
  }
  wrap.appendChild(rail);
  const detail = h('div', { style: 'flex:1;min-width:0' });
  if (!S.itemSel || !S.def.itemTypes[S.itemSel]) detail.appendChild(h('div', { class: 'empty' }, 'Select an item on the left, or create one.'));
  else detail.appendChild(itemDetail(S.itemSel));
  wrap.appendChild(detail);
  main.appendChild(wrap);
}

// ======================= STORY FLOW =======================
function storyZoomControls() {
  const setZ = (z) => { S.storyZoom = clampZoom(z); renderMain(); };
  return h('span', { class: 'inline', style: 'gap:4px' },
    h('button', { class: 'tiny', onclick: () => setZ((S.storyZoom || 1) - 0.15) }, '−'),
    h('span', { class: 'hint', style: 'min-width:40px;text-align:center' }, Math.round((S.storyZoom || 1) * 100) + '%'),
    h('button', { class: 'tiny', onclick: () => setZ((S.storyZoom || 1) + 0.15) }, '+'));
}

function ensureStoryPos(mid) {
  const e = ed(); e.story = e.story || {}; e.story[mid] = e.story[mid] || { pos: {} };
  const pos = e.story[mid].pos = e.story[mid].pos || {};
  const m = S.def.machines[mid];
  const cols = Math.max(1, Math.ceil(Math.sqrt((m.states || []).length)));
  let i = 0;
  for (const st of (m.states || [])) { if (!pos[st]) pos[st] = { x: 60 + (i % cols) * 240, y: 50 + Math.floor(i / cols) * 150 }; i++; }
  return pos;
}

function scaffoldStory() {
  S.def.machines = S.def.machines || {};
  if (!S.def.machines.arc) S.def.machines.arc = { description: 'The main story arc', initial: 'start', states: ['start', 'end'], stateMeta: { end: { description: 'The story concludes.', terminal: true, ending: true } }, transitions: [{ id: 'finish', from: 'start', to: 'end' }] };
  S.storyTab = 'arc'; touched(); refresh();
}

function addMachine() {
  let i = 1, nk = 'arc'; while (nk in (S.def.machines || {})) nk = 'arc' + (++i);
  S.def.machines[nk] = { initial: 'start', states: ['start', 'end'], stateMeta: { end: { terminal: true, ending: true } }, transitions: [{ id: 'finish', from: 'start', to: 'end' }] };
  S.storyTab = nk; S.storySel = null; touched(); refresh();
}

function addState(mid) {
  const m = S.def.machines[mid]; m.states = m.states || [];
  let i = 1, nk = 'scene'; while (m.states.includes(nk)) nk = 'scene' + (++i);
  m.states.push(nk);
  S.storySel = { kind: 'state', id: nk }; touched(); renderMain();
}

// renameState updates every reference: states, initial, stateMeta, transitions,
// positions, beats, and scenes (so the flow graph stays consistent).
function renameState(mid, oldS, newS) {
  const m = S.def.machines[mid];
  if (!newS || newS === oldS || m.states.includes(newS)) return;
  m.states = m.states.map(s2 => s2 === oldS ? newS : s2);
  if (m.initial === oldS) m.initial = newS;
  if (m.stateMeta && m.stateMeta[oldS]) { m.stateMeta[newS] = m.stateMeta[oldS]; delete m.stateMeta[oldS]; }
  for (const tr of (m.transitions || [])) {
    if (tr.to === oldS) tr.to = newS;
    if (Array.isArray(tr.from)) tr.from = tr.from.map(f => f === oldS ? newS : f);
    else if (tr.from === oldS) tr.from = newS;
  }
  const pos = ed().story && ed().story[mid] && ed().story[mid].pos;
  if (pos && pos[oldS]) { pos[newS] = pos[oldS]; delete pos[oldS]; }
  for (const b of Object.values(S.def.beats || {})) if (b.machineState && b.machineState.machine === mid && b.machineState.state === oldS) b.machineState.state = newS;
  for (const sc of Object.values((ed().scenes) || {})) if (sc.machine === mid && sc.state === oldS) sc.state = newS;
  if (typeof syncScenes === 'function' && ed().map) syncScenes(mapCfg());
}

function deleteState(mid, st) {
  const m = S.def.machines[mid];
  m.states = m.states.filter(s2 => s2 !== st);
  if (m.stateMeta) delete m.stateMeta[st];
  m.transitions = (m.transitions || []).filter(tr => tr.to !== st && !(Array.isArray(tr.from) ? tr.from.includes(st) : tr.from === st));
  if (m.initial === st) m.initial = m.states[0] || '';
  const pos = ed().story && ed().story[mid] && ed().story[mid].pos; if (pos) delete pos[st];
  S.storySel = null; touched(); refresh();
}

function storyCanvas(mid) {
  const m = S.def.machines[mid];
  const pos = ensureStoryPos(mid);
  const zoom = S.storyZoom || 1;
  const NW = 150, NH = 50;
  let maxX = 1000, maxY = 700;
  for (const st of m.states) { const p = pos[st]; if (p) { maxX = Math.max(maxX, p.x + 320); maxY = Math.max(maxY, p.y + 240); } }
  const W = maxX * zoom, H = maxY * zoom;
  const box = h('div', { style: 'flex:1;min-width:0;align-self:stretch;overflow:auto;background:var(--bg);border:1px solid var(--border);border-radius:8px;cursor:grab' });
  const svg = s('svg', { width: W, height: H, style: 'display:block' });
  svg.appendChild(s('defs', {}, s('marker', { id: 'flowarrow', viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 8, markerHeight: 8, orient: 'auto-start-reverse' }, s('path', { d: 'M0,0 L10,5 L0,10 z', fill: '#5a6180' }))));

  const bg = s('rect', { x: 0, y: 0, width: W, height: H, fill: 'transparent' });
  bg.addEventListener('mousedown', (ev) => {
    ev.preventDefault();
    const sx = ev.clientX, sy = ev.clientY, sl = box.scrollLeft, stp = box.scrollTop; let moved = false;
    const mm = (e2) => { moved = true; box.scrollLeft = sl - (e2.clientX - sx); box.scrollTop = stp - (e2.clientY - sy); };
    const mu = () => { document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', mu); if (!moved && S.storySel) { S.storySel = null; renderMain(); } };
    document.addEventListener('mousemove', mm); document.addEventListener('mouseup', mu);
  });
  svg.appendChild(bg);
  box.addEventListener('wheel', (e) => {
    e.preventDefault();
    const rect = box.getBoundingClientRect(); const ox = e.clientX - rect.left, oy = e.clientY - rect.top;
    const fx = (ox + box.scrollLeft) / zoom, fy = (oy + box.scrollTop) / zoom;
    const nz = clampZoom((S.storyZoom || 1) * (e.deltaY < 0 ? 1.12 : 1 / 1.12));
    if (nz === (S.storyZoom || 1)) return;
    S.storyZoom = nz; S.storyAnchor = { fx, fy, ox, oy }; renderMain();
  }, { passive: false });

  const ctr = (st) => { const p = pos[st]; return { x: (p.x + NW / 2) * zoom, y: (p.y + NH / 2) * zoom }; };
  const edgeG = s('g', {}); svg.appendChild(edgeG);
  const pairs = {};
  (m.transitions || []).forEach((tr, ti) => {
    const froms = Array.isArray(tr.from) ? tr.from : [tr.from];
    froms.forEach(f => { const fr = (f === '*' || f == null) ? m.initial : f; if (!pos[fr] || !pos[tr.to] || fr === tr.to) return; const key = fr + '>' + tr.to; (pairs[key] = pairs[key] || []).push({ tr, ti, fr }); });
  });
  for (const key in pairs) {
    const list = pairs[key];
    list.forEach((ed2, idx) => {
      const a = ctr(ed2.fr), b = ctr(ed2.tr.to);
      const dx = b.x - a.x, dy = b.y - a.y, len = Math.hypot(dx, dy) || 1, ux = dx / len, uy = dy / len, nx = -uy, ny = ux;
      const off = (idx - (list.length - 1) / 2) * 18;
      const x1 = a.x + ux * (NW / 2 + 6) * zoom + nx * off, y1 = a.y + uy * 20 * zoom + ny * off;
      const x2 = b.x - ux * (NW / 2 + 8) * zoom + nx * off, y2 = b.y - uy * 22 * zoom + ny * off;
      const seld = S.storySel && S.storySel.kind === 'transition' && S.storySel.ti === ed2.ti;
      const line = s('line', { x1, y1, x2, y2, stroke: seld ? '#7c9cff' : '#5a6180', 'stroke-width': seld ? 3 : 2, 'marker-end': 'url(#flowarrow)', style: 'cursor:pointer' });
      line.addEventListener('mousedown', (ev) => { ev.stopPropagation(); ev.preventDefault(); S.storySel = { kind: 'transition', ti: ed2.ti }; renderMain(); });
      edgeG.appendChild(line);
      edgeG.appendChild(s('text', { x: (x1 + x2) / 2 + nx * 9, y: (y1 + y2) / 2 + ny * 9 + 3, 'text-anchor': 'middle', fill: '#9aa3b2', 'font-size': 11, style: 'pointer-events:none' }, ed2.tr.id));
    });
  }

  for (const st of m.states) {
    const p = pos[st]; const meta = (m.stateMeta || {})[st] || {}; const ending = meta.ending || meta.terminal; const isInit = m.initial === st;
    const selfLoop = (m.transitions || []).some(tr => tr.to === st && (Array.isArray(tr.from) ? tr.from.includes(st) : tr.from === st));
    const seld = S.storySel && S.storySel.kind === 'state' && S.storySel.id === st;
    const g = s('g', { transform: `translate(${p.x * zoom},${p.y * zoom})`, style: 'cursor:move' });
    g.appendChild(s('rect', { x: 0, y: 0, width: NW * zoom, height: NH * zoom, rx: 9, fill: ending ? '#1f2c1c' : (seld ? '#222b44' : '#1b1e26'), stroke: seld ? '#7c9cff' : (ending ? '#4caf7d' : '#2e3342'), 'stroke-width': seld ? 2 : 1 }));
    g.appendChild(s('text', { x: 9 * zoom, y: 21 * zoom, fill: '#e6e8ee', 'font-size': 13 * zoom, 'font-weight': 600, style: 'pointer-events:none' }, st));
    g.appendChild(s('text', { x: 9 * zoom, y: 38 * zoom, fill: ending ? '#4caf7d' : '#9aa3b2', 'font-size': 10 * zoom, style: 'pointer-events:none' }, [isInit ? '● start' : '', ending ? '■ ending' : '', selfLoop ? '↺' : ''].filter(Boolean).join('  ')));
    let sx, sy, ox, oy, moved;
    g.addEventListener('mousedown', (ev) => {
      ev.stopPropagation(); ev.preventDefault();
      sx = ev.clientX; sy = ev.clientY; ox = p.x; oy = p.y; moved = false;
      const mm = (e2) => { p.x = Math.max(0, ox + (e2.clientX - sx) / zoom); p.y = Math.max(0, oy + (e2.clientY - sy) / zoom); if (Math.abs(e2.clientX - sx) + Math.abs(e2.clientY - sy) > 3) moved = true; g.setAttribute('transform', `translate(${p.x * zoom},${p.y * zoom})`); };
      const mu = () => { document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', mu); if (moved) { touched(); renderMain(); } else { S.storySel = { kind: 'state', id: st }; renderMain(); } };
      document.addEventListener('mousemove', mm); document.addEventListener('mouseup', mu);
    });
    svg.appendChild(g);
  }
  box.appendChild(svg);
  box.addEventListener('scroll', () => { S.storyScroll = { left: box.scrollLeft, top: box.scrollTop }; });
  requestAnimationFrame(() => {
    if (S.storyAnchor) { box.scrollLeft = S.storyAnchor.fx * zoom - S.storyAnchor.ox; box.scrollTop = S.storyAnchor.fy * zoom - S.storyAnchor.oy; S.storyAnchor = null; }
    else if (S.storyScroll) { box.scrollLeft = S.storyScroll.left; box.scrollTop = S.storyScroll.top; }
    S.storyScroll = { left: box.scrollLeft, top: box.scrollTop };
  });
  return box;
}

function stateInspector(mid, st) {
  const m = S.def.machines[mid]; m.stateMeta = m.stateMeta || {};
  const meta = m.stateMeta[st] = m.stateMeta[st] || {};
  const box = h('div', {});
  const nameInp = h('input', { class: 'key', type: 'text', value: st });
  nameInp.addEventListener('change', () => { renameState(mid, st, nameInp.value.trim()); S.storySel = { kind: 'state', id: nameInp.value.trim() }; touched(); refresh(); });
  box.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Scene / state'), h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => deleteState(mid, st) }, '✕ delete')));
  box.appendChild(field('id', nameInp));
  box.appendChild(field('description', textArea(meta, 'description', 3), 'what the narrator sees in this scene'));
  box.appendChild(h('div', { class: 'field' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = m.initial === st; c.addEventListener('change', () => { if (c.checked) { m.initial = st; touched(); renderMain(); } }); return h('label', { class: 'checkbox' }, c, 'starting scene'); })()));
  box.appendChild(h('div', { class: 'field' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = !!meta.terminal; c.addEventListener('change', () => { meta.terminal = c.checked || undefined; touched(); renderMain(); }); return h('label', { class: 'checkbox' }, c, 'terminal (no actions out)'); })()));
  box.appendChild(h('div', { class: 'field' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = !!meta.ending; c.addEventListener('change', () => { meta.ending = c.checked || undefined; touched(); renderMain(); }); return h('label', { class: 'checkbox' }, c, 'an ending'); })()));
  // outgoing actions
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Actions out of this scene'));
  const outs = (m.transitions || []).map((tr, ti) => ({ tr, ti })).filter(({ tr }) => (Array.isArray(tr.from) ? tr.from.includes(st) : tr.from === st) || tr.from === '*');
  if (!outs.length) box.appendChild(h('div', { class: 'empty' }, 'none'));
  for (const { tr, ti } of outs) box.appendChild(h('button', { class: 'action-btn', onclick: () => { S.storySel = { kind: 'transition', ti }; renderMain(); } }, `${tr.id} → ${tr.to}`));
  box.appendChild(h('button', { class: 'tiny add-btn', onclick: () => {
    m.transitions = m.transitions || [];
    let i = 1, nk = 'action'; while ((m.transitions).some(t => t.id === nk)) nk = 'action' + (++i);
    m.transitions.push({ id: nk, from: st, to: m.states[0] || st });
    S.storySel = { kind: 'transition', ti: m.transitions.length - 1 }; touched(); renderMain();
  } }, '+ add action from here'));
  return box;
}

function transitionInspector(mid, ti) {
  const m = S.def.machines[mid]; const tr = (m.transitions || [])[ti];
  if (!tr) { S.storySel = null; return h('div', { class: 'empty' }, 'transition gone'); }
  const box = h('div', {});
  box.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Action'), h('span', { class: 'hint', style: 'margin-left:6px' }, `→ ${tr.to}`), h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => { m.transitions.splice(ti, 1); S.storySel = null; touched(); renderMain(); } }, '✕ delete')));
  box.appendChild(h('div', { class: 'hint', style: 'margin-bottom:8px' }, 'Build the action from blocks below — a guard that gates it, and effects (checks, moves, items, lore, flags, journal…) that fire when it runs.'));
  box.appendChild(transitionEditor(tr, m));
  return box;
}

function storyInspector(mid) {
  const m = S.def.machines[mid];
  if (S.storySel && S.storySel.kind === 'state' && m.states.includes(S.storySel.id)) return stateInspector(mid, S.storySel.id);
  if (S.storySel && S.storySel.kind === 'transition') return transitionInspector(mid, S.storySel.ti);
  // overview
  const box = h('div', {});
  box.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, 'Arc: ' + mid)));
  box.appendChild(field('description', textArea(m, 'description', 2)));
  box.appendChild(field('intent', textInput(m, 'intent')));
  box.appendChild(field('starting scene', selectInput(m, 'initial', m.states, { allowEmpty: false, onChange: () => { touched(); renderMain(); } })));
  box.appendChild(h('div', { class: 'hint', style: 'margin-top:10px' }, 'Click a scene to edit it or add actions; click an action (arrow) to build what it checks and does. Green = an ending.'));
  return box;
}

function renderStoryFlow(main) {
  S.def.machines = S.def.machines || {};
  const globals = Object.keys(S.def.machines).filter(mid => !S.def.machines[mid].attach);
  if (!globals.length) {
    main.appendChild(h('p', { class: 'section-blurb' }, 'The story flow is a graph of scenes connected by actions, with endings as terminal scenes. None yet.'));
    main.appendChild(h('button', { class: 'primary', onclick: scaffoldStory }, '+ Create a story arc'));
    return;
  }
  if (!globals.includes(S.storyTab)) { S.storyTab = globals[0]; S.storySel = null; }
  const mid = S.storyTab;
  const bar = h('div', { class: 'tabs' });
  for (const g of globals) bar.appendChild(h('div', { class: 'tab' + (S.storyTab === g ? ' active' : ''), onclick: () => { S.storyTab = g; S.storySel = null; renderMain(); } }, g));
  bar.appendChild(h('div', { class: 'tab', style: 'color:var(--accent)', onclick: addMachine }, '+ arc'));
  main.appendChild(bar);
  const tb = h('div', { class: 'inline', style: 'margin:10px 0;gap:12px' });
  tb.appendChild(h('button', { class: 'primary tiny', onclick: () => addState(mid) }, '+ add scene'));
  tb.appendChild(storyZoomControls());
  tb.appendChild(h('span', { class: 'hint' }, 'drag scenes to arrange · click a scene or action to edit · drag empty space to pan · scroll to zoom'));
  main.appendChild(tb);
  const wrap = h('div', { style: 'display:flex;gap:14px;align-items:stretch;flex:1;min-height:0' });
  wrap.appendChild(storyCanvas(mid));
  const panel = h('div', { style: 'flex:0 0 340px;overflow:auto' });
  panel.appendChild(storyInspector(mid));
  wrap.appendChild(panel);
  main.appendChild(wrap);
}

function renderJSON(main) {
  main.appendChild(h('p', { class: 'section-blurb' }, 'The whole definition as JSON — the universal fallback. Edit and Apply to update the editor, or just review.'));
  const ta = h('textarea', { class: 'json-area' });
  ta.value = JSON.stringify(S.def, null, 2);
  main.appendChild(h('div', { class: 'inline', style: 'margin-bottom:8px' },
    h('button', { class: 'primary', onclick: () => {
      try { const obj = JSON.parse(ta.value); S.def = obj; touched(); toast('JSON applied', 'good'); refresh(); }
      catch (e) { toast('Invalid JSON: ' + e.message, 'bad'); }
    } }, 'Apply JSON to editor'),
    h('button', { onclick: () => { try { ta.value = JSON.stringify(JSON.parse(ta.value), null, 2); } catch (e) { toast('Invalid JSON', 'bad'); } } }, 'Format')));
  main.appendChild(ta);
  main.appendChild(renderValidationList());
}

function renderValidationList() {
  const wrap = h('div', { class: 'validation-list' });
  if (!S.validation.length) { wrap.appendChild(h('div', { class: 'empty' }, '✓ no validation problems')); return wrap; }
  wrap.appendChild(h('div', { class: 'pt-section-label' }, `${S.validation.length} validation problem(s)`));
  for (const v of S.validation) wrap.appendChild(h('div', { class: 'validation-item' }, h('span', { class: 'vpath' }, v.path), h('span', {}, v.message)));
  return wrap;
}

// ---------- main render ----------
function renderSidebar() {
  const nav = $('#sidebar');
  nav.innerHTML = '';
  const c = counts();
  for (const [group, items] of NAV) {
    nav.appendChild(h('div', { class: 'nav-section-label' }, group));
    for (const [id, label] of items) {
      const cnt = c[id];
      nav.appendChild(h('div', { class: 'nav-item' + (S.section === id ? ' active' : ''), onclick: () => { S.section = id; refresh(); } },
        h('span', {}, label),
        cnt ? h('span', { class: 'count' }, cnt) : null));
    }
  }
}

const SECTION_RENDER = {
  overview: (m) => renderGame(m), characters: (m) => renderCharacters(m), items: (m) => renderItems(m),
  map: (m) => renderMap(m), story: (m) => renderStoryFlow(m), beats: (m) => renderBeats(m),
  lore: (m) => renderLore(m), types: (m) => renderTypes(m), world: (m) => renderWorld(m),
  systems: (m) => renderSystems(m), json: (m) => renderJSON(m),
};

function renderMain() {
  const main = $('#main');
  // Map and Story are full-height flex columns so their canvases fill all space;
  // other sections keep the default scrolling block layout.
  const full = S.section === 'map' || S.section === 'story';
  main.style.display = full ? 'flex' : '';
  main.style.flexDirection = full ? 'column' : '';
  main.style.overflow = full ? 'hidden' : '';
  main.innerHTML = '';
  if (!S.def) { main.appendChild(h('div', { class: 'empty' }, 'Open or create a game file to begin.')); return; }
  main.appendChild(h('div', { class: 'section-head' }, h('h1', {}, SECTION_LABEL[S.section] || S.section)));
  (SECTION_RENDER[S.section] || SECTION_RENDER.overview)(main);
}

function refresh() { renderSidebar(); renderMain(); updateValidityPill(); }

// ---------- validation + save ----------
const scheduleValidate = debounce(async () => {
  if (!S.def) return;
  try { const r = await api('POST', '/api/validate', { definition: S.def }); S.validation = r.validation || []; }
  catch (e) { S.validation = [{ path: '(error)', message: e.message }]; }
  updateValidityPill();
  if (S.section === 'json') { /* refresh list under JSON view */ const m = $('#main'); const old = m.querySelector('.validation-list'); if (old) old.replaceWith(renderValidationList()); }
}, 350);

function touched() { S.dirty = true; $('#btn-save').disabled = false; scheduleValidate(); }

function updateValidityPill() {
  const pill = $('#valid-pill');
  if (!S.def) { pill.textContent = '—'; pill.className = 'pill'; return; }
  if (!S.validation.length) { pill.textContent = '✓ valid'; pill.className = 'pill good'; }
  else { pill.textContent = `✗ ${S.validation.length} issue${S.validation.length > 1 ? 's' : ''}`; pill.className = 'pill bad'; }
}

async function save() {
  if (!S.file || !S.def) return;
  try {
    const r = await api('PUT', '/api/games/' + encodeURIComponent(S.file), { definition: S.def });
    S.validation = r.validation || []; S.dirty = false; $('#btn-save').disabled = true;
    updateValidityPill(); await loadFileList();
    toast('Saved ' + S.file, 'good');
  } catch (e) { toast('Save failed: ' + e.message, 'bad'); }
}

// ---------- files ----------
async function loadFileList() {
  S.files = await api('GET', '/api/games');
  const sel = $('#file-select');
  sel.innerHTML = '';
  if (!S.files.length) sel.appendChild(h('option', { value: '' }, '— no files —'));
  for (const f of S.files) {
    sel.appendChild(h('option', { value: f.file }, `${f.file}${f.valid ? '' : ' (⚠ ' + f.errorCount + ')'}`));
  }
  if (S.file) sel.value = S.file;
}

async function openFile(file) {
  if (!file) return;
  if (S.dirty && !confirm('Discard unsaved changes?')) { $('#file-select').value = S.file || ''; return; }
  const r = await api('GET', '/api/games/' + encodeURIComponent(file));
  S.file = file; S.def = JSON.parse(JSON.stringify(r.definition)); S.validation = r.validation || []; S.dirty = false;
  S.mapSel = null; S.mapTab = 'layout'; S.mapScroll = null; // fresh map view per game
  S.charSel = null; S.itemSel = null; S.storyTab = null; S.storySel = null; S.storyScroll = null;
  $('#btn-save').disabled = true;
  refresh();
}

function newGameModal() {
  const idInp = h('input', { type: 'text', placeholder: 'my-game', style: 'width:100%' });
  const backdrop = h('div', { class: 'modal-backdrop' },
    h('div', { class: 'modal' },
      h('h3', {}, 'New game'),
      field('game id', idInp, 'becomes <id>.lono.json with a small starter template'),
      h('div', { class: 'actions' },
        h('button', { onclick: () => backdrop.remove() }, 'Cancel'),
        h('button', { class: 'primary', onclick: async () => {
          const id = idInp.value.trim().replace(/[^A-Za-z0-9 ._-]/g, '-');
          if (!id) return;
          try { const r = await api('POST', '/api/games', { id }); backdrop.remove(); await loadFileList(); await openFile(r.file); toast('Created ' + r.file, 'good'); }
          catch (e) { toast('Create failed: ' + e.message, 'bad'); }
        } }, 'Create'))));
  document.body.appendChild(backdrop);
  idInp.focus();
}

async function deleteFile() {
  if (!S.file) return;
  if (!confirm('Delete ' + S.file + '? This removes the file from disk.')) return;
  try { await api('DELETE', '/api/games/' + encodeURIComponent(S.file)); const gone = S.file; S.file = null; S.def = null; await loadFileList(); refresh(); toast('Deleted ' + gone, 'good'); if (S.files[0]) openFile(S.files[0].file); }
  catch (e) { toast('Delete failed: ' + e.message, 'bad'); }
}

// ---------- playtest ----------
async function openPlaytest() {
  const panel = $('#playtest-panel');
  panel.classList.remove('hidden');
  panel.innerHTML = '';
  panel.appendChild(h('div', { class: 'pt-head' },
    h('h2', {}, '▶ Playtest'),
    h('label', { style: 'font-size:12px;color:var(--muted)' }, 'seed'),
    (() => { const i = h('input', { type: 'number', value: S.ptSeed, style: 'width:80px' }); i.addEventListener('input', () => S.ptSeed = Number(i.value || 0)); return i; })(),
    h('button', { class: 'primary tiny', onclick: startPlaytest }, 'Start / restart'),
    h('button', { class: 'tiny', onclick: () => { panel.classList.add('hidden'); endPlaytest(); } }, '✕')));
  const body = h('div', { class: 'pt-body', id: 'pt-body' });
  panel.appendChild(body);
  body.appendChild(h('div', { class: 'empty' }, 'Press Start to launch an in-memory playthrough of the current (unsaved) definition.'));
}

async function startPlaytest() {
  try {
    const r = await api('POST', '/api/playtest', { definition: S.def, seed: S.ptSeed });
    S.pt = { session: r.session, view: r.view };
    renderPlaytest();
  } catch (e) {
    const body = $('#pt-body'); body.innerHTML = '';
    if (e.data && e.data.validation) {
      body.appendChild(h('div', { class: 'ending', style: 'border-color:var(--bad);background:rgba(226,97,90,.1)' }, 'Fix validation problems before playtesting:'));
      for (const v of e.data.validation) body.appendChild(h('div', { class: 'validation-item' }, h('span', { class: 'vpath' }, v.path), h('span', {}, v.message)));
    } else body.appendChild(h('div', { class: 'ending', style: 'border-color:var(--bad)' }, e.message));
  }
}

async function endPlaytest() { if (S.pt) { try { await api('DELETE', '/api/playtest/' + S.pt.session); } catch {} S.pt = null; } }

async function ptAct(machine, action, host, params) {
  try { const r = await api('POST', `/api/playtest/${S.pt.session}/act`, { machine, action, host, params }); S.pt.view = r.view; S.pt.lastResult = r.result; renderPlaytest(); }
  catch (e) { toast(e.message, 'bad'); }
}
async function ptAdvance() {
  try { const r = await api('POST', `/api/playtest/${S.pt.session}/advance`, { n: 1 }); S.pt.view = r.view; S.pt.lastResult = r.result; renderPlaytest(); }
  catch (e) { toast(e.message, 'bad'); }
}

function renderPlaytest() {
  const body = $('#pt-body'); body.innerHTML = '';
  const v = S.pt.view;
  // endings
  if (v.endingReached && v.endingReached.length) {
    for (const e of v.endingReached) body.appendChild(h('div', { class: 'ending' }, h('strong', {}, '🏁 Ending: ' + e.state), h('div', {}, e.description || '')));
  }
  body.appendChild(h('div', { class: 'inline' }, h('strong', {}, 'clock: ' + v.clock), h('span', { style: 'flex:1' }), h('button', { class: 'tiny', onclick: ptAdvance }, '⏭ advance 1')));

  if (S.pt.lastResult && S.pt.lastResult.fired && S.pt.lastResult.fired.length)
    body.appendChild(h('div', { class: 'fired' }, '⚡ fired: ' + S.pt.lastResult.fired.join(', ')));
  if (S.pt.lastResult && S.pt.lastResult.rolls && S.pt.lastResult.rolls.length)
    body.appendChild(h('div', { class: 'fired' }, '🎲 ' + S.pt.lastResult.rolls.map(r => `${r.dice}=${r.total}`).join(', ')));

  // beats
  if (v.beats && v.beats.length) {
    body.appendChild(h('div', { class: 'pt-section-label' }, 'Active beats'));
    for (const b of v.beats) body.appendChild(h('div', { class: 'beat' }, b.text));
  }

  // actions
  body.appendChild(h('div', { class: 'pt-section-label' }, 'Actions'));
  const acts = (v.actions || []);
  if (!acts.length) body.appendChild(h('div', { class: 'empty' }, 'no actions'));
  for (const a of acts) {
    let label = `${a.machine}.${a.action}  (${a.from}→${a.to})`;
    if (a.host) label += a.host.kind === 'entity' ? `  [${a.host.id}]` : `  [${a.host.from}→${a.host.to}]`;
    const btn = h('button', { class: 'action-btn', disabled: !a.enabled, onclick: () => {
      let params;
      if (a.requiresParams) { const raw = prompt('Params as JSON for ' + a.action, '{}'); if (raw == null) return; try { params = JSON.parse(raw); } catch { toast('bad params JSON', 'bad'); return; } }
      ptAct(a.machine, a.action, a.host, params);
    } }, label);
    body.appendChild(btn);
    if (!a.enabled && a.reason) body.appendChild(h('div', { class: 'action-reason' }, '↳ ' + a.reason));
  }

  // derived
  const der = v.derived || {};
  if (der.global && Object.keys(der.global).length) {
    body.appendChild(h('div', { class: 'pt-section-label' }, 'Derived'));
    body.appendChild(h('pre', { class: 'state-dump' }, JSON.stringify(der.global, null, 2)));
  }

  // log
  if (v.log && v.log.length) {
    body.appendChild(h('div', { class: 'pt-section-label' }, 'Journal'));
    for (const l of v.log.slice(-12)) body.appendChild(h('div', { class: 'log-entry' }, `[${l.clock}] ${l.text}`));
  }

  // state dump
  const dump = h('details', {}, h('summary', { style: 'cursor:pointer;color:var(--muted);margin-top:10px' }, 'raw state'), h('pre', { class: 'state-dump' }, JSON.stringify(v.state, null, 2)));
  body.appendChild(dump);
}

// ---------- boot ----------
async function boot() {
  S.meta = await api('GET', '/api/meta');
  // reduce-verb datalist
  document.body.appendChild(h('datalist', { id: 'reduce-verbs' }, ...S.meta.reduceVerbs.map(r => h('option', { value: r }))));
  $('#file-select').addEventListener('change', (e) => openFile(e.target.value));
  $('#btn-new').addEventListener('click', newGameModal);
  $('#btn-delete').addEventListener('click', deleteFile);
  $('#btn-save').addEventListener('click', save);
  $('#btn-playtest').addEventListener('click', openPlaytest);
  $('#valid-pill').addEventListener('click', () => { if (S.validation.length) { S.section = 'json'; refresh(); } });
  window.addEventListener('keydown', (e) => { if ((e.metaKey || e.ctrlKey) && e.key === 's') { e.preventDefault(); save(); } });

  await loadFileList();
  renderSidebar(); renderMain();
  if (S.files[0]) await openFile(S.files[0].file);
  document.title = 'lono studio — ' + S.meta.dir;
}

boot().catch(e => { document.body.innerHTML = '<pre style="color:#e2615a;padding:20px">Failed to start: ' + e.message + '</pre>'; });
