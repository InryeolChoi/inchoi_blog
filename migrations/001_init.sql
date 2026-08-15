-- 001_init.sql — 초기 스키마.
--
-- 주의: 이미 적용된 마이그레이션 파일은 고치지 않는다. 러너가 체크섬을 기록해두고
-- 내용이 바뀌면 실행을 거부한다. 스키마를 바꿔야 하면 새 번호의 파일을 추가한다.

CREATE TABLE categories (
    id         INTEGER PRIMARY KEY,
    parent_id  INTEGER REFERENCES categories(id) ON DELETE RESTRICT,
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_categories_parent_id ON categories(parent_id);

-- 카테고리는 최대 2단계다. SQLite의 CHECK 제약에는 서브쿼리를 쓸 수 없어서
-- 트리거로 막는다: 부모가 이미 부모를 가지고 있으면(= 2단계면) 그 밑에 못 붙인다.
CREATE TRIGGER categories_max_depth_on_insert
BEFORE INSERT ON categories
WHEN NEW.parent_id IS NOT NULL
     AND (SELECT parent_id FROM categories WHERE id = NEW.parent_id) IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'categories: 최대 2단계까지만 허용');
END;

CREATE TRIGGER categories_max_depth_on_update
BEFORE UPDATE OF parent_id ON categories
WHEN NEW.parent_id IS NOT NULL
     AND (SELECT parent_id FROM categories WHERE id = NEW.parent_id) IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'categories: 최대 2단계까지만 허용');
END;

CREATE TABLE posts (
    id       INTEGER PRIMARY KEY,

    -- 영문 소문자 + 하이픈. 발행 후에는 바꾸지 않는다(외부 링크가 깨진다).
    -- 형식 검증은 애플리케이션에서 한다.
    slug     TEXT NOT NULL UNIQUE,
    title    TEXT NOT NULL,
    body     TEXT NOT NULL,  -- 마크다운 원문. 렌더링 결과가 아니라 원문이 정본이다.

    status   TEXT NOT NULL CHECK (status IN ('draft', 'published', 'unlisted')),

    -- 글 계층. NULL이면 최상위. 자식이 달린 글은 지울 수 없다.
    parent_id   INTEGER REFERENCES posts(id) ON DELETE RESTRICT,
    sort_order  INTEGER NOT NULL DEFAULT 0,  -- 형제간 순서

    -- 카테고리가 지워져도 글은 남는다. 분류만 사라진다.
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,

    -- 유입 경로: notion | github | native.
    -- CHECK을 걸지 않았다. 나중에 값이 늘어날 때(obsidian 등) SQLite는 CHECK 제약을
    -- ALTER로 못 고치고 테이블을 새로 만들어 옮겨야 하는데, 그건 이 프로젝트의
    -- "삭제하는 마이그레이션 금지" 원칙과 정면으로 부딪힌다. 검증은 애플리케이션에서 한다.
    source TEXT NOT NULL,

    -- 재이관 멱등 키. 같은 노션 페이지를 두 번 넣지 않기 위한 것.
    -- 노션에서 온 글이 아니면 NULL이고, SQLite는 NULL을 UNIQUE 중복으로 치지 않는다.
    notion_page_id TEXT UNIQUE,

    original_path       TEXT,       -- 노션 원본 경로 문자열 ("école 42 > pipex > ...")
    original_created_at TIMESTAMP,  -- 원본 작성일. 이관 시점이 아니라 원래 쓴 날.

    published_at TIMESTAMP,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL
);

-- slug와 notion_page_id는 UNIQUE 제약이 인덱스를 이미 만들어주므로 따로 만들지 않는다.
CREATE INDEX idx_posts_parent_id   ON posts(parent_id);
CREATE INDEX idx_posts_category_id ON posts(category_id);
CREATE INDEX idx_posts_status      ON posts(status);

CREATE TABLE tags (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE
);

CREATE TABLE post_tags (
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);

-- post_id는 복합 PK의 선두 컬럼이라 이미 인덱스가 있다. 태그로 글을 찾는 반대 방향만 추가.
CREATE INDEX idx_post_tags_tag_id ON post_tags(tag_id);

CREATE TABLE images (
    id     INTEGER PRIMARY KEY,
    sha256 TEXT NOT NULL UNIQUE,  -- 내용 해시. 같은 이미지는 한 번만 저장한다.
    data   BLOB NOT NULL,
    mime   TEXT NOT NULL,
    width  INTEGER,
    height INTEGER,
    original_url TEXT,  -- 노션 원본 URL. 만료되므로 참고용 기록일 뿐이다.
    caption      TEXT,
    created_at   TIMESTAMP NOT NULL
);
