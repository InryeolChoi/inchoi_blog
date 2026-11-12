import { Schema } from 'prosemirror-model';
import { TextSelection } from 'prosemirror-state';
import { InputRule } from 'prosemirror-inputrules';
import { MarkdownParser, MarkdownSerializer, defaultMarkdownParser, defaultMarkdownSerializer } from 'prosemirror-markdown';
import { tableNodes } from 'prosemirror-tables';
import MarkdownIt from 'markdown-it';
import './vendor/katex.js';

const katex = window.katex;

export function linkURL(input, externalOnly = false) {
  const value = input.trim();
  if (!value || /[\s<>\\\u0000-\u001f]/.test(value)) return null;
  if (!externalOnly && !/^[a-z][a-z\d+.-]*:|^\/\//i.test(value)) return value;
  try {
    const url = new URL(value);
    if (url.protocol === 'https:' || url.protocol === 'http:' || (!externalOnly && url.protocol === 'mailto:')) return value;
  } catch { /* Keep incomplete destinations as text. */ }
  return null;
}

export function typedLinkURL(input) {
  const value = input.trim().replace(/^<(.*)>$/, '$1')
    .replace(/[\s<>"`\\\u0000-\u001f]/g, character => encodeURIComponent(character));
  const domain = /^(?:[\p{L}\p{N}](?:[\p{L}\p{N}-]*[\p{L}\p{N}])?\.)+(?:[\p{L}]{2,63}|xn--[a-z0-9-]+)(?::\d{1,5})?(?:[/?#]|$)/iu;
  if (value.startsWith('//')) return linkURL('https:' + value, true);
  return domain.test(value) ? linkURL('https://' + value, true) : linkURL(value);
}

export function youtubeID(input) {
  const href = linkURL(input, true);
  if (!href) return null;
  const url = new URL(href);
  const host = url.hostname.toLowerCase();
  let id;
  if (host === 'youtu.be') id = url.pathname.replace(/^\/+|\/+$/g, '');
  else if (['youtube.com', 'www.youtube.com', 'm.youtube.com', 'youtube-nocookie.com', 'www.youtube-nocookie.com'].includes(host)) {
    id = url.pathname === '/watch' ? url.searchParams.get('v') : url.pathname.match(/^\/(?:embed|shorts)\/([^/]+)\/?$/)?.[1];
  }
  return id && /^[\w-]{6,64}$/.test(id) ? id : null;
}

function mathDOM(node, displayMode) {
  const dom = document.createElement(displayMode ? 'div' : 'span');
  dom.className = displayMode ? 'display-math' : 'inline-math';
  dom.dataset.tex = node.attrs.tex;
  dom.contentEditable = 'false';
  dom.title = '수식 편집';
  katex.render(node.attrs.tex, dom, {
    displayMode, output: 'mathml', trust: false, throwOnError: false, maxExpand: 1000, maxSize: 20,
  });
  return dom;
}

let specs = defaultMarkdownParser.schema.spec.nodes.append(tableNodes({ tableGroup: 'block', cellContent: 'paragraph' }));
specs = specs.addToEnd('inline_math', {
  inline: true, group: 'inline', atom: true,
  attrs: { tex: {} },
  parseDOM: [{ tag: 'span.inline-math[data-tex]', getAttrs: dom => ({ tex: dom.dataset.tex }) }],
  toDOM: node => mathDOM(node, false),
}).addToEnd('display_math', {
  group: 'block', atom: true, attrs: { tex: {} },
  parseDOM: [{ tag: 'div.display-math[data-tex]', getAttrs: dom => ({ tex: dom.dataset.tex }) }],
  toDOM: node => mathDOM(node, true),
}).addToEnd('video', {
  group: 'block', atom: true,
  attrs: { href: {} },
  parseDOM: [{ tag: 'div[data-video-url]', getAttrs(dom) {
    const href = dom.dataset.videoUrl;
    return youtubeID(href) ? { href } : false;
  } }],
  toDOM: node => ['div', { class: 'video-block', contenteditable: 'false', 'data-video-url': node.attrs.href },
    ['button', { type: 'button' }, '▶ 유튜브 동영상 재생']],
}).addToEnd('source_block', {
  group: 'block', atom: true,
  attrs: { raw: {} },
  toDOM: node => ['pre', { 'aria-label': '원문 보존 블록' }, ['code', node.attrs.raw]],
});
specs.forEach((name, spec) => {
  if (spec.group?.split(' ').includes('block')) {
    specs = specs.update(name, { ...spec, attrs: { ...spec.attrs, sourceId: { default: null } } });
  }
});
const image = specs.get('image');
specs = specs.update('image', {
  ...image,
  attrs: { ...image.attrs, show: { default: false } },
  toDOM(node) {
    const { src, alt, show } = node.attrs;
    if (/^\/img\/[a-f0-9]{64}$/.test(src) || (show && linkURL(src, true))) {
      return ['img', { src, alt, referrerpolicy: 'no-referrer' }];
    }
    return ['span', { contenteditable: 'false' }, '[이미지] ' + (alt || '')];
  },
});
let marks = defaultMarkdownParser.schema.spec.marks;
marks = marks.update('link', {
  ...marks.get('link'),
  parseDOM: [{ tag: 'a[href]', getAttrs(dom) {
    const href = linkURL(dom.getAttribute('href'));
    return href ? { href, title: dom.getAttribute('title') } : false;
  } }],
});
export const schema = new Schema({ nodes: specs, marks });

// Consume TeX before CommonMark can interpret its backslashes and underscores.
const markdown = MarkdownIt({ html: false });
markdown.inline.ruler.before('escape', 'math_inline', (state, silent) => {
  const before = state.src[state.pos - 1];
  if (state.src[state.pos] !== '$' || before === '$') return false;
  const match = /^\$((?:\\[^$\n]|[^$\\\n])+)\$(?!\$)/.exec(state.src.slice(state.pos, state.posMax));
  if (!match || !match[1].trim()) return false;
  if (!silent) state.push('math_inline', '', 0).content = match[1];
  state.pos += match[0].length;
  return true;
});
// Markdown-It puts inline content directly inside a cell. The editor uses a paragraph.
markdown.core.ruler.after('inline', 'table_cell_paragraph', state => {
  state.tokens = state.tokens.flatMap(token => {
    if (token.type === 'th_open' || token.type === 'td_open') {
      return [token, new state.Token('paragraph_open', 'p', 1)];
    }
    if (token.type === 'th_close' || token.type === 'td_close') {
      return [new state.Token('paragraph_close', 'p', -1), token];
    }
    return [token];
  });
});
const parser = new MarkdownParser(schema, markdown, {
  ...defaultMarkdownParser.tokens,
  math_inline: { node: 'inline_math', getAttrs: token => ({ tex: token.content }) },
  table: { block: 'table' },
  thead: { ignore: true },
  tbody: { ignore: true },
  tr: { block: 'table_row' },
  th: { block: 'table_header' },
  td: { block: 'table_cell' },
});

export function parseBlock(source) {
  const math = /^\$\$[ \t]*\r?\n([\s\S]*?)\r?\n\$\$[ \t]*$/.exec(source);
  if (math) return schema.nodes.display_math.create({ tex: math[1] });
  if (/<\/?[a-z!][^>]*>|\$\$|^\s*:::/im.test(source)) return null;
  const parsed = parser.parse(source);
  if (parsed.childCount !== 1) return null;
  const block = parsed.firstChild;
  if (block.type === schema.nodes.paragraph && block.childCount === 1 && block.firstChild.isText) {
    const href = schema.marks.link.isInSet(block.firstChild.marks)?.attrs.href;
    if (href && youtubeID(href)) return schema.nodes.video.create({ href });
  }
  return block;
}

const serializer = new MarkdownSerializer({
  ...defaultMarkdownSerializer.nodes,
  inline_math(state, node) { state.write('$' + node.attrs.tex + '$'); },
  display_math(state, node) { state.write('$$\n' + node.attrs.tex + '\n$$'); state.closeBlock(node); },
  video(state, node) {
    const link = schema.marks.link.create({ href: node.attrs.href });
    state.renderInline(schema.nodes.paragraph.create(null, schema.text('유튜브 동영상', [link])));
    state.closeBlock(node);
  },
  table(state, node) {
    const rows = [];
    node.forEach(row => {
      const cells = [];
      row.forEach(cell => {
        const doc = schema.topNodeType.create(null, cell.firstChild);
        cells.push(serializer.serialize(doc).replace(/\|/g, '\\|').replace(/\n/g, ' '));
      });
      rows.push('| ' + cells.join(' | ') + ' |');
    });
    rows.splice(1, 0, '| ' + Array(node.firstChild.childCount).fill('---').join(' | ') + ' |');
    state.write(rows.join('\n'));
    state.closeBlock(node);
  },
  source_block(state, node) { state.write(node.attrs.raw); state.closeBlock(node); },
}, defaultMarkdownSerializer.marks);
export const serializeBlock = node => serializer.serialize(schema.topNodeType.create(null, node));

function balancedLinkDestination(value) {
  let depth = 0;
  for (const [token] of value.matchAll(/\\.|[()]/g)) {
    if (token === '(') depth++;
    if (token === ')' && --depth < 0) return false;
  }
  return depth === 0;
}

export const inlineRules = [
  new InputRule(/(^|[^\\$])\$((?:\\[^$\n]|[^$\\\n])+)\$$/, (state, match, start, end) => {
    if (!match[2].trim()) return null;
    const from = start + match[1].length;
    const tr = state.tr.replaceWith(from, end, schema.nodes.inline_math.create({ tex: match[2] }));
    return tr.setSelection(TextSelection.create(tr.doc, from + 1));
  }, { inCodeMark: false }),
  new InputRule(/(^|[^\\])\[([^\[\]\n]+)\]\(([^\n]*)\)$/, (state, match, start, end) => {
    if (!balancedLinkDestination(match[3])) return null;
    const href = typedLinkURL(match[3].replace(/\\([()])/g, '$1'));
    if (!href) return null;
    const from = start + match[1].length;
    const link = schema.marks.link.create({ href });
    const tr = state.tr.replaceWith(from, end, schema.text(match[2], [link]));
    return tr.setSelection(TextSelection.create(tr.doc, from + match[2].length)).removeStoredMark(schema.marks.link);
  }, { inCodeMark: false }),
];
