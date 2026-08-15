#!/usr/bin/env node
/**
 * notion-page-status.mjs
 *
 * 일회용 조사 스크립트 — scripts/dump/*.json(블록 원본 덤프)을 분석해서
 * 1311개 전 페이지의 이관 후 초기 status를 결정. 읽기 전용, 노션 API 호출 없음,
 * 변환/DB 작업 없음. (전체 이관이 전제이므로 "제외 대상 판별"이 아니라
 * "무엇을 draft로 깔고 시작할지" 판별용.)
 *
 * 입력:
 *   - scripts/dump/{page_id}.json  (notion-block-dump.mjs 결과물)
 *   - scripts/notion-audit-raw.json (notion-workspace-audit.mjs 결과물,
 *     부모 체인에 등장하는 database/workspace 등 비-page 조상의 제목을 알기 위해 사용)
 *
 * 페이지별 CSV 컬럼:
 *   page_id, title, top_level_ancestor, full_path, parent_type,
 *   block_count, text_length, has_image, has_code, has_equation,
 *   created_time, last_edited_time, is_db_row, is_stub, is_untitled, status
 *
 * status 규칙 (전 페이지 이관 전제, published는 나중에 수동 지정):
 *   - draft    : 블록 5개 미만(stub) 이거나 제목이 비어있음/untitled
 *   - unlisted : 그 외 전부 (parent가 database인 것도 포함 — 인라인 데이터베이스로
 *                목차/글을 관리한 노트가 섞여 있어서 부모 타입만으로는 제외 불가)
 *
 * 출력: 콘솔 집계 요약 + scripts/notion-page-status.csv (전체 목록)
 */

import { readFileSync, readdirSync, writeFileSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const DUMP_DIR = join(__dirname, "dump");
const AUDIT_RAW_PATH = join(__dirname, "notion-audit-raw.json");
const CSV_OUT_PATH = join(__dirname, "notion-page-status.csv");

if (!existsSync(DUMP_DIR)) {
  console.error(`덤프 디렉토리가 없어: ${DUMP_DIR} (notion-block-dump.mjs 먼저 실행해줘)`);
  process.exit(1);
}
if (!existsSync(AUDIT_RAW_PATH)) {
  console.error(`${AUDIT_RAW_PATH}가 없어 (notion-workspace-audit.mjs 먼저 실행해줘)`);
  process.exit(1);
}

// ---------- 계층 정보 로드 (부모 체인 타이틀 조회용) ----------
const auditRaw = JSON.parse(readFileSync(AUDIT_RAW_PATH, "utf-8"));
const objectsById = new Map();
for (const obj of auditRaw.objects) {
  objectsById.set(obj.id, obj);
}

// 페이지의 parent.type이 "block_id"인 경우(토글/컬럼/동기화 블록 등 안에 바로 중첩된 페이지)
// search 결과(objectsById)에는 그 블록 자체가 없어서 부모 체인이 거기서 끊김.
// dump/ 블록 트리에 그 블록이 어느 페이지 안에 있는지 정보가 있으므로,
// blockId -> 그 블록을 담고 있는 페이지 id 매핑을 만들어 보정한다.
function effectiveParentId(obj, blockOwnerMap) {
  if (obj.parentType === "workspace" || !obj.parentId) return null;
  if (obj.parentType === "block") {
    const ownerPageId = blockOwnerMap.get(obj.parentId);
    if (ownerPageId && ownerPageId !== obj.id) return ownerPageId;
    return null; // 그 블록이 어느 덤프에도 없음(권한 밖 등) -> 더 못 올라감
  }
  return obj.parentId; // page_id / database_id 그대로
}

function resolveAncestorChain(id, blockOwnerMap) {
  // [최상위 ... 자기 자신] 순서로 {id, title} 배열 반환. 부모 접근 불가하면 거기서 끊음.
  const chain = [];
  const seen = new Set();
  let current = objectsById.get(id);
  if (!current) return chain;
  chain.unshift({ id: current.id, title: current.title });
  seen.add(current.id);

  let guard = 0;
  while (current && guard++ < 500) {
    const parentId = effectiveParentId(current, blockOwnerMap);
    if (!parentId || seen.has(parentId)) break;
    const parent = objectsById.get(parentId);
    if (!parent) break; // 접근 불가한 조상 -> 여기서 체인 끊김
    chain.unshift({ id: parent.id, title: parent.title });
    seen.add(parent.id);
    current = parent;
  }
  return chain;
}

// ---------- 블록 트리 집계 (블록 수 / 텍스트 길이 / 이미지·코드·수식 포함 여부) ----------
// 동시에 blockId -> 이 블록을 담고 있는 페이지 id 도 색인한다 (부모체인 보정용).
function walkBlocksForStats(blocks, stats, pageId, blockOwnerMap) {
  for (const block of blocks) {
    if (block.id) blockOwnerMap.set(block.id, pageId);
    stats.blockCount++;
    if (block.type === "image") stats.hasImage = true;
    if (block.type === "code") stats.hasCode = true;
    if (block.type === "equation") stats.hasEquation = true;
    accumulateText(block, stats);
    if (Array.isArray(block.children) && block.children.length > 0) {
      walkBlocksForStats(block.children, stats, pageId, blockOwnerMap);
    }
  }
}

function accumulateText(node, stats) {
  if (!node || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const item of node) accumulateText(item, stats);
    return;
  }
  if (typeof node.plain_text === "string") {
    stats.textLength += node.plain_text.length;
  }
  for (const key of Object.keys(node)) {
    if (key === "children") continue; // children은 walkBlocksForStats에서 별도 처리(블록 수 포함해서)
    accumulateText(node[key], stats);
  }
}

// ---------- CSV ----------
function csvEscape(value) {
  const s = value === null || value === undefined ? "" : String(value);
  if (/[",\n]/.test(s)) return `"${s.replace(/"/g, '""')}"`;
  return s;
}

const CSV_COLUMNS = [
  "page_id",
  "title",
  "top_level_ancestor",
  "full_path",
  "parent_type",
  "block_count",
  "text_length",
  "has_image",
  "has_code",
  "has_equation",
  "created_time",
  "last_edited_time",
  "is_db_row",
  "is_stub",
  "is_untitled",
  "status",
];

// ---------- 메인 ----------
function main() {
  const files = readdirSync(DUMP_DIR).filter((f) => f.endsWith(".json") && f !== "_errors.json");
  console.log(`덤프 파일 ${files.length}개 분석 중...`);

  // 1차 패스: 파일 파싱 + 블록 통계 + blockId->page 소유 색인 (부모체인 보정용)
  const blockOwnerMap = new Map();
  const entries = [];
  for (const file of files) {
    const pageId = file.replace(/\.json$/, "");
    let data;
    try {
      data = JSON.parse(readFileSync(join(DUMP_DIR, file), "utf-8"));
    } catch (e) {
      console.warn(`  [경고] ${file} 파싱 실패: ${e.message}`);
      continue;
    }
    const pageObj = data.page || {};
    const stats = { blockCount: 0, textLength: 0, hasImage: false, hasCode: false, hasEquation: false };
    walkBlocksForStats(data.blocks || [], stats, pageId, blockOwnerMap);
    entries.push({ pageId, pageObj, stats });
  }

  // 2차 패스: 완성된 blockOwnerMap으로 부모체인(최상위 조상/전체 경로) 해석
  const rows = [];
  for (const { pageId, pageObj, stats } of entries) {
    const auditEntry = objectsById.get(pageId);
    const title = auditEntry?.title ?? "(제목 없음)";
    const parentType = auditEntry?.parentType ?? "unknown";
    const chain = resolveAncestorChain(pageId, blockOwnerMap);
    const topLevelAncestor = chain.length > 0 ? chain[0].title : title;
    const fullPath = chain.length > 0 ? chain.map((c) => c.title).join(" > ") : title;

    const isDbRow = parentType === "database"; // 정보용 플래그. status 판단에는 안 씀.
    const isStub = stats.blockCount < 5;
    const isUntitled = !title || title.trim() === "" || title === "(제목 없음)";
    const status = isStub || isUntitled ? "draft" : "unlisted";

    rows.push({
      page_id: pageId,
      title,
      top_level_ancestor: topLevelAncestor,
      full_path: fullPath,
      parent_type: parentType,
      block_count: stats.blockCount,
      text_length: stats.textLength,
      has_image: stats.hasImage,
      has_code: stats.hasCode,
      has_equation: stats.hasEquation,
      created_time: pageObj.created_time ?? auditEntry?.created_time ?? "",
      last_edited_time: pageObj.last_edited_time ?? auditEntry?.last_edited_time ?? "",
      is_db_row: isDbRow,
      is_stub: isStub,
      is_untitled: isUntitled,
      status,
    });
  }

  // ---------- CSV 저장 ----------
  const csvLines = [CSV_COLUMNS.join(",")];
  for (const r of rows) {
    csvLines.push(CSV_COLUMNS.map((c) => csvEscape(r[c])).join(","));
  }
  writeFileSync(CSV_OUT_PATH, csvLines.join("\n") + "\n", "utf-8");

  // ---------- 집계 ----------
  const total = rows.length;
  const dbRowCount = rows.filter((r) => r.is_db_row).length;
  const dbRowNonStubCount = rows.filter((r) => r.is_db_row && !r.is_stub).length;
  const stubCount = rows.filter((r) => r.is_stub).length;
  const untitledCount = rows.filter((r) => r.is_untitled).length;
  const draftRows = rows.filter((r) => r.status === "draft");
  const unlistedRows = rows.filter((r) => r.status === "unlisted");

  const ancestorDist = {};
  for (const r of unlistedRows) {
    ancestorDist[r.top_level_ancestor] = (ancestorDist[r.top_level_ancestor] || 0) + 1;
  }

  const buckets = [
    [0, 100],
    [100, 500],
    [500, 1000],
    [1000, 2000],
    [2000, 5000],
    [5000, 10000],
    [10000, Infinity],
  ];
  const bucketLabel = ([lo, hi]) => (hi === Infinity ? `${lo}+` : `${lo}-${hi}`);
  const textLenHist = buckets.map(([lo, hi]) => ({
    label: bucketLabel([lo, hi]),
    count: unlistedRows.filter((r) => r.text_length >= lo && r.text_length < hi).length,
  }));

  console.log("\n" + "=".repeat(60));
  console.log("초기 status 분류 요약 (전 페이지 이관 전제)");
  console.log("=".repeat(60));
  console.log(`전체 페이지: ${total}`);
  console.log(`부모가 database인 페이지(정보용, status 판단엔 미사용): ${dbRowCount}`);
  console.log(`  - 그 중 블록 5개 이상(실제 글로 추정): ${dbRowNonStubCount}`);
  console.log(`블록 5개 미만인 페이지(stub): ${stubCount}`);
  console.log(`제목 없음("(제목 없음)" 포함, untitled): ${untitledCount}`);
  console.log(`\nstatus = draft (stub 또는 untitled): ${draftRows.length} / ${total}`);
  console.log(`status = unlisted (그 외 전부): ${unlistedRows.length} / ${total}`);
  console.log(`(published는 이 스크립트가 정하지 않음 — 나중에 수동 지정)`);

  console.log(`\n[unlisted 페이지의 최상위 조상별 분포]`);
  for (const [ancestor, count] of Object.entries(ancestorDist).sort((a, b) => b[1] - a[1])) {
    console.log(`  ${ancestor}: ${count}`);
  }

  console.log(`\n[unlisted 페이지의 텍스트 길이 분포 (문자 수)]`);
  const maxCount = Math.max(...textLenHist.map((b) => b.count), 1);
  for (const b of textLenHist) {
    const barLen = Math.round((b.count / maxCount) * 40);
    console.log(`  ${b.label.padStart(9)}: ${"#".repeat(barLen)} ${b.count}`);
  }

  console.log("\n" + "=".repeat(60));
  console.log(`CSV 저장됨: ${CSV_OUT_PATH} (${rows.length}행)`);
  console.log("=".repeat(60));
}

main();
