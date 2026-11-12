import { EditorState, Plugin, Selection, TextSelection } from 'prosemirror-state';
import { Decoration, DecorationSet, EditorView } from 'prosemirror-view';
import { baseKeymap, chainCommands, exitCode, toggleMark } from 'prosemirror-commands';
import { closeHistory, history, redo, undo } from 'prosemirror-history';
import { keymap } from 'prosemirror-keymap';
import { inputRules, textblockTypeInputRule, wrappingInputRule, undoInputRule } from 'prosemirror-inputrules';
import { addRowAfter, goToNextCell, tableEditing } from 'prosemirror-tables';
import MarkdownIt from 'markdown-it';
import { inlineRules, parseBlock, schema, serializeBlock } from './document-content.js';
import { insertionPlugin, videoView } from './document-insert.js';

const { paragraph, heading, code_block, blockquote, bullet_list, ordered_list, source_block } = schema.nodes;
const tokenizer = MarkdownIt({ html: true });

function readMarkdown(source) {
  const offsets = [0, ...Array.from(source.matchAll(/\r\n|\n|\r/g), match => match.index + match[0].length)];
  const lines = source.split(/\r\n|\n|\r/);
  const nodes = [];
  const spans = tokenizer.parse(source, {}).filter(token => token.level === 0 && token.map);
  let cursor = 0;
  const originals = [];
  let prefix = '';
  let suffix = '';

  function keep(raw, gap, node) {
    const sourceId = originals.length;
    const tagged = node ? node.type.create({ ...node.attrs, sourceId }, node.content, node.marks)
      : source_block.create({ sourceId, raw });
    originals.push({ raw, gap, node: tagged });
    nodes.push(tagged);
  }

  for (const token of spans) {
    const [line, endLine] = token.map;
    const start = offsets[line];
    if (start < cursor) continue;
    const gap = source.slice(cursor, start);
    if (gap.trim()) keep(gap, '', null);
    else if (!nodes.length) prefix = gap;

    let end = offsets[endLine] ?? source.length;
    const opening = lines[line].trim();
    if (opening === '$$') {
      const closing = lines.findIndex((value, index) => index > line && value.trim() === '$$');
      end = closing < 0 ? source.length : (offsets[closing + 1] ?? source.length);
    }
    const raw = source.slice(start, end).replace(/(?:\r\n|\n|\r)$/, '');
    cursor = start + raw.length;
    let node = null;
    try { if (token.type !== 'html_block') node = parseBlock(raw); }
    catch { /* Keep unsupported Markdown as one source block. */ }
    keep(raw, nodes.length && !gap.trim() ? gap : '', node);
  }
  const tail = source.slice(cursor);
  if (tail.trim()) keep(tail, '', null);
  else if (nodes.length) suffix = tail;
  else prefix = tail;
  const doc = schema.topNodeType.createAndFill(null, nodes.length ? nodes : undefined);
  return { doc, originals, prefix, suffix, source };
}

function writeMarkdown(doc, originalDocument) {
  if (doc.eq(originalDocument.doc)) return originalDocument.source;
  const { originals, prefix, suffix } = originalDocument;
  let output = prefix;
  let previousId = null;
  let written = false;
  const usedIds = new Set();
  doc.forEach(node => {
    if (node.type === paragraph && !node.content.size && node.attrs.sourceId === null) return;
    const id = node.attrs.sourceId;
    const original = id !== null && !usedIds.has(id) ? originals[id] : undefined;
    if (original) usedIds.add(id);
    const raw = node.type === source_block ? node.attrs.raw : original?.node.eq(node) ? original.raw
      : serializeBlock(node);
    if (written) output += original && id === previousId + 1 && previousId !== null ? original.gap : '\n\n';
    output += raw;
    written = true;
    previousId = original ? id : null;
  });
  return output + suffix;
}

function blockStart($pos) {
  return $pos.depth ? $pos.before(1) : $pos.nodeAfter ? $pos.pos : $pos.pos - ($pos.nodeBefore?.nodeSize || 0);
}

class BlockSelection extends Selection {
  constructor(doc, first, last = first) {
    const left = Math.min(first, last);
    const right = Math.max(first, last);
    const end = right + doc.nodeAt(right).nodeSize;
    super(doc.resolve(first <= last ? left : end), doc.resolve(first <= last ? end : left));
    this.first = first;
    this.last = last;
  }
  map(doc, mapping) { return this.getBookmark().map(mapping).resolve(doc); }
  eq(other) { return other instanceof BlockSelection && this.first === other.first && this.last === other.last; }
  toJSON() { return { type: 'prototype-blocks', first: this.first, last: this.last }; }
  static fromJSON(doc, value) { return new BlockSelection(doc, value.first, value.last); }
  getBookmark() { return new BlockBookmark(this.first, this.last); }
}
BlockSelection.prototype.visible = false;
Selection.jsonID('prototype-blocks', BlockSelection);

class BlockBookmark {
  constructor(first, last) { this.first = first; this.last = last; }
  map(mapping) { return new BlockBookmark(mapping.map(this.first, 1), mapping.map(this.last, 1)); }
  resolve(doc) {
    const resolveBlock = pos => blockStart(doc.resolve(Math.max(0, Math.min(doc.content.size, pos))));
    return new BlockSelection(doc, resolveBlock(this.first), resolveBlock(this.last));
  }
}

function chooseBlock(state, dispatch) {
  const { doc, selection } = state;
  const next = selection instanceof BlockSelection ? Selection.near(doc.resolve(selection.from), 1)
    : new BlockSelection(doc, blockStart(selection.$from));
  dispatch(state.tr.setSelection(next).scrollIntoView());
  return true;
}

function moveBlockSelection(direction, extend = false) {
  return (state, dispatch) => {
    const { doc, selection } = state;
    if (!(selection instanceof BlockSelection)) return false;
    const position = selection.last;
    const next = direction > 0 ? position + doc.nodeAt(position).nodeSize
      : position - (doc.resolve(position).nodeBefore?.nodeSize || 0);
    if (next >= doc.content.size || next === position) return true;
    dispatch(state.tr.setSelection(new BlockSelection(doc, extend ? selection.first : next, next)).scrollIntoView());
    return true;
  };
}

function deleteBlocks(state, dispatch, view, emptyAllowed = false) {
  const { doc, selection } = state;
  const { $from } = selection;
  const selected = selection instanceof BlockSelection;
  const empty = emptyAllowed && selection.empty && $from.depth === 1 && !$from.parent.content.size;
  if (view?.composing || (!selected && !empty)) return false;
  if (empty && doc.childCount === 1 && $from.parent.type === paragraph) return true;
  const from = selected ? selection.from : $from.before(1);
  const to = selected ? selection.to : $from.after(1);
  const tr = closeHistory(state.tr);
  if (from === 0 && to === doc.content.size) tr.replaceWith(from, to, paragraph.create());
  else tr.delete(from, to);
  tr.setSelection(Selection.near(tr.doc.resolve(from), -1));
  dispatch(tr.scrollIntoView());
  return true;
}

function enterBlock(state, dispatch, view) {
  const { doc, selection } = state;
  if (view?.composing) return false;
  if (selection instanceof BlockSelection) {
    dispatch(state.tr.setSelection(Selection.near(doc.resolve(selection.from), 1)).scrollIntoView());
    return true;
  }
  const close = selection.$from.parent.type !== code_block;
  return baseKeymap.Enter(state, tr => dispatch(close ? closeHistory(tr) : tr), view);
}

const blockKeys = keymap({
  Escape: chooseBlock,
  ArrowUp: moveBlockSelection(-1),
  ArrowDown: moveBlockSelection(1),
  'Shift-ArrowUp': moveBlockSelection(-1, true),
  'Shift-ArrowDown': moveBlockSelection(1, true),
  Tab: chainCommands(goToNextCell(1), (state, dispatch) => addRowAfter(state, tr => {
    goToNextCell(1)(state.apply(tr), moved => tr.setSelection(moved.selection));
    dispatch(tr.scrollIntoView());
  })),
  'Shift-Tab': goToNextCell(-1),
  Backspace: chainCommands((state, dispatch, view) => deleteBlocks(state, dispatch, view, true), undoInputRule),
  Delete: deleteBlocks,
  Enter: enterBlock,
  'Mod-Enter': exitCode,
  'Mod-z': undo,
  'Shift-Mod-z': redo,
  'Mod-b': toggleMark(schema.marks.strong),
  'Mod-i': toggleMark(schema.marks.em),
});
const blockDecorations = new Plugin({ props: {
  decorations(state) {
    const { doc, selection } = state;
    if (!(selection instanceof BlockSelection)) return null;
    const nodes = [];
    doc.forEach((node, pos) => {
      if (pos >= selection.from && pos < selection.to) nodes.push(Decoration.node(pos, pos + node.nodeSize, { class: 'block-selected' }));
    });
    return DecorationSet.create(doc, nodes);
  },
} });
const rules = inputRules({ rules: [
  ...inlineRules,
  textblockTypeInputRule(/^(#{1,6}) $/, heading, match => ({ level: match[1].length })),
  textblockTypeInputRule(/^```([\w+-]*) $/, code_block, match => ({ params: match[1] })),
  wrappingInputRule(/^> $/, blockquote),
  wrappingInputRule(/^[-*+] $/, bullet_list),
  wrappingInputRule(/^(\d+)\. $/, ordered_list, match => ({ order: Number(match[1]) })),
] });
export function mountEditor(host, { markdown = '', onPickImage, clickArea } = {}) {
  host.classList.add('rich-editor');
  host.innerHTML = `
  <div class="workspace">
    <div id="block-tools" hidden>
      <button type="button" class="quick-add" data-add-block aria-label="아래에 문단 추가">+</button>
      <details>
        <summary aria-label="블록 메뉴">⠿</summary>
        <div class="block-menu">
          <button type="button" data-add-block>아래에 문단 추가</button>
          <button type="button" id="remove-block">블록 삭제</button>
        </div>
      </details>
    </div>
    <div id="editor"></div>
  </div>
<div id="insert-menu" role="listbox" aria-label="삽입 메뉴" hidden></div>
<dialog id="insert-dialog" aria-labelledby="insert-title">
  <form>
    <h2 id="insert-title"></h2>
    <label><span></span><input type="text" required autocomplete="off"></label>
    <label><span></span><input type="text" autocomplete="off"></label>
    <p id="insert-error" role="alert"></p>
    <div class="actions">
      <button type="button" data-cancel>취소</button>
      <button type="submit">적용</button>
    </div>
  </form>
</dialog>
`;
  let originalDocument = readMarkdown(markdown);
  const events = new AbortController();
  const plugins = [rules, history(), insertionPlugin(host, onPickImage), blockKeys, keymap(baseKeymap), blockDecorations, tableEditing()];
  const editor = host.querySelector('#editor');
  const workspace = editor.parentElement;
  const tools = host.querySelector('#block-tools');
  let toolPosition = null;
  let hoveredBlock = null;

  function updateTools(view) {
    const { selection, doc } = view.state;
    const position = hoveredBlock ?? blockStart(selection.$from);
    const node = position < doc.content.size ? view.nodeDOM(position) : null;
    tools.hidden = !node?.getBoundingClientRect;
    toolPosition = tools.hidden ? null : position;
    if (!tools.hidden) tools.style.top = node.getBoundingClientRect().top - editor.getBoundingClientRect().top + 'px';
  }

  const view = new EditorView(editor, {
    state: EditorState.create({ doc: originalDocument.doc, plugins }),
    nodeViews: { video: videoView },
    attributes: { role: 'textbox', 'aria-controls': 'insert-menu', 'aria-label': '본문', 'aria-multiline': 'true', 'aria-describedby': 'editor-shortcuts' },
    dispatchTransaction(tr) {
      if (tr.docChanged) hoveredBlock = null;
      this.updateState(this.state.apply(tr));
      updateTools(this);
    },
  });
  updateTools(view);
  const observer = new ResizeObserver(() => updateTools(view));
  observer.observe(editor);
  const { signal } = events;
  editor.addEventListener('pointermove', event => {
    let block = event.target;
    while (block && block.parentElement !== view.dom) block = block.parentElement;
    if (!block) return;
    hoveredBlock = blockStart(view.state.doc.resolve(view.posAtDOM(block, 0)));
    updateTools(view);
  }, { signal });
  workspace.addEventListener('pointerleave', () => {
    if (tools.querySelector('details').open) return;
    hoveredBlock = null;
    updateTools(view);
  }, { signal });

  function insertParagraph(position) {
    const { state } = view;
    const tr = closeHistory(state.tr).insert(position, paragraph.create());
    tr.setSelection(TextSelection.create(tr.doc, position + 1));
    view.dispatch(tr.scrollIntoView());
    view.focus();
  }

  function addAfterBlock() {
    if (toolPosition === null) return;
    tools.querySelector('details').open = false;
    insertParagraph(toolPosition + view.state.doc.nodeAt(toolPosition).nodeSize);
  }

  (clickArea || document).addEventListener('click', event => {
    if (view.composing || event.shiftKey || event.altKey || event.metaKey || event.ctrlKey) return;
    const backgrounds = [clickArea, host, workspace, editor, view.dom];
    if (!clickArea) backgrounds.push(document.documentElement, document.body, workspace.closest('main'));
    if (!backgrounds.includes(event.target)) return;
    const bounds = view.dom.getBoundingClientRect();
    if (event.clientY < bounds.top) return;
    const nativeSelection = window.getSelection();
    if (nativeSelection?.toString() && view.dom.contains(nativeSelection.anchorNode)) return;

    const { state } = view;
    const { doc } = state;
    const last = doc.lastChild;
    const lastBlock = view.nodeDOM(doc.content.size - last.nodeSize);
    if (event.clientY >= lastBlock.getBoundingClientRect().bottom) {
      if (last.type !== paragraph || last.content.size) {
        insertParagraph(doc.content.size);
        return;
      }
      view.dispatch(state.tr.setSelection(TextSelection.create(doc, doc.content.size - 1)));
    } else {
      const left = Math.max(bounds.left + 1, Math.min(event.clientX, bounds.right - 1));
      const hit = view.posAtCoords({ left, top: event.clientY });
      if (!hit) return;
      const $pos = doc.resolve(hit.pos);
      const selection = $pos.parent.inlineContent ? TextSelection.create(doc, hit.pos) : Selection.near($pos);
      view.dispatch(state.tr.setSelection(selection));
    }
    view.focus();
  }, { signal });

  tools.querySelectorAll('[data-add-block]').forEach(button => button.addEventListener('click', addAfterBlock, { signal }));
  tools.querySelector('#remove-block').addEventListener('click', () => {
    if (toolPosition === null) return;
    tools.querySelector('details').open = false;
    view.dispatch(view.state.tr.setSelection(new BlockSelection(view.state.doc, toolPosition)));
    deleteBlocks(view.state, view.dispatch, view);
    view.focus();
  }, { signal });
  return {
    getMarkdown: () => writeMarkdown(view.state.doc, originalDocument),
    setMarkdown(markdown) {
      const next = readMarkdown(markdown);
      view.updateState(EditorState.create({ doc: next.doc, plugins }));
      originalDocument = next;
      hoveredBlock = null;
      updateTools(view);
    },
    insertMarkdown(markdown) {
      const block = parseBlock(markdown);
      if (!block) return;
      const node = block.type === paragraph && block.childCount === 1 && block.firstChild.isInline
        ? block.firstChild : block;
      view.dispatch(closeHistory(view.state.tr).replaceSelectionWith(node, false).scrollIntoView());
      view.focus();
    },
    focus: () => view.focus(),
    destroy() {
      events.abort();
      observer.disconnect();
      view.destroy();
      host.replaceChildren();
    },
  };
}
