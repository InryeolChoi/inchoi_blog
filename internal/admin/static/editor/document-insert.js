import { Plugin, Selection, TextSelection } from 'prosemirror-state';
import { closeHistory } from 'prosemirror-history';
import { linkURL, schema, typedLinkURL, youtubeID } from './document-content.js';

const nodes = schema.nodes;
const commands = [
  { id: 'image', label: '사진 추가', keys: 'image photo picture 그림 이미지', form: [['사진 URL', 'https://…'], ['대체 텍스트', '사진 설명']] },
  { id: 'table', label: '표 추가', keys: 'table 테이블', build: () => nodes.table.create(null, [nodes.table_header, nodes.table_cell, nodes.table_cell].map(type => nodes.table_row.create(null, Array.from({ length: 3 }, () => type.createAndFill())))) },
  { id: 'link', label: '링크 추가', keys: 'link url 주소', form: [['링크 주소', 'example.com'], ['표시할 텍스트', '링크 이름']] },
  { id: 'video', label: '유튜브 동영상', keys: 'youtube video 영상 임베드', form: [['유튜브 URL', 'https://youtu.be/…']] },
  { id: 'inline_math', label: '인라인 수식', keys: 'inline math latex katex 문장', form: [['LaTeX 수식', '\\frac{a}{b}']] },
  { id: 'display_math', label: '수식 블록', keys: 'display math latex katex', form: [['LaTeX 수식', '\\sum_{i=1}^{n} i']] },
  { id: 'heading2', label: '제목 2', keys: 'heading h2 소제목', level: 2 },
  { id: 'heading3', label: '제목 3', keys: 'heading h3 소제목', level: 3 },
  { id: 'bullet_list', label: '글머리 목록', keys: 'bullet list 리스트' },
  { id: 'ordered_list', label: '번호 목록', keys: 'ordered list 리스트' },
  { id: 'blockquote', label: '인용', keys: 'quote blockquote' },
  { id: 'code_block', label: '코드 블록', keys: 'code fence' },
  { id: 'horizontal_rule', label: '구분선', keys: 'divider rule hr' },
];
const commandByID = id => commands.find(command => command.id === id);

function beforeCursor(state) {
  const { selection } = state;
  const { $from } = selection;
  if (!(selection instanceof TextSelection) || !selection.empty || $from.parent.type.spec.code || $from.marks().some(mark => mark.type.spec.code)) return null;
  return $from.parent.textBetween(0, $from.parentOffset, '', '\ufffc');
}

function rangeAtCursor(view) {
  const prefix = beforeCursor(view.state);
  const match = prefix?.match(/(?:^|\s)\/([^\s/\ufffc]{0,24})$/);
  if (!match) return null;
  const to = view.state.selection.from;
  return { from: to - match[1].length - 1, to, query: match[1].toLowerCase() };
}

function insertContent(view, range, command, node) {
  const tr = closeHistory(view.state.tr);
  if (node?.isInline) {
    tr.replaceWith(range.from, range.to, node);
    tr.setSelection(TextSelection.create(tr.doc, range.from + node.nodeSize)).removeStoredMark(schema.marks.link);
  } else if (range.replace) {
    tr.replaceWith(range.from, range.to, node);
    tr.setSelection(Selection.near(tr.doc.resolve(range.from + node.nodeSize), 1));
  } else {
    tr.delete(range.from, range.to);
    const $pos = tr.doc.resolve(range.from);
    const index = $pos.index(-1);
    if (command.level && $pos.node(-1).canReplaceWith(index, index + 1, nodes.heading)) {
      tr.setBlockType(range.from, range.from, nodes.heading, { level: command.level });
    }
    else {
      const block = node || nodes.heading.create({ level: command.level });
      const replace = $pos.depth === 1 && $pos.parent.type === nodes.paragraph && !$pos.parent.content.size;
      const pos = replace ? $pos.before(1) : $pos.after(1);
      if (replace) tr.replaceWith(pos, $pos.after(1), block);
      else tr.insert(pos, block);
      const end = pos + block.nodeSize;
      if (block.isAtom && end === tr.doc.content.size) tr.insert(end, nodes.paragraph.create());
      tr.setSelection(Selection.findFrom(tr.doc.resolve(block.isAtom ? end : pos), 1, true) || Selection.near(tr.doc.resolve(end), -1));
    }
  }
  view.dispatch(tr.scrollIntoView());
  view.focus();
}

function readForm(command, value, description) {
  if (command.id === 'inline_math' || command.id === 'display_math') {
    if (!value || value.includes('$')) throw new Error('달러 기호 없이 LaTeX 수식만 입력하세요.');
    return nodes[command.id].create({ tex: value });
  }
  const href = command.id === 'link' ? typedLinkURL(value) : linkURL(value, true);
  if (!href) throw new Error('올바른 주소를 입력하세요.');
  if (command.id === 'image') return nodes.image.create({ src: href, alt: description, show: true });
  if (command.id === 'video') {
    if (!youtubeID(href)) throw new Error('유튜브 동영상 주소를 입력하세요.');
    return nodes.video.create({ href });
  }
  return schema.text(description || href, [schema.marks.link.create({ href })]);
}

export function insertionPlugin(host, onPickImage) {
  const menu = host.querySelector('#insert-menu');
  const dialog = host.querySelector('#insert-dialog');
  const items = onPickImage
    ? [{ id: 'upload', label: '이미지 올리기', keys: 'image photo picture 사진 파일', pick: true }, ...commands]
    : commands;
  const form = dialog.querySelector('form');
  const fields = Array.from(form.querySelectorAll('label'));
  const error = dialog.querySelector('#insert-error');
  let editorView;
  let current;
  let dismissed;
  let active = 0;
  let matches = [];
  let pending;

  function hide() {
    menu.hidden = true;
    editorView.dom.removeAttribute('aria-activedescendant');
    current = null;
  }

  function positionMenu() {
    if (menu.hidden || !current) return;
    const caret = editorView.coordsAtPos(current.from);
    const viewport = window.visualViewport;
    const width = viewport?.width ?? window.innerWidth;
    const height = viewport?.height ?? window.innerHeight;
    const minLeft = (viewport?.offsetLeft ?? 0) + 8;
    const minTop = (viewport?.offsetTop ?? 0) + 8;
    const bottom = minTop + height - 16;
    menu.style.maxWidth = Math.max(0, width - 16) + 'px';
    menu.style.maxHeight = Math.min(300, height * 0.4) + 'px';
    const left = Math.max(minLeft, Math.min(caret.left, minLeft + width - 16 - menu.offsetWidth));
    const below = caret.bottom + 6;
    const top = below + menu.offsetHeight <= bottom ? below : caret.top - menu.offsetHeight - 6;
    menu.style.left = left + 'px';
    menu.style.top = Math.max(minTop, Math.min(top, bottom - menu.offsetHeight)) + 'px';
  }

  function highlight() {
    const options = menu.querySelectorAll('[role="option"]');
    options.forEach((option, index) => option.setAttribute('aria-selected', String(index === active)));
    const option = options[active];
    if (!option) {
      editorView.dom.removeAttribute('aria-activedescendant');
      return;
    }
    editorView.dom.setAttribute('aria-activedescendant', option.id);
    if (option.offsetTop < menu.scrollTop) menu.scrollTop = option.offsetTop;
    else if (option.offsetTop + option.offsetHeight > menu.scrollTop + menu.clientHeight) {
      menu.scrollTop = option.offsetTop + option.offsetHeight - menu.clientHeight;
    }
  }

  function update(view) {
    if (dialog.open || view.composing || !view.hasFocus()) { hide(); return; }
    const range = rangeAtCursor(view);
    if (!range) { dismissed = null; hide(); return; }
    const key = `${range.from}:${range.to}:${range.query}`;
    if (key === dismissed) { hide(); return; }
    if (key === current?.key) { positionMenu(); return; }
    current = { ...range, key };
    active = 0;
    matches = items.filter(command => (command.label + ' ' + command.keys).includes(range.query));
    menu.replaceChildren(...matches.map(command => {
      const button = document.createElement('button');
      button.type = 'button';
      button.tabIndex = -1;
      button.id = 'insert-' + command.id;
      button.dataset.command = command.id;
      button.setAttribute('role', 'option');
      button.textContent = command.label;
      return button;
    }));
    if (!matches.length) menu.textContent = '일치하는 항목이 없습니다.';
    menu.hidden = false;
    positionMenu();
    highlight();
  }

  function openForm(command, range, values = []) {
    const doc = editorView.state.doc;
    pending = { command, range, doc };
    hide();
    dialog.querySelector('h2').textContent = command.label;
    fields.forEach((label, index) => {
      const field = command.form[index];
      const input = label.querySelector('input');
      label.hidden = !field;
      input.disabled = !field;
      input.value = values[index] || '';
      if (!field) return;
      label.querySelector('span').textContent = field[0];
      input.placeholder = field[1];
    });
    error.textContent = '';
    dialog.showModal();
    fields[0].querySelector('input').focus();
  }

  function choose(command) {
    if (!current || !command) return;
    const range = current;
    dismissed = range.key;
    if (command.pick) {
      const doc = editorView.state.doc;
      hide();
      onPickImage(markdown => {
        const src = /^!\[\]\((\/img\/[a-f0-9]{64})\)$/.exec(markdown)?.[1];
        if (!src || editorView.isDestroyed || !editorView.state.doc.eq(doc) || host.hidden) return false;
        insertContent(editorView, range, command, schema.nodes.image.create({ src }));
        return true;
      });
    } else if (command.form) openForm(command, range);
    else {
      hide();
      insertContent(editorView, range, command, command.build?.() || nodes[command.id]?.createAndFill());
    }
  }

  function keyboard(view, event) {
    if (menu.hidden || view.composing || event.isComposing) return false;
    if (event.key === 'Escape') { dismissed = current?.key; hide(); return true; }
    if (!matches.length) return false;
    if (event.key === 'Enter') { choose(matches[active]); return true; }
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return false;
    const step = event.key === 'ArrowUp' ? -1 : 1;
    active = (active + step + matches.length) % matches.length;
    highlight();
    return true;
  }

  function editableMath(node, _view, getPos) {
    const dom = node.type.spec.toDOM(node);
    dom.setAttribute('role', 'button');
    dom.setAttribute('aria-label', '수식 편집');
    dom.tabIndex = 0;
    const edit = () => {
      const from = getPos();
      if (from === undefined) return;
      openForm(commandByID(node.type.name), { from, to: from + node.nodeSize, replace: true }, [node.attrs.tex]);
    };
    dom.addEventListener('click', edit);
    dom.addEventListener('keydown', event => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      edit();
    });
    return { dom, stopEvent: event => ['mousedown', 'click', 'keydown'].includes(event.type) };
  }

  menu.addEventListener('mousedown', event => event.preventDefault());
  menu.addEventListener('click', event => choose(items.find(command => command.id === event.target.closest('[data-command]')?.dataset.command)));
  form.addEventListener('submit', event => {
    event.preventDefault();
    if (!pending || !pending.doc.eq(editorView.state.doc)) { dialog.close(); return; }
    const [value, description] = fields.map(label => label.querySelector('input').value.trim());
    try {
      const node = readForm(pending.command, value, description);
      const { range, command } = pending;
      dialog.close();
      insertContent(editorView, range, command, node);
    } catch (failure) { error.textContent = failure.message; }
  });
  dialog.querySelector('[data-cancel]').addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', () => {
    if (dialog.open) return;
    pending = null;
    editorView.focus();
  });

  return new Plugin({
    props: {
      handleKeyDown: keyboard,
      nodeViews: { inline_math: editableMath, display_math: editableMath },
      handleTextInput(view, from, to, value) {
        if (value !== ')' || view.composing || !beforeCursor(view.state)?.endsWith('[](')) return false;
        view.dispatch(view.state.tr.insertText(value, from, to));
        openForm(commandByID('link'), { from: from - 3, to: from + 1 });
        return true;
      },
    },
    view(view) {
      editorView = view;
      const focus = () => update(view);
      const blur = () => hide();
      view.dom.addEventListener('focus', focus);
      view.dom.addEventListener('blur', blur);
      view.dom.addEventListener('compositionend', focus);
      window.addEventListener('scroll', positionMenu, true);
      window.addEventListener('resize', positionMenu);
      window.visualViewport?.addEventListener('resize', positionMenu);
      window.visualViewport?.addEventListener('scroll', positionMenu);
      return { update, destroy() {
        view.dom.removeEventListener('focus', focus);
        view.dom.removeEventListener('blur', blur);
        view.dom.removeEventListener('compositionend', focus);
        window.removeEventListener('scroll', positionMenu, true);
        window.removeEventListener('resize', positionMenu);
        window.visualViewport?.removeEventListener('resize', positionMenu);
        window.visualViewport?.removeEventListener('scroll', positionMenu);
      } };
    },
  });
}

export function videoView(node) {
  const dom = document.createElement('div');
  dom.className = 'video-block';
  dom.contentEditable = 'false';
  dom.dataset.videoUrl = node.attrs.href;
  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = '▶ 유튜브 동영상 재생';
  dom.append(button);
  button.addEventListener('click', () => {
    const id = youtubeID(node.attrs.href);
    if (!id) return;
    const player = document.createElement('iframe');
    const url = new URL('https://www.youtube-nocookie.com/embed/' + id);
    url.searchParams.set('autoplay', '1');
    const playlist = new URL(node.attrs.href).searchParams.get('list');
    if (playlist && /^[\w-]{6,64}$/.test(playlist)) url.searchParams.set('list', playlist);
    player.src = url.href;
    player.title = '유튜브 동영상';
    player.allow = 'autoplay; encrypted-media; picture-in-picture; fullscreen';
    player.allowFullscreen = true;
    player.referrerPolicy = 'strict-origin-when-cross-origin';
    button.replaceWith(player);
  });
  return { dom, ignoreMutation: () => true, stopEvent: event => event.target === button };
}
