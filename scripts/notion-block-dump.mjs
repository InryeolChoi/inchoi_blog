#!/usr/bin/env node
/**
 * notion-block-dump.mjs
 *
 * 일회용 조사 스크립트 — 이관 설계를 위한 노션 페이지 블록 원본 덤프.
 * 읽기 전용(노션 쪽 쓰기 없음). 이미지 바이너리만 로컬에 새로 저장함.
 *
 * notion-audit-raw.json (audit 스캔 결과)이 있으면 거기서 페이지 ID 목록을 재사용해서
 * search API를 다시 호출하지 않음. 없으면 새로 search + database query로 목록을 만듦.
 *
 * 페이지 하나당 파일 하나: dump/{page_id}.json
 *   {
 *     page: <GET /v1/pages/{id} 응답 그대로 — 제목/작성일/수정일/부모 다 포함>,
 *     blocks: [ <블록 API 응답 그대로> + has_children면 children 재귀 첨부
 *               + image 블록이면 image.local에 로컬 저장 정보 첨부 ]
 *   }
 *
 * 이미지: image 블록의 file/external URL은 만료되므로(특히 file.url은 임시 서명 URL)
 * 바이트를 그대로 받아 sha256으로 해시해서 dump/images/{sha256}.{ext}에 저장.
 * 같은 이미지가 여러 페이지에서 참조돼도 내용 기준으로 한 번만 저장됨(dedup).
 *
 * 재시작 가능: dump/{page_id}.json이 이미 있으면 그 페이지는 통째로 스킵.
 * 완성된 파일만 남도록 임시파일(.tmp)에 먼저 쓰고 성공 시에만 rename.
 *
 * 사용법:
 *   node --env-file=.env scripts/notion-block-dump.mjs
 */

import { writeFileSync, mkdirSync, existsSync, readFileSync, renameSync } from "node:fs";
import { dirname, join, extname } from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

const __dirname = dirname(fileURLToPath(import.meta.url));
const TOKEN = process.env.NOTION_TOKEN;
const NOTION_VERSION = "2022-06-28";
const API_BASE = "https://api.notion.com/v1";
const DUMP_DIR = join(__dirname, "dump");
const IMAGES_DIR = join(DUMP_DIR, "images");
const AUDIT_RAW_PATH = join(__dirname, "notion-audit-raw.json");

if (!TOKEN) {
  console.error("NOTION_TOKEN 환경변수가 없어. `node --env-file=.env scripts/notion-block-dump.mjs`로 실행해줘.");
  process.exit(1);
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---------- Notion API: rate limiter (순차 페이싱 + 재시도) ----------
let lastRequestAt = 0;
const MIN_INTERVAL_MS = 350;
let requestCount = 0;

async function notionFetch(path, options = {}) {
  const now = Date.now();
  const wait = Math.max(0, lastRequestAt + MIN_INTERVAL_MS - now);
  if (wait > 0) await sleep(wait);
  lastRequestAt = Date.now();

  for (let attempt = 0; attempt < 6; attempt++) {
    let res;
    try {
      res = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers: {
          Authorization: `Bearer ${TOKEN}`,
          "Notion-Version": NOTION_VERSION,
          "Content-Type": "application/json",
          ...(options.headers || {}),
        },
      });
    } catch (networkErr) {
      await sleep((attempt + 1) * 1000);
      continue;
    }
    requestCount++;

    if (res.status === 429) {
      const retryAfter = Number(res.headers.get("retry-after")) || attempt + 1;
      console.warn(`    [429] rate limited, ${retryAfter}s 대기 후 재시도 (${attempt + 1}/6)`);
      await sleep(retryAfter * 1000);
      continue;
    }
    if (res.status >= 500) {
      console.warn(`    [${res.status}] 서버 오류, 재시도 (${attempt + 1}/6)`);
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

// ---------- 이미지 다운로드 (Notion API가 아니라 S3/외부 호스트라 rate limit 별개) ----------
const CONTENT_TYPE_EXT = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/jpg": "jpg",
  "image/gif": "gif",
  "image/webp": "webp",
  "image/svg+xml": "svg",
  "image/bmp": "bmp",
  "image/tiff": "tiff",
  "image/heic": "heic",
  "image/x-icon": "ico",
};

function extFromUrl(url) {
  try {
    const u = new URL(url);
    const ext = extname(u.pathname).replace(".", "").toLowerCase();
    return ext || null;
  } catch {
    return null;
  }
}

const imageUrlCache = new Map(); // 같은 실행 중 동일 URL 중복 다운로드 방지
const imageStats = { attempted: 0, newFiles: 0, deduped: 0, failed: 0 };

async function downloadImage(url) {
  if (!url) return null;
  if (imageUrlCache.has(url)) return imageUrlCache.get(url);

  imageStats.attempted++;
  let lastErr;
  for (let attempt = 0; attempt < 4; attempt++) {
    try {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const buf = Buffer.from(await res.arrayBuffer());
      const sha256 = createHash("sha256").update(buf).digest("hex");
      const contentType = res.headers.get("content-type") || "";
      const ext = CONTENT_TYPE_EXT[contentType.split(";")[0].trim()] || extFromUrl(url) || "bin";
      const filename = `${sha256}.${ext}`;
      const filePath = join(IMAGES_DIR, filename);

      let isNew = false;
      if (!existsSync(filePath)) {
        const tmpPath = `${filePath}.tmp`;
        writeFileSync(tmpPath, buf);
        renameSync(tmpPath, filePath);
        isNew = true;
      }
      isNew ? imageStats.newFiles++ : imageStats.deduped++;

      const result = { sha256, file: `images/${filename}`, contentType, bytes: buf.length };
      imageUrlCache.set(url, result);
      return result;
    } catch (e) {
      lastErr = e;
      await sleep((attempt + 1) * 500);
    }
  }
  imageStats.failed++;
  const errResult = { error: lastErr?.message || "unknown error", sourceUrl: url };
  imageUrlCache.set(url, errResult);
  return errResult;
}

async function attachLocalImage(block) {
  if (block.type !== "image" || !block.image) return;
  const url = block.image.type === "file" ? block.image.file?.url : block.image.external?.url;
  block.image.local = await downloadImage(url);
}

// ---------- 블록 재귀 수집 ----------
async function fetchBlocksRecursive(blockId, visited) {
  if (visited.has(blockId)) return []; // 순환 방지
  visited.add(blockId);

  const children = await getAllBlockChildren(blockId);
  for (const block of children) {
    await attachLocalImage(block);
    if (block.has_children) {
      // child_page / child_database는 별개 페이지 파일로 처리되므로 여기서 재귀 안 함
      if (block.type !== "child_page" && block.type !== "child_database") {
        block.children = await fetchBlocksRecursive(block.id, visited);
      }
    }
  }
  return children;
}

// ---------- 페이지 목록 확보 ----------
async function getPageIds() {
  // --pages-file <경로>: 줄바꿈으로 구분된 page id 목록만 덤프한다.
  // 나중에 접근 권한이 열린 페이지들만 골라 받을 때 쓴다. search를 다시 돌리지 않고,
  // 기존 덤프 파일은 아래 메인 루프의 existsSync 검사가 그대로 건너뛴다.
  const flagIdx = process.argv.indexOf("--pages-file");
  if (flagIdx !== -1) {
    const listPath = process.argv[flagIdx + 1];
    if (!listPath) {
      console.error("--pages-file 뒤에 목록 파일 경로가 필요해.");
      process.exit(1);
    }
    const ids = readFileSync(listPath, "utf-8")
      .split("\n")
      .map((s) => s.trim())
      .filter(Boolean);
    console.log(`${listPath}에서 페이지 ${ids.length}개 로드 (search 호출 안 함)`);
    return ids;
  }

  if (existsSync(AUDIT_RAW_PATH)) {
    const raw = JSON.parse(readFileSync(AUDIT_RAW_PATH, "utf-8"));
    const pageIds = raw.objects.filter((o) => o.object === "page").map((o) => o.id);
    console.log(`기존 ${AUDIT_RAW_PATH}에서 페이지 ${pageIds.length}개 로드 (search 재호출 안 함)`);
    return pageIds;
  }

  console.log("notion-audit-raw.json 없음 → search API로 새로 조회");
  const searchResults = await collectPaginated("/search", {});
  const ids = new Map();
  const databases = [];
  for (const obj of searchResults) {
    if (obj.object === "page") ids.set(obj.id, true);
    if (obj.object === "database") databases.push(obj.id);
  }
  for (const dbId of databases) {
    try {
      const rows = await collectPaginated(`/databases/${dbId}/query`, {});
      for (const row of rows) ids.set(row.id, true);
    } catch (e) {
      console.warn(`  [경고] 데이터베이스 ${dbId} 쿼리 실패: ${e.message}`);
    }
  }
  const pageIds = [...ids.keys()];
  console.log(`  -> 페이지 ${pageIds.length}개 발견`);
  return pageIds;
}

// ---------- 메인 ----------
async function main() {
  const startedAt = Date.now();
  mkdirSync(IMAGES_DIR, { recursive: true });

  const pageIds = await getPageIds();
  console.log(`총 ${pageIds.length}개 페이지 덤프 시작 (dump/{page_id}.json, 이미지는 dump/images/{sha256}.ext)`);

  let dumped = 0;
  let skipped = 0;
  let failed = 0;
  const errors = [];

  for (let i = 0; i < pageIds.length; i++) {
    const pageId = pageIds[i];
    const outPath = join(DUMP_DIR, `${pageId}.json`);

    if (existsSync(outPath)) {
      skipped++;
      continue;
    }

    try {
      const pageMeta = await notionFetch(`/pages/${pageId}`, { method: "GET" });
      const blocks = await fetchBlocksRecursive(pageId, new Set());
      const dump = { page: pageMeta, blocks };

      const tmpPath = `${outPath}.tmp`;
      writeFileSync(tmpPath, JSON.stringify(dump, null, 2), "utf-8");
      renameSync(tmpPath, outPath); // 원자적 커밋: 완성된 파일만 outPath에 존재
      dumped++;
    } catch (e) {
      failed++;
      errors.push({ pageId, message: e.message });
      console.warn(`  [실패] ${pageId}: ${e.message}`);
    }

    const processed = i + 1;
    if (processed % 20 === 0 || processed === pageIds.length) {
      console.log(
        `  ... ${processed}/${pageIds.length} (덤프 ${dumped}, 건너뜀 ${skipped}, 실패 ${failed}, ` +
          `API 요청 ${requestCount}회, 이미지 신규 ${imageStats.newFiles}/중복생략 ${imageStats.deduped}/실패 ${imageStats.failed})`
      );
    }
  }

  if (errors.length > 0) {
    writeFileSync(join(DUMP_DIR, "_errors.json"), JSON.stringify(errors, null, 2), "utf-8");
  }

  const elapsedSec = ((Date.now() - startedAt) / 1000).toFixed(1);
  console.log("\n" + "=".repeat(60));
  console.log("블록 덤프 완료");
  console.log("=".repeat(60));
  console.log(`전체 대상: ${pageIds.length}`);
  console.log(`새로 덤프: ${dumped}`);
  console.log(`이미 있어서 건너뜀: ${skipped}`);
  console.log(`실패: ${failed}${failed > 0 ? ` (자세한 내용: ${join(DUMP_DIR, "_errors.json")}, 재실행하면 실패한 것만 재시도됨)` : ""}`);
  console.log(
    `이미지: 시도 ${imageStats.attempted}, 신규 저장 ${imageStats.newFiles}, 중복(생략) ${imageStats.deduped}, 실패 ${imageStats.failed}`
  );
  console.log(`API 요청 ${requestCount}회, 소요 시간 ${elapsedSec}초`);
  console.log(`저장 위치: ${DUMP_DIR}`);
  console.log("=".repeat(60));
}

main().catch((e) => {
  console.error("\n[치명적 실패]", e);
  process.exit(1);
});
