# 열렬히.뛰기

**https://inquieto.dev** — 기술 블로그 겸 학습 아카이브.

노션에 흩어져 있던 학습 노트 1,400여 편을 하나의 사이트로 옮긴 것이다.
개발·컴퓨터과학·데이터와 수학을 오가며 쌓은 기록이고, 글은 웹 화면에서
직접 쓰고 고친다.

## 무엇으로 만들었나

**Go 바이너리 하나와 SQLite 파일 하나다.** 프론트엔드 프레임워크도, npm도,
빌드 스텝도 없다. 템플릿과 정적 자산은 `embed.FS`로 바이너리에 박혀 있고,
CSS와 JS는 손으로 쓴 것을 그대로 서빙한다.

```
cmd/blog/          서버. 시작할 때 마이그레이션을 적용하고 HTTP를 연다
internal/web/      읽기 전용 공개 페이지
internal/admin/    글쓰기 화면 — GitHub 로그인, -admin을 줘야 뜬다
internal/markdown/ 본문 마크다운 → HTML (goldmark + 수식·코드 라벨 확장)
internal/notion/   노션 덤프 파싱과 마크다운 변환기
internal/curation/ 사람이 정한 예외. 분류·삭제·본문 수정이 전부 코드에 있다
migrations/        번호순 SQL
deploy/            GCP 배포물 — systemd 유닛, Caddy, 백업, DB 동기화
```

직접 의존성은 둘뿐이다: `modernc.org/sqlite`(cgo 없는 SQLite 드라이버)와
`github.com/yuin/goldmark`(CommonMark 렌더러).

## 어떻게 돼 있나

- **DB가 정본이다.** 마크다운 파일을 빌드해서 배포하는 정적 사이트 생성기가
  아니다. 정본은 서버의 SQLite 파일이고, 로컬은 이관 파이프라인을 돌리는
  작업본이다.
- **사람이 정한 결정은 전부 `internal/curation`에 있다.** DB를 손으로 고치지
  않는다 — 표에 적으면 몇 번을 다시 이관해도 같은 결과가 나온다.
- **수식은 KaTeX, 코드는 highlight.js.** 서버가 수식과 코드를 미리 골라
  표시해 두고, 브라우저는 그 표시가 있는 페이지에서만 CDN을 받는다.
  CDN이 막혀도 원문 LaTeX와 색 없는 코드가 그대로 읽힌다.
- **글 화면에서 바로 고친다.** 로그인이 확인되면 글 제목 옆에 고치기가
  나오고, 본문이 있던 자리가 편집기가 된다. `/`를 치면 수식·표·코드·
  애니메이션 조각을 꽂는 팔레트가 뜬다.
- **애니메이션은 본문에 이름만 둔다** (`:::anim sort-bubble`). 본문에 임의
  JS를 담는 길은 열지 않는다 — 실제 스크립트는 저장소에 사람이 쓴 파일로 있다.

## 돌려보기

```sh
go test ./...
go run ./cmd/blog -db blog.db -addr 127.0.0.1:8080
```

`blog.db`는 저장소에 없다(gitignore). 만드는 방법과 운영 절차는 각 문서에 있다.

## 문서

| | |
|---|---|
| [`CLAUDE.md`](CLAUDE.md) | **이 프로젝트의 진짜 문서다.** 구조·결정·이력 전부 |
| [`deploy/README.md`](deploy/README.md) | GCP 배포, HTTPS, 백업, DB 동기화 절차 |

`AGENTS.md`는 `CLAUDE.md`를 가리키는 심볼릭 링크다 — 어느 도구로 열어도
같은 문서를 읽는다.
