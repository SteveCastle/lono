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
  validation: [], dirty: false, section: 'game',
  subtab: { types: 'entityTypes', cast: 'entities', systems: 'triggers', map: 'layout' },
  mapSel: null,
  pt: null, ptSeed: 42,
};

const SECTIONS = [
  ['game', 'Game'], ['world', 'World'], ['types', 'Types'], ['cast', 'Cast'],
  ['map', 'Map'], ['story', 'Story'], ['beats', 'Beats'], ['systems', 'Systems'], ['lore', 'Lore'], ['json', 'JSON'],
];

function counts() {
  const d = S.def || {};
  const n = (o) => o ? (Array.isArray(o) ? o.length : Object.keys(o).length) : 0;
  return {
    world: n(d.world) + n(d.setup),
    types: n(d.entityTypes) + n(d.itemTypes) + n(d.relationshipTypes),
    cast: n(d.entities) + n(d.relationships),
    map: placesCount(d),
    story: n(d.machines),
    beats: n(d.beats),
    systems: n(d.triggers) + n(d.derived),
    lore: n(d.lore),
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
  m.positions = m.positions || {};
  return m;
}

function detectMapConfig() {
  let exitType = '', placeType = '';
  for (const [name, rt] of Object.entries(S.def.relationshipTypes || {})) {
    if (rt.from && rt.from === rt.to) { if (name === 'exit' || !exitType) { exitType = name; placeType = rt.from; } }
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
  if (!pt) for (const rt of Object.values(d.relationshipTypes || {})) if (rt.from && rt.from === rt.to) { pt = rt.from; break; }
  if (!pt) return 0;
  return Object.values(d.entities || {}).filter(x => x.type === pt).length;
}

function getPlaces(m) { return Object.keys(S.def.entities || {}).filter(id => S.def.entities[id].type === m.placeType).map(id => ({ id, e: S.def.entities[id] })); }
function getExits(m) { return (S.def.relationships || []).filter(r => r.type === m.exitType); }
function moverTypes(m) { return Object.keys(S.def.entityTypes || {}).filter(t => { const a = (S.def.entityTypes[t].attributes || {})[m.moverAttr]; return a && a.type === 'ref'; }); }
function movers(m) { const mt = new Set(moverTypes(m)); return Object.keys(S.def.entities || {}).filter(id => mt.has(S.def.entities[id].type)); }
function occupantsOf(place, m) { return movers(m).filter(id => (S.def.entities[id].attrs || {})[m.moverAttr] === place); }
function placeName(p) { return (p.e.attrs && p.e.attrs.name) || p.id; }

function renderMap(main) {
  const m = mapCfg();
  const haveType = m.placeType && (m.placeType in (S.def.entityTypes || {}));
  const haveExit = m.exitType && (m.exitType in (S.def.relationshipTypes || {}));
  if (!haveType || !haveExit) {
    main.appendChild(h('p', { class: 'section-blurb' },
      'A map is location entities connected by exit relationships; characters and objects are placed via a location reference. This game has no map structure yet.'));
    main.appendChild(h('button', { class: 'primary', onclick: scaffoldMap }, '+ Create map structure (location + exit)'));
    return;
  }
  main.appendChild(subTabs('map', [['layout', 'Layout & places'], ['scenes', 'Scenes']]));
  if (S.subtab.map === 'scenes') renderScenes(main, m);
  else renderMapLayout(main, m);
}

function scaffoldMap() {
  S.def.entityTypes = S.def.entityTypes || {};
  S.def.relationshipTypes = S.def.relationshipTypes || {};
  if (!S.def.entityTypes.location) S.def.entityTypes.location = { description: 'A place in the world', attributes: { name: { type: 'string' } } };
  if (!S.def.relationshipTypes.exit) S.def.relationshipTypes.exit = { description: 'A passage between places', from: 'location', to: 'location', directed: true, attributes: { direction: { type: 'string' }, locked: { type: 'bool' } } };
  ed().map = { placeType: 'location', exitType: 'exit', moverAttr: 'location', positions: {} };
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

function ensurePositions(places, m) {
  const cx = 480, cy = 250, R = Math.min(210, 80 + places.length * 16);
  places.forEach((p, i) => {
    if (!m.positions[p.id]) {
      const a = (i / Math.max(1, places.length)) * 2 * Math.PI - Math.PI / 2;
      m.positions[p.id] = { x: Math.round(cx + R * Math.cos(a)), y: Math.round(cy + R * Math.sin(a)) };
    }
  });
}

function renderMapLayout(main, m) {
  const places = getPlaces(m);
  const exits = getExits(m);
  ensurePositions(places, m);

  main.appendChild(mapSettings(m));
  main.appendChild(h('div', { class: 'inline', style: 'margin:10px 0' },
    h('button', { class: 'primary tiny', onclick: () => addPlace(m) }, '+ add place'),
    h('span', { class: 'hint' }, 'drag to arrange · click a place to name it, edit its exits, and place who/what is there')));

  const wrap = h('div', { style: 'display:flex;gap:14px;align-items:flex-start' });
  const canvasBox = h('div', { style: 'flex:2;min-width:0;background:var(--bg);border:1px solid var(--border);border-radius:8px;overflow:auto' });
  const Hpx = 540;
  const svg = s('svg', { width: '100%', height: Hpx, viewBox: `0 0 960 ${Hpx}`, style: 'display:block' });
  svg.appendChild(s('defs', {}, s('marker', { id: 'arrow', viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 7, markerHeight: 7, orient: 'auto-start-reverse' }, s('path', { d: 'M0,0 L10,5 L0,10 z', fill: '#5a6180' }))));
  const edgeG = s('g', {}), nodeG = s('g', {});
  svg.appendChild(edgeG); svg.appendChild(nodeG);
  drawEdges(edgeG, exits, m);
  if (!places.length) edgeG.appendChild(s('text', { x: 480, y: 250, 'text-anchor': 'middle', fill: '#9aa3b2', 'font-size': 14 }, 'No places yet — click “+ add place”.'));
  for (const p of places) nodeG.appendChild(drawNode(p, m, svg, edgeG, exits));
  svg.addEventListener('mousedown', (ev) => { if (ev.target === svg || ev.target.tagName === 'svg') { S.mapSel = null; renderMain(); } });
  canvasBox.appendChild(svg);
  wrap.appendChild(canvasBox);
  wrap.appendChild(h('div', { style: 'flex:1;min-width:280px' }, mapInspector(m, places, exits)));
  main.appendChild(wrap);
}

function drawEdges(g, exits, m) {
  g.innerHTML = '';
  for (const r of exits) {
    const a = m.positions[r.from], b = m.positions[r.to];
    if (!a || !b) continue;
    const dx = b.x - a.x, dy = b.y - a.y, len = Math.hypot(dx, dy) || 1, ux = dx / len, uy = dy / len;
    const x1 = a.x + ux * 64, y1 = a.y + uy * 26, x2 = b.x - ux * 66, y2 = b.y - uy * 28;
    const locked = r.attrs && r.attrs.locked;
    g.appendChild(s('line', { x1, y1, x2, y2, stroke: locked ? '#e2615a' : '#4a5068', 'stroke-width': 2, 'stroke-dasharray': locked ? '6 4' : '', 'marker-end': 'url(#arrow)' }));
    const dir = (r.attrs && r.attrs.direction) || '';
    if (dir) g.appendChild(s('text', { x: (x1 + x2) / 2, y: (y1 + y2) / 2 - 4, 'text-anchor': 'middle', fill: '#9aa3b2', 'font-size': 10 }, dir));
  }
}

function drawNode(p, m, svg, edgeG, exits) {
  const pos = m.positions[p.id];
  const sel = S.mapSel === p.id;
  const occ = occupantsOf(p.id, m);
  const g = s('g', { transform: `translate(${pos.x},${pos.y})`, style: 'cursor:move' });
  g.appendChild(s('rect', { x: -62, y: -24, width: 124, height: 48, rx: 9, fill: sel ? '#2f3b66' : '#222633', stroke: sel ? '#7c9cff' : '#2e3342', 'stroke-width': sel ? 2 : 1 }));
  g.appendChild(s('text', { x: 0, y: -3, 'text-anchor': 'middle', fill: '#e6e8ee', 'font-size': 13, 'font-weight': 600 }, placeName(p)));
  g.appendChild(s('text', { x: 0, y: 15, 'text-anchor': 'middle', fill: '#9aa3b2', 'font-size': 10 }, occ.length ? `${occ.length} here` : 'empty'));
  let sx, sy, ox, oy, moved;
  g.addEventListener('mousedown', (ev) => {
    ev.stopPropagation(); ev.preventDefault();
    sx = ev.clientX; sy = ev.clientY; ox = pos.x; oy = pos.y; moved = false;
    const sc = (svg.clientWidth / 960) || 1;
    const mm = (e2) => {
      pos.x = Math.round(ox + (e2.clientX - sx) / sc); pos.y = Math.round(oy + (e2.clientY - sy) / sc);
      if (Math.abs(e2.clientX - sx) + Math.abs(e2.clientY - sy) > 3) moved = true;
      g.setAttribute('transform', `translate(${pos.x},${pos.y})`);
      drawEdges(edgeG, exits, m);
    };
    const mu = () => {
      document.removeEventListener('mousemove', mm); document.removeEventListener('mouseup', mu);
      if (moved) touched(); else { S.mapSel = p.id; renderMain(); }
    };
    document.addEventListener('mousemove', mm); document.addEventListener('mouseup', mu);
  });
  return g;
}

function addPlace(m) {
  const et = S.def.entityTypes[m.placeType];
  const hasName = et && et.attributes && et.attributes.name;
  let i = 1, id = 'place'; while (id in (S.def.entities || {})) id = 'place' + (++i);
  S.def.entities = S.def.entities || {};
  S.def.entities[id] = { type: m.placeType, attrs: hasName ? { name: 'New place' } : {} };
  S.mapSel = id;
  touched(); refresh();
}

function mapInspector(m, places, exits) {
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
  box.appendChild(h('div', { class: 'card-head' }, h('span', { class: 'title' }, sel),
    h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => deletePlace(sel, m) }, '✕ delete place')));
  if (S.def.entityTypes[m.placeType].attributes && S.def.entityTypes[m.placeType].attributes.name)
    box.appendChild(field('name', textInput(e.attrs, 'name')));
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
    const selBox = selectInput({ v: '' }, 'v', uniq, { emptyLabel: '+ place someone/something here…' });
    selBox.addEventListener('change', () => { if (selBox.value) { placeEntity(selBox.value, sel, m); } });
    box.appendChild(h('div', { class: 'field', style: 'margin-top:6px' }, selBox));
  }

  // exits
  box.appendChild(h('div', { class: 'pt-section-label' }, 'Exits from here'));
  const fromHere = exits.filter(r => r.from === sel);
  if (!fromHere.length) box.appendChild(h('div', { class: 'empty' }, 'no exits'));
  for (const r of fromHere) {
    r.attrs = r.attrs || {};
    const sub = h('div', { class: 'subcard' });
    sub.appendChild(h('div', { class: 'kv-row' }, h('span', { style: 'flex:1' }, `→ ${r.to}`),
      h('button', { class: 'tiny', onclick: () => { S.mapSel = r.to; renderMain(); } }, 'go'),
      h('button', { class: 'tiny del', onclick: () => { const i = S.def.relationships.indexOf(r); if (i >= 0) S.def.relationships.splice(i, 1); touched(); renderMain(); } }, '✕')));
    const rtAttrs = (S.def.relationshipTypes[m.exitType].attributes) || {};
    if ('direction' in rtAttrs) sub.appendChild(field('direction', textInput(r.attrs, 'direction', { ph: 'e.g. north' })));
    if ('locked' in rtAttrs) sub.appendChild(h('div', { class: 'field' }, checkbox(r.attrs, 'locked', 'locked')));
    box.appendChild(sub);
  }
  const others = places.filter(p => p.id !== sel && !fromHere.some(r => r.to === p.id));
  if (others.length) {
    const selBox = selectInput({ v: '' }, 'v', others.map(p => p.id), { emptyLabel: '+ add exit to…' });
    selBox.addEventListener('change', () => { if (selBox.value) addExit(sel, selBox.value, m); });
    box.appendChild(h('div', { class: 'field', style: 'margin-top:6px' }, selBox));
  }
  return box;
}

function placeEntity(id, place, m) {
  const e = S.def.entities[id];
  const et = S.def.entityTypes[e.type];
  et.attributes = et.attributes || {};
  if (!et.attributes[m.moverAttr] || et.attributes[m.moverAttr].type !== 'ref')
    et.attributes[m.moverAttr] = { type: 'ref', refType: m.placeType };
  e.attrs = e.attrs || {};
  e.attrs[m.moverAttr] = place;
  touched(); renderMain();
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
  if (ed().map.positions) delete ed().map.positions[id];
  if (S.mapSel === id) S.mapSel = null;
  touched(); refresh();
}

// ---- Scenes ----
function renderScenes(main, m) {
  const scenes = (ed().scenes = ed().scenes || {});
  main.appendChild(h('p', { class: 'section-blurb' },
    'A scene repositions the cast (and marks a beat / journal note) when the story reaches a moment. Each scene compiles to a trigger that fires when a machine enters a chosen state.'));
  const names = Object.keys(scenes);
  if (!names.length) main.appendChild(h('div', { class: 'empty' }, 'No scenes yet.'));
  for (const id of names) main.appendChild(sceneCard(id, scenes, m));
  main.appendChild(h('button', { class: 'primary add-btn', onclick: () => {
    let i = 1, nk = 'scene'; while (nk in scenes) nk = 'scene' + (++i);
    scenes[nk] = { name: 'New scene', placements: {}, once: true };
    syncScenes(m); touched(); renderMain();
  } }, '+ new scene'));
}

function sceneCard(id, scenes, m) {
  const sc = scenes[id];
  sc.placements = sc.placements || {};
  const card = h('div', { class: 'card' });
  card.appendChild(h('div', { class: 'card-head' }, renameInput(scenes, id, () => { syncScenes(m); renderMain(); }),
    h('span', { style: 'flex:1' }),
    h('button', { class: 'tiny del', onclick: () => { delete scenes[id]; syncScenes(m); touched(); renderMain(); } }, '✕ delete')));
  card.appendChild(field('name', textInput(sc, 'name', { onChange: () => { syncScenes(m); touched(); } })));

  // when: machine enters state
  card.appendChild(h('div', { class: 'pt-section-label' }, 'Fires when'));
  card.appendChild(h('div', { class: 'row' },
    field('machine', selectInput(sc, 'machine', machineNames(), { onChange: () => { syncScenes(m); touched(); renderMain(); } })),
    field('enters state', selectInput(sc, 'state', stateNames(sc.machine), { onChange: () => { syncScenes(m); touched(); } }))));
  card.appendChild(h('div', { class: 'field' }, (() => { const c = h('input', { type: 'checkbox' }); c.checked = sc.once !== false; c.addEventListener('change', () => { sc.once = c.checked; syncScenes(m); touched(); }); return h('label', { class: 'checkbox' }, c, 'fire once'); })()));
  if (!sc.machine || !sc.state) card.appendChild(h('div', { class: 'hint' }, 'pick a machine + state so the scene has something to fire on'));

  // placements
  card.appendChild(h('div', { class: 'pt-section-label' }, 'Move the cast'));
  const placesIds = getPlaces(m).map(p => p.id);
  const moverIds = movers(m);
  for (const ent of Object.keys(sc.placements)) {
    card.appendChild(h('div', { class: 'kv-row' },
      selectInput({ v: ent }, 'v', moverIds.length ? moverIds : [ent], { allowEmpty: false, onChange: () => {} }),
      (() => { const sel = selectInput(sc.placements, ent, placesIds, { allowEmpty: false, onChange: () => { syncScenes(m); touched(); } }); return sel; })(),
      h('button', { class: 'tiny del', onclick: () => { delete sc.placements[ent]; syncScenes(m); touched(); renderMain(); } }, '✕')));
  }
  const freeMovers = moverIds.filter(id => !(id in sc.placements));
  if (freeMovers.length && placesIds.length) {
    const selBox = selectInput({ v: '' }, 'v', freeMovers, { emptyLabel: '+ move a character/object…' });
    selBox.addEventListener('change', () => { if (selBox.value) { sc.placements[selBox.value] = placesIds[0]; syncScenes(m); touched(); renderMain(); } });
    card.appendChild(h('div', { class: 'field', style: 'margin-top:6px' }, selBox));
  }
  card.appendChild(h('button', { class: 'tiny', onclick: () => { for (const p of getPlaces(m)) for (const occ of occupantsOf(p.id, m)) sc.placements[occ] = p.id; syncScenes(m); touched(); renderMain(); } }, 'capture current positions'));

  // narrative
  card.appendChild(h('div', { class: 'pt-section-label' }, 'On entry (optional)'));
  card.appendChild(h('div', { class: 'row' },
    field('mark beat', selectInput(sc, 'markBeat', keys(S.def.beats), { onChange: () => { syncScenes(m); touched(); } })),
    field('journal note', textInput(sc, 'record', { onChange: () => { syncScenes(m); touched(); } }))));
  return card;
}

function compileScene(id, sc, m) {
  const effects = [];
  for (const [ent, place] of Object.entries(sc.placements || {})) if (ent && place) effects.push({ op: 'set', target: `entity.${ent}.${m.moverAttr}`, value: place });
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
  for (const [id, label] of SECTIONS) {
    const cnt = c[id];
    nav.appendChild(h('div', { class: 'nav-item' + (S.section === id ? ' active' : ''), onclick: () => { S.section = id; refresh(); } },
      h('span', {}, label),
      cnt ? h('span', { class: 'count' }, cnt) : null));
  }
}

function renderMain() {
  const main = $('#main');
  main.innerHTML = '';
  if (!S.def) { main.appendChild(h('div', { class: 'empty' }, 'Open or create a game file to begin.')); return; }
  const label = SECTIONS.find(s => s[0] === S.section)[1];
  main.appendChild(h('div', { class: 'section-head' }, h('h1', {}, label)));
  ({ game: renderGame, world: renderWorld, types: renderTypes, cast: renderCast, map: renderMap, story: renderStory, beats: renderBeats, systems: renderSystems, lore: renderLore, json: renderJSON }[S.section])(main);
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
