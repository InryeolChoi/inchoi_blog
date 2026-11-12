import { mountEditor } from './editor.js';

const editor = mountEditor(document.querySelector('#editor-host'));
const status = document.querySelector('#status');
let lastImported = '';

const file = document.querySelector('#file');
const importButton = document.querySelector('#import');
const copy = document.querySelector('#copy');
importButton.addEventListener('click', () => file.click());
file.addEventListener('change', async () => {
  const input = file.files[0];
  file.value = '';
  if (!input) return;
  if (editor.getMarkdown() !== lastImported && !window.confirm('현재 작성 중인 글을 새 파일로 바꿀까요?')) return;
  try {
    const source = await input.text();
    editor.setMarkdown(source);
    lastImported = source;
    status.textContent = input.name + '을 열었습니다.';
    editor.focus();
  } catch (error) {
    status.textContent = '파일을 열 수 없습니다.';
    console.error(error);
  }
});
copy.addEventListener('click', async () => {
  const markdown = editor.getMarkdown();
  const fallback = document.querySelector('#copy-fallback');
  try {
    await navigator.clipboard.writeText(markdown);
    fallback.hidden = true;
    status.textContent = '마크다운을 복사했습니다.';
  } catch {
    fallback.hidden = false;
    const field = document.querySelector('#markdown');
    field.value = markdown;
    field.focus();
    field.select();
    status.textContent = '선택된 마크다운을 복사하세요.';
  }
});
importButton.disabled = copy.disabled = false;
status.textContent = '이 화면에서 쓴 글은 탭을 닫으면 사라집니다.';
