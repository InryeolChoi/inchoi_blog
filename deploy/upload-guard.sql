-- deploy/upload-db.sh가 인스턴스에서 돌린다. **한 줄이라도 나오면 올리지 않는다.**
--
-- main   = 지금 서버가 쓰고 있는 /var/lib/blog/blog.db (정본)
-- newdb  = 올리려는 로컬 파일
--
-- 묻는 것은 하나다: **이 파일로 덮으면 서버에만 있는 것이 사라지는가?**
-- admin 화면에서 글을 쓰기 시작한 뒤로 그 답이 "그렇다"인 순간이 온다.

.mode list
.headers off

-- ① 서버에만 있는 글이 통째로 사라진다.
--    source가 무엇이든 묻는다. 웹에서 쓴 글(native)이면 되찾을 길이 아예
--    없고, 이관해서 들어온 글이면 "그 이관을 여기서 안 돌렸다"는 뜻이다 —
--    둘 다 이 파일로 덮기 전에 사람이 봐야 한다.
select 'LOST  ' || slug || '  (' || source || ', ' || status || ', ' || updated_at || ')'
from main.posts
where slug not in (select slug from newdb.posts)
  -- curation.DropPosts에 있는 글만 의도한 삭제로 허용한다. native 글은
  -- notion_page_id가 NULL이라 이 예외를 통과할 수 없다.
  and (notion_page_id is null or notion_page_id not in (
    select notion_page_id from intentional_dropped_posts
  ));

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
where sha256 not in (select sha256 from newdb.images)
  and sha256 not in (select sha256 from intentional_dropped_images);

-- ④ 웹에서 지운 글이 **되살아난다.**
--
--    앞의 셋과 방향이 반대다. 저 셋은 "덮으면 잃는 것"을 묻는데, 이건
--    "덮으면 돌아오는 것"이다. admin에 지우기가 생기면서 열린 자리다
--    (2026-08-31).
--
--    **source가 native인 글만 본다.** 이관은 native 글을 절대 만들지
--    않으므로, 올릴 파일에만 있는 native 글은 **서버에서 지웠다는 뜻밖에
--    없다.** 그래서 오탐이 없다.
--
--    이관해서 들어온 글은 여기서 안 본다. 재이관이 되살리는 것이 이미
--    정해진 계약이고(지우기 화면도 그렇게 경고한다), 진짜로 빼려면
--    internal/curation의 DropPosts에 적어야 한다. 여기서 같이 막으면
--    멀쩡한 재이관마다 걸려서, 확인이 습관이 되고 습관은 곧 안 읽는 것이다.
--
--    **예전에는 이 자리가 `notion_page_id is null`이었다.** 그때는 출처가
--    노션과 웹 둘뿐이라 그 NULL이 곧 native였는데, GitHub 이관이 붙으면서
--    (2026-09-01) 그 글 16편이 전부 "서버에서 지운 글"로 잡혔다.
--    **출처가 하나 늘 때 이런 자리가 또 있는지 보라** — cmd/categorize도
--    같은 이유로 깨졌다.
select 'BACK  ' || slug || '  (' || status || ', 웹에서 쓴 글)'
from newdb.posts
where source = 'native'
  and slug not in (select slug from main.posts);
