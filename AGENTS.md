# blog — 공통 작업 지침

개인 기술 블로그와 노션·GitHub 학습 노트 아카이브다.
이 파일이 Claude Code와 Codex의 공통 지침이다. `CLAUDE.md`는 `AGENTS.md`를
가리키는 심볼릭 링크다. 공통 규칙을 두 파일에 복사하지 않는다.

## 작업에 앞서 읽을 문서

상세 문서는 필요한 작업에 맞춰 읽는다. 문서 전체를 매번 읽을 필요는 없다.

| 작업 | 먼저 읽을 문서와 절 |
|---|---|
| 구조·스키마·정렬·글 계층 | [architecture.md](docs/architecture.md)의 구조, 형제 순서, 글 계층, 마이그레이션 |
| 공개 화면·디자인·접근 제어·마크다운 | [architecture.md](docs/architecture.md)의 공개 서버, 레이아웃, 렌더링 관련 절 |
| Admin·글 편집·저장·인증 | [architecture.md](docs/architecture.md)의 Admin 화면과 편집기 후속 규칙 |
| 노션/GitHub 이관·카테고리 재구성·본문 정리 | [operations.md](docs/operations.md)의 이관 순서, 카테고리, 사람이 정한 예외 |
| 배포·DB 백업·동기화 | [operations.md](docs/operations.md)의 운영 원칙과 [deploy/README.md](deploy/README.md)의 실행 절차 |
| 다음 작업 선택·AI 삽입·번역 | [operations.md](docs/operations.md)의 남은 작업과 번역 결정 |

## 프로젝트 전제

- **정본은 서버의 `/var/lib/blog/blog.db`다.** 로컬 `blog.db`는 이관 작업본이다.
  웹 UI에서 글을 쓰고 고친다. 본문은 DB에 저장한 마크다운이며 블록 JSON으로 바꾸지 않는다.
  `content/*.md`·프론트매터·빌드 시 콘텐츠 렌더링을 전제한 정적 사이트 생성기가 아니다.
- **Go 단일 바이너리 + SQLite.** 공개 페이지는 `html/template` SSR,
  Admin은 별도 패키지의 CSR이다. 템플릿·정적 자산은 `embed.FS`에 포함한다.
- **프론트엔드 프레임워크와 자산 빌드 단계 없음.** npm/webpack/vite/tailwind CLI를
  도입하지 않는다. `scripts/`의 Node는 이관 도구이며 런타임과 무관하다.
- 새 의존성은 필요한 이유를 명시한다. 표준 라이브러리로 가능한 일은 그것으로 한다.

## 데이터와 운영 규칙

- **`scripts/dump/`는 수정·삭제하지 않는다.** 청소 목적으로도 건드리지 않는다.
  파생물은 다른 경로에 만든다. 새 페이지 수집 절차는 operations.md를 따른다.
- 이관 데이터의 삭제·이동·본문 보정 등 사람이 정한 예외는 `internal/curation`에 적는다.
  DB만 고치면 재이관이 되돌릴 수 있다. 웹 편집도 이관 대상이면 같은 주의가 필요하다.
  현재 운영 DB에 반영됐는지와 예외 표에 기록됐는지는 별도로 확인한다.
- 이관·분류 도구를 변경하면 재실행 시 수렴하는지 확인한다. categorize와 regroup은
  새 분류를 만들 때 두 바퀴가 필요할 수 있으며, 도구 실행 순서를 바꿔도 최종 결과가 같아야 한다.
- **CI는 서버 DB를 덮지 않는다.** 평소에는 `deploy/fetch-db.sh`로 서버에서 받는다.
  로컬 이관 결과 업로드는 `deploy/upload-db.sh`를 사용하고 가드를 우회하지 않는다.
- WAL이 남은 DB 본체만 복사하거나 파일 해시만으로 동일성을 판정하지 않는다.
  편집·쓰기 검증에는 일관된 DB 사본을 사용한다. 서버 시작도 마이그레이션을 적용한다.
- 글은 기본적으로 status로 가린다. 이관 제외는 이유를 적은 curation 예외로 관리한다.
  Admin 삭제는 자식·표지·들어오는 링크·rev 확인 등 기존 계약을 유지한다.
- 웹에서 만든 글은 `source = 'native'`, `notion_page_id = NULL`이다.
  GitHub 글도 notion ID가 NULL이므로 NULL만으로 native라고 판단하지 않는다.

## 스키마와 SQLite

- 스키마 변경은 번호가 붙은 SQL 마이그레이션으로만 한다. 운영 DB에 직접 ALTER하지 않는다.
- 적용된 마이그레이션은 고치지 않는다. down 마이그레이션도 만들지 않는다.
- 컬럼·테이블을 삭제하는 마이그레이션은 만들지 않는다. 이름 변경은 새 컬럼과 백필로 처리한다.
- DSN의 `_time_format=sqlite`와 `foreign_keys` 설정을 유지한다.
- 테스트 DB는 `t.TempDir()` 아래 파일로 만든다. 풀의 커넥션마다 분리되는 `:memory:`를 쓰지 않는다.

## 공개 화면과 편집

- `draft`는 기본 공개 목록·카운트·본문 링크·글 상세에서 숨긴다. `unlisted`는 보이는
  아카이브 글이다. `visibility`는 별도 접근 권한 축이며 `-drafts`로 private가 열리지 않는다.
- 공개 조회의 접근 제어는 store에 모은다. 로그인 여부만으로 draft 규칙을 바꾸지 않는다.
  이미지 URL은 글 권한과 별도라는 현재 한계도 유지·개선 작업 시 확인한다.
- Admin은 허용된 계정만 접근한다. 옵션·허용 목록·설정이 빠졌을 때 열리는 방향으로 바꾸지 않는다.
  `-admin-no-auth`는 loopback 전용이며 운영에 `-drafts`를 넣지 않는다.
- 미리보기는 공개 렌더러·CSS·자산 정의를 공유한다. 저장의 rev 충돌 검사, 트랜잭션,
  slug 변경 시 링크 갱신, CSRF 검사를 유지한다.
- 본문에 임의 JS를 넣지 않는다. 애니메이션은 `:::anim 이름`과 등록된 컴포넌트로 구현한다.
  렌더러가 raw HTML을 허용하므로 이것이 HTML 전반의 안전성을 보장하는 것은 아니다.
- **HTML → CSS → 필요한 경우 순수 JS** 순서로 구현한다. 의미 있는 요소와 기본 동작을 먼저 쓴다.
  JS가 없어도 콘텐츠 탐색은 가능해야 한다. 동작에는 `prefers-reduced-motion`을 적용한다.
- 디자인은 잉크 괘선과 색 블록을 사용한다. 현재 토큰을 따르고 가독성·포커스·모바일 폭을 지킨다.
  코드·표·수식의 넘침은 요소 안에서 처리한다. 터치 대상과 입력 글자 크기도 확인한다.

## 기본 확인

```sh
git log --oneline -8
go test ./...
CGO_ENABLED=0 go build -o /tmp/blog-check ./cmd/blog
```

서버를 눈으로 확인할 때는 operations.md의 DB 사본 절차를 먼저 따른다.
테스트가 실패한 상태에서 완료라고 말하지 않는다. 실패가 남으면 사실과 출력을 보고한다.
화면 변경은 관련 화면을 좁은 폭·라이트/다크에서도 확인하고, 검사기가 실제 실패를 잡는지 확인한다.
이 문서 정리처럼 실행 코드가 바뀌지 않는 작업은 링크·구조·차이 검증을 우선한다.

## 문서 유지 방법

- 공통 규칙은 이 파일, 현재 구조·동작은 architecture.md, 운영·이관·남은 일은 operations.md에 기록한다.
- 기존 설명을 최신 상태로 고친다. 완료 일지나 대화 내용을 누적하지 않는다.
- 설계 이유와 재발 방지 조건은 담당 절에 남긴다. 폐기된 상태·중복 검증 기록은 반복하지 않는다.
- 데이터 건수는 실시간 사실로 복사하지 않는다. 필요하면 확인 날짜·DB 출처와 함께 기록한다.
- 완료한 할 일은 목록에서 제거하고 필요한 동작 설명을 해당 절에 합친다. 과거 변경은 Git 이력에서 찾는다.
- 상세 문서를 추가할 때는 가능하면 기존 두 문서의 적절한 절을 사용한다.

## Agent skills

### 이슈 추적

이슈는 이 저장소의 GitHub Issues에 있고 `gh` CLI로 다룬다. `docs/agents/issue-tracker.md`를 본다.

### triage 라벨

다섯 개 표준 라벨을 기본 이름 그대로 쓴다. `docs/agents/triage-labels.md`를 본다.

### 도메인 문서

단일 컨텍스트(루트 `CONTEXT.md` + `docs/adr/`)다. `docs/agents/domain.md`를 본다.
