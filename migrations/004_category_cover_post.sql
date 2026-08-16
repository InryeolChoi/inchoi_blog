-- 004_category_cover_post.sql — 카테고리의 표지 글을 가리키는 자리를 만든다.
--
-- 노션 최상위 페이지 19개는 그 아래 글들의 묶음을 소개하는 글이다. 지금은 다른 글과
-- 똑같이 category_id로만 붙어 있어서, 목록에서 이게 표지 글인지 알 수가 없다.
--
-- "original_path = title이면 표지 글"이라는 규칙으로 찾을 수는 있지만, 그건 선언이
-- 아니라 우연에 기댄 것이다. 웹에서 글을 쓰다 제목을 바꾸면 그 규칙이 깨진다.
-- 카테고리 쪽에 명시적으로 적어둔다.
--
-- 방향을 categories → posts로 잡은 이유: 카테고리당 표지 글은 하나다. posts 쪽에
-- 깃발을 두면 한 카테고리에 표지가 여럿 생기는 걸 스키마가 막지 못한다.
--
-- ON DELETE SET NULL: 표지로 쓰던 글을 지우는 걸 막을 이유는 없다. 표지만 없어진다.

ALTER TABLE categories ADD COLUMN cover_post_id INTEGER REFERENCES posts(id) ON DELETE SET NULL;

-- 한 글이 두 카테고리의 표지가 되면 어느 쪽이 맞는지 알 수 없다.
-- SQLite는 UNIQUE 인덱스에서 NULL을 중복으로 치지 않으므로, 표지가 없는 카테고리는
-- 얼마든지 있어도 된다.
CREATE UNIQUE INDEX idx_categories_cover_post_id ON categories(cover_post_id);
