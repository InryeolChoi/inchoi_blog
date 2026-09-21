-- 009_notes.sql — 프로젝트 하면서 남기는 글감을 담는다.
--
-- # 왜 필요한가
--
-- 프로젝트를 진행하는 동안 남긴 메모(글감)를 나중에 AI가 정리해서 draft 글로
-- 만든다. 글감은 글이 아니다 — 분류도 없고 slug도 없고 본문 링크의 대상도
-- 아니라서 posts에 넣으면 그쪽 계약(카테고리·형제 순서·표지 판정)이 전부
-- "이건 예외"를 하나씩 더 들어야 한다.
--
-- # generated_post_id
--
-- AI가 초안을 만들면 그 결과 post를 가리켜 둔다. **초안은 반드시 status='draft'로
-- 만든다** — 최종 편집은 사람이 admin에서 한다는 것이 이 기능의 전제다.
-- ON DELETE SET NULL: 그 draft를 나중에 지워도 글감 자체는 남아야 다시 시도할 수 있다.
-- **created_at·updated_at은 TEXT가 아니라 TIMESTAMP다.** posts와 같은 이유다
-- (db.Open의 _time_format=sqlite 주석 참고) — 드라이버가 Go time.Time으로
-- Scan할 칸은 선언된 타입이 TIMESTAMP/DATETIME이어야 한다. TEXT로 두면
-- "unsupported Scan … into type *time.Time"으로 죽는다.
CREATE TABLE notes (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    title             TEXT NOT NULL,
    body              TEXT NOT NULL,
    -- open: 아직 초안을 안 만들었다. generated: 만들었다(재생성은 다시 open으로 돌리고 한다).
    status            TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'generated')),
    generated_post_id INTEGER REFERENCES posts(id) ON DELETE SET NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT (datetime('now')),
    updated_at        TIMESTAMP NOT NULL DEFAULT (datetime('now'))
);
