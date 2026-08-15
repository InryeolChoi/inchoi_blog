#!/usr/bin/env node
/**
 * notion-workspace-audit.mjs
 *
 * 일회용 조사 스크립트 — 이관 설계를 위한 노션 워크스페이스 실물 파악용.
 * 읽기 전용. DB/스키마를 만들거나 워크스페이스에 쓰기 작업을 하지 않음.
 *
 * 사용법:
 *   NOTION_TOKEN=secret_xxx node scripts/notion-workspace-audit.mjs
 * 또는 .env에 NOTION_TOKEN이 있으면:
 *   node --env-file=.env scripts/notion-workspace-audit.mjs
 *
 * 세는 것:
 *  - 접근 가능한 전체 페이지 수
 *  - 페이지 계층 깊이 분포
 *  - 최상위 페이지 목록 + 하위 페이지 수
 *  - 블록 타입별 등장 횟수(전체 집계)
 *  - 이미지 블록 총 개수
 *  - database 타입 개수/목록
 *  - 내부 링크(mention, link_to_page) 총 개수
 *  - 제목 중복 페이지
 *
 * 출력: 콘솔 요약 + ./scripts/notion-audit-raw.json
 */

import { writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const TOKEN = process.env.NOTION_TOKEN;
const NOTION_VERSION = "2022-06-28";
const API_BASE = "https://api.notion.com/v1";
const OUT_PATH = join(__dirname, "notion-audit-raw.json");

if (!TOKEN) {
  console.error("NOTION_TOKEN 환경변수가 없어. `.env`에 있다면 `node --env-file=.env scripts/notion-workspace-audit.mjs`로 실행해줘.");
  process.exit(1);
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---------- 아주 단순한 rate limiter (약 3req/s로 순차 페이싱) ----------
let lastRequestAt = 0;
const MIN_INTERVAL_MS = 350;
let requestCount = 0;

async function notionFetch(path, options = {}) {
  const now = Date.now();
  const wait = Math.max(0, lastRequestAt + MIN_INTERVAL_MS - now);
  if (wait > 0) await sleep(wait);
  lastRequestAt = Date.now();

  for (let attempt = 0; attempt < 6; attempt++) {
    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: {
        Authorization: `Bearer ${TOKEN}`,
        "Notion-Version": NOTION_VERSION,
        "Content-Type": "application/json",
        ...(options.headers || {}),
      },
    });
    requestCount++;

    if (res.status === 429) {
      const retryAfter = Number(res.headers.get("retry-after")) || attempt + 1;
      console.warn(`  [429] rate limited, ${retryAfter}s 대기 후 재시도 (${attempt + 1}/6)`);
      await sleep(retryAfter * 1000);
      continue;
    }
    if (res.status >= 500) {
      await sleep((attempt + 1) * 1000);
      continue;
    }
    if (!res.ok) {
      const body = await res.text().catch(() => "");
      throw new Error(`Notion API ${res.status} ${path}: ${body.slice(0, 300)}`);
    }
    return res.json();
  }
  throw new Error(`재시도 초과: ${path}`);
}

async function collectPaginated(path, extraBody = {}) {
  let cursor;
  const all = [];
  do {
    const body = { ...extraBody, page_size: 100 };
    if (cursor) body.start_cursor = cursor;
    const data = await notionFetch(path, { method: "POST", body: JSON.stringify(body) });
    all.push(...(data.results || []));
    cursor = data.has_more ? data.next_cursor : undefined;
  } while (cursor);
  return all;
}

async function getAllBlockChildren(blockId) {
  let cursor;
  const all = [];
  do {
    const qs = new URLSearchParams({ page_size: "100", ...(cursor ? { start_cursor: cursor } : {}) });
    const data = await notionFetch(`/blocks/${blockId}/children?${qs}`, { method: "GET" });
    all.push(...(data.results || []));
    cursor = data.has_more ? data.next_cursor : undefined;
  } while (cursor);
  return all;
}

// ---------- 파싱 헬퍼 ----------
function extractPageTitle(pageObj) {
  const props = pageObj.properties || {};
  for (const key of Object.keys(props)) {
    const p = props[key];
    if (p.type === "title") {
      const t = (p.title || []).map((t) => t.plain_text).join("");
      return t || "(제목 없음)";
    }
  }
  return "(제목 없음)";
}

function extractDbTitle(dbObj) {
  const t = (dbObj.title || []).map((t) => t.plain_text).join("");
  return t || "(제목 없음)";
}

function parseParent(obj) {
  const p = obj.parent || {};
  switch (p.type) {
    case "workspace":
      return { parentType: "workspace", parentId: null };
    case "page_id":
      return { parentType: "page", parentId: p.page_id };
    case "database_id":
      return { parentType: "database", parentId: p.database_id };
    case "block_id":
      return { parentType: "block", parentId: p.block_id };
    default:
      return { parentType: "unknown", parentId: null };
  }
}

function scanMentions(obj, pageStats) {
  if (!obj || typeof obj !== "object") return;
  if (Array.isArray(obj)) {
    for (const item of obj) scanMentions(item, pageStats);
    return;
  }
  if (obj.type === "mention" && obj.mention && obj.mention.type) {
    const mtype = obj.mention.type;
    pageStats.mentionCounts[mtype] = (pageStats.mentionCounts[mtype] || 0) + 1;
  }
  for (const key of Object.keys(obj)) {
    scanMentions(obj[key], pageStats);
  }
}

async function walkBlocks(blockId, pageStats, visited) {
  if (visited.has(blockId)) return;
  visited.add(blockId);

  let children;
  try {
    children = await getAllBlockChildren(blockId);
  } catch (e) {
    pageStats.errors.push(`${blockId}: ${e.message}`);
    return;
  }

  for (const block of children) {
    pageStats.blockCountTotal++;
    pageStats.blockTypeCounts[block.type] = (pageStats.blockTypeCounts[block.type] || 0) + 1;
    if (block.type === "image") pageStats.imageCount++;
    if (block.type === "link_to_page") pageStats.linkToPageCount++;
    scanMentions(block, pageStats);

    // child_page / child_database는 별개의 page 객체로 이미 순회 대상이므로 재귀 안 함
    if (block.has_children && block.type !== "child_page" && block.type !== "child_database") {
      await walkBlocks(block.id, pageStats, visited);
    }
  }
}

function computeDepth(id, objectsById, memo, stack = new Set()) {
  if (memo.has(id)) return memo.get(id);
  const obj = objectsById.get(id);
  if (!obj) return 0;
  if (obj.parentType === "workspace" || obj.parentType === "unknown" || obj.parentType === "block") {
    memo.set(id, 0);
    return 0;
  }
  const parentId = obj.parentId;
  if (!parentId || !objectsById.has(parentId) || stack.has(id)) {
    memo.set(id, 0);
    return 0;
  }
  stack.add(id);
  const d = 1 + computeDepth(parentId, objectsById, memo, stack);
  stack.delete(id);
  memo.set(id, d);
  return d;
}

function countDescendantPages(id, childrenMap, objectsById, visited = new Set()) {
  let count = 0;
  for (const childId of childrenMap[id] || []) {
    if (visited.has(childId)) continue;
    visited.add(childId);
    const child = objectsById.get(childId);
    if (child && child.object === "page") count++;
    count += countDescendantPages(childId, childrenMap, objectsById, visited);
  }
  return count;
}

// ---------- 메인 ----------
async function main() {
  const startedAt = Date.now();
  console.log("1/4 워크스페이스 검색 중 (search API)...");

  const searchResults = await collectPaginated("/search", {});
  const objectsById = new Map();

  for (const obj of searchResults) {
    const { parentType, parentId } = parseParent(obj);
    objectsById.set(obj.id, {
      id: obj.id,
      object: obj.object, // 'page' | 'database'
      title: obj.object === "database" ? extractDbTitle(obj) : extractPageTitle(obj),
      url: obj.url,
      archived: !!obj.archived,
      in_trash: !!obj.in_trash,
      created_time: obj.created_time,
      last_edited_time: obj.last_edited_time,
      parentType,
      parentId,
      discoveredVia: "search",
    });
  }

  console.log(`  -> search로 ${objectsById.size}개 객체 발견`);
  console.log("2/4 데이터베이스 행(row) 페이지 보강 조회 중...");

  const databases = [...objectsById.values()].filter((o) => o.object === "database");
  let discoveredRows = 0;
  for (const db of databases) {
    try {
      const rows = await collectPaginated(`/databases/${db.id}/query`, {});
      for (const row of rows) {
        if (!objectsById.has(row.id)) {
          const { parentType, parentId } = parseParent(row);
          objectsById.set(row.id, {
            id: row.id,
            object: "page",
            title: extractPageTitle(row),
            url: row.url,
            archived: !!row.archived,
            in_trash: !!row.in_trash,
            created_time: row.created_time,
            last_edited_time: row.last_edited_time,
            parentType,
            parentId,
            discoveredVia: "database_query",
          });
          discoveredRows++;
        }
      }
    } catch (e) {
      console.warn(`  [경고] 데이터베이스 ${db.id} 쿼리 실패: ${e.message}`);
    }
  }
  console.log(`  -> 추가로 ${discoveredRows}개 행 페이지 발견 (총 ${objectsById.size}개 객체)`);

  // 계층 관계
  const childrenMap = {};
  for (const obj of objectsById.values()) {
    if (obj.parentId && objectsById.has(obj.parentId)) {
      (childrenMap[obj.parentId] ||= []).push(obj.id);
    }
  }
  const depthMemo = new Map();
  for (const obj of objectsById.values()) {
    obj.depth = computeDepth(obj.id, objectsById, depthMemo);
  }

  // 3/4: 블록 순회 (전체 페이지 콘텐츠)
  const pages = [...objectsById.values()].filter((o) => o.object === "page");
  console.log(`3/4 페이지 ${pages.length}개 블록 콘텐츠 순회 중... (시간 좀 걸림)`);

  const pageContents = {};
  const globalBlockTypeCounts = {};
  const globalMentionCounts = {};
  let globalImageCount = 0;
  let globalLinkToPageCount = 0;
  let processed = 0;

  for (const page of pages) {
    const pageStats = {
      blockTypeCounts: {},
      mentionCounts: {},
      imageCount: 0,
      linkToPageCount: 0,
      blockCountTotal: 0,
      errors: [],
    };
    await walkBlocks(page.id, pageStats, new Set());
    pageContents[page.id] = pageStats;

    for (const [type, n] of Object.entries(pageStats.blockTypeCounts)) {
      globalBlockTypeCounts[type] = (globalBlockTypeCounts[type] || 0) + n;
    }
    for (const [mtype, n] of Object.entries(pageStats.mentionCounts)) {
      globalMentionCounts[mtype] = (globalMentionCounts[mtype] || 0) + n;
    }
    globalImageCount += pageStats.imageCount;
    globalLinkToPageCount += pageStats.linkToPageCount;

    processed++;
    if (processed % 20 === 0 || processed === pages.length) {
      console.log(`  ... ${processed}/${pages.length} (API 요청 ${requestCount}회)`);
    }
  }

  console.log("4/4 집계 중...");

  // 깊이 분포
  const depthDistribution = {};
  for (const p of pages) {
    depthDistribution[p.depth] = (depthDistribution[p.depth] || 0) + 1;
  }

  // 최상위 페이지 + 하위 페이지 수
  const topLevelPages = pages
    .filter((p) => p.parentType === "workspace")
    .map((p) => ({
      id: p.id,
      title: p.title,
      url: p.url,
      descendantPageCount: countDescendantPages(p.id, childrenMap, objectsById),
    }))
    .sort((a, b) => b.descendantPageCount - a.descendantPageCount);

  // 중복 제목
  const titleGroups = new Map();
  for (const p of pages) {
    const key = p.title.trim();
    if (!titleGroups.has(key)) titleGroups.set(key, []);
    titleGroups.get(key).push({ id: p.id, url: p.url });
  }
  const duplicateTitles = [...titleGroups.entries()]
    .filter(([, list]) => list.length > 1)
    .map(([title, list]) => ({ title, count: list.length, pages: list }))
    .sort((a, b) => b.count - a.count);

  // 내부 링크 총합 = link_to_page 블록 + mention(page/database)
  const internalLinkTotal =
    globalLinkToPageCount + (globalMentionCounts.page || 0) + (globalMentionCounts.database || 0);

  const dbList = databases.map((d) => ({ id: d.id, title: d.title, url: d.url }));

  const elapsedSec = ((Date.now() - startedAt) / 1000).toFixed(1);

  const aggregate = {
    totalPages: pages.length,
    totalDatabases: databases.length,
    depthDistribution,
    topLevelPages,
    globalBlockTypeCounts,
    imageBlockTotal: globalImageCount,
    databaseList: dbList,
    mentionCounts: globalMentionCounts,
    linkToPageBlockTotal: globalLinkToPageCount,
    internalLinkTotal,
    duplicateTitles,
    requestCount,
    elapsedSec: Number(elapsedSec),
  };

  const raw = {
    meta: {
      generatedAt: new Date().toISOString(),
      notionVersion: NOTION_VERSION,
      requestCount,
      elapsedSec: Number(elapsedSec),
    },
    objects: [...objectsById.values()],
    pageContents,
    aggregate,
  };

  mkdirSync(dirname(OUT_PATH), { recursive: true });
  writeFileSync(OUT_PATH, JSON.stringify(raw, null, 2), "utf-8");

  // ---------- 콘솔 요약 ----------
  console.log("\n" + "=".repeat(60));
  console.log("노션 워크스페이스 조사 요약");
  console.log("=".repeat(60));
  console.log(`전체 페이지 수: ${aggregate.totalPages}`);
  console.log(`전체 데이터베이스 수: ${aggregate.totalDatabases}`);

  console.log("\n[페이지 계층 깊이 분포] (0 = 최상위)");
  for (const depth of Object.keys(depthDistribution).sort((a, b) => a - b)) {
    console.log(`  depth ${depth}: ${depthDistribution[depth]}개`);
  }

  console.log(`\n[최상위 페이지 목록] (${topLevelPages.length}개)`);
  for (const tp of topLevelPages) {
    console.log(`  - ${tp.title}  (하위 페이지 ${tp.descendantPageCount}개)  ${tp.url}`);
  }

  console.log("\n[블록 타입별 등장 횟수 (전체)]");
  for (const [type, n] of Object.entries(globalBlockTypeCounts).sort((a, b) => b[1] - a[1])) {
    console.log(`  ${type}: ${n}`);
  }

  console.log(`\n이미지 블록 총 개수: ${globalImageCount}`);

  console.log(`\n[데이터베이스 목록] (${dbList.length}개)`);
  for (const db of dbList) {
    console.log(`  - ${db.title}  ${db.url}`);
  }

  console.log(`\n[내부 링크]`);
  console.log(`  link_to_page 블록: ${globalLinkToPageCount}`);
  console.log(`  mention(page): ${globalMentionCounts.page || 0}`);
  console.log(`  mention(database): ${globalMentionCounts.database || 0}`);
  console.log(`  mention(기타: user/date 등): ${Object.entries(globalMentionCounts)
    .filter(([k]) => k !== "page" && k !== "database")
    .map(([k, v]) => `${k}=${v}`)
    .join(", ") || "없음"}`);
  console.log(`  내부 링크 총합(link_to_page + mention page/database): ${internalLinkTotal}`);

  console.log(`\n[제목 중복 페이지] (${duplicateTitles.length}개 제목이 중복됨)`);
  if (duplicateTitles.length === 0) {
    console.log("  없음");
  } else {
    for (const d of duplicateTitles.slice(0, 30)) {
      console.log(`  - "${d.title}" x${d.count}`);
    }
    if (duplicateTitles.length > 30) console.log(`  ... 외 ${duplicateTitles.length - 30}개 더`);
  }

  console.log("\n" + "=".repeat(60));
  console.log(`API 요청 ${requestCount}회, 소요 시간 ${elapsedSec}초`);
  console.log(`raw 데이터 저장됨: ${OUT_PATH}`);
  console.log("=".repeat(60));
}

main().catch((e) => {
  console.error("\n[실패]", e);
  process.exit(1);
});
