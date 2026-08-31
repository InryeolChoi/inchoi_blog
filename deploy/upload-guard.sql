-- deploy/upload-db.sh가 인스턴스에서 돌린다. **한 줄이라도 나오면 올리지 않는다.**
--
-- main   = 지금 서버가 쓰고 있는 /var/lib/blog/blog.db (정본)
-- newdb  = 올리려는 로컬 파일
--
-- 묻는 것은 하나다: **이 파일로 덮으면 서버에만 있는 것이 사라지는가?**
-- admin 화면에서 글을 쓰기 시작한 뒤로 그 답이 "그렇다"인 순간이 온다.

.mode list
.headers off

-- ① 웹에서 쓴 글이 통째로 사라진다.
--    notion_page_id가 NULL이면 이관이 아니라 사람이 여기서 쓴 글이다
--    (import는 언제나 노션 page id를 넣는다).
select 'LOST  ' || slug || '  (' || status || ', ' || updated_at || ')'
from main.posts
where notion_page_id is null
  and slug not in (select slug from newdb.posts);

-- ② 웹에서 고친 내용이 되돌아간다.
--    서버 쪽이 더 최근에 바뀌었다는 뜻이다. 로컬은 재이관할 때마다
--    updated_at이 올라가므로, 이 줄이 나온다는 건 그 뒤에 웹에서
--    손댔다는 것이다.
select 'STALE ' || m.slug || '  서버=' || m.updated_at || ' 올릴것=' || n.updated_at
from main.posts m
join newdb.posts n using (slug)
where m.updated_at > n.updated_at;

-- ③ 웹에서 올린 이미지가 사라진다.
select 'LOSTIMG ' || substr(sha256, 1, 12)
from main.images
where sha256 not in (select sha256 from newdb.images);
