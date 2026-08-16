-- 003_category_source_name.sql — 카테고리에 "원본 경로에서 온 이름"을 따로 기록한다.
--
-- 문제:
--   cmd/categorize는 posts.original_path에서 카테고리를 뽑아내고 slug로 식별한다.
--   그런데 사람이 카테고리 이름을 바꾸면(소프트스킬 → tooling) slug도 바뀌고,
--   categorize는 그 카테고리를 못 찾아서 옛 이름으로 새로 만들어버린다. 글도 새로
--   만든 쪽으로 옮겨간다. 실제로 그렇게 됐다.
--
-- 해결:
--   화면에 보이는 이름(name/slug)과 경로에서 온 이름(source_name)을 분리한다.
--   categorize는 source_name으로만 찾고, 사람은 name/slug를 마음대로 바꾼다.
--
-- source_name이 NULL인 카테고리는 경로에서 온 게 아니라 사람이 만든 것이다
-- (dev, cs-theory 같은 상위 분류). categorize는 그런 건 건드리지 않는다.
-- SQLite는 UNIQUE 인덱스에서 NULL을 중복으로 치지 않으므로 여럿 있어도 된다.

ALTER TABLE categories ADD COLUMN source_name TEXT;

CREATE UNIQUE INDEX idx_categories_source_name ON categories(source_name);

-- 지금 있는 카테고리는 전부 경로에서 만들어진 것이라 이름이 곧 경로의 이름이다.
-- 이름을 바꾸기 전에 채워둬야 연결이 유지된다.
UPDATE categories SET source_name = name WHERE source_name IS NULL;
