# blog

개인 기술 블로그 + 노션/깃헙에 흩어져 있던 학습 노트 아카이브를 하나로 합친 것.

**DB가 정본이다.** 글은 웹 UI에서 직접 쓰고 고친다. 마크다운 파일을 정본으로 두고
빌드해서 배포하는 정적 사이트 생성기가 아니다. 파일 기반 워크플로를 전제한 제안
(콘텐츠를 `content/*.md`로 빼기, 프론트매터 파싱, 빌드 시 렌더링 등)은 이 프로젝트의
전제와 어긋난다.

## 스택

- **Go 단일 바이너리** — `html/template`로 서버사이드 렌더링, `embed.FS`로 템플릿과
  정적 자산을 바이너리에 포함. 배포물은 바이너리 + SQLite 파일.
- **SQLite** — 애플리케이션 DB.
- **프론트엔드 프레임워크 없음, 빌드 스텝 없음.** npm/webpack/vite/tailwind CLI 등
  자산 빌드 파이프라인을 도입하지 않는다. CSS와 JS는 손으로 쓴 것을 그대로 서빙한다.
  (`scripts/`의 노션 이관 스크립트만 Node를 쓴다. 이건 일회용 도구고 런타임과 무관하다.)

## 구조

```
cmd/blog/       서버. 시작할 때 마이그레이션을 적용하고 HTTP를 연다.
cmd/import/     노션 이관 CLI. 아직 뼈대만 있고 변환/INSERT는 미구현.
internal/db/    연결(Open)과 마이그레이션 러너(Migrate)
migrations/     번호순 SQL. 001_init.sql 형식.
embed.go        루트 패키지. migrations/를 embed해서 MigrationsFS()로 노출.
scripts/        노션 이관용 일회용 Node 스크립트 (런타임과 무관)
  notion-workspace-audit.mjs   워크스페이스 구조 조사 → notion-audit-raw.json
  notion-block-dump.mjs        페이지 블록 원본 덤프 → dump/
  notion-page-status.mjs       덤프 분석해서 초기 status 결정 → notion-page-status.csv
  dump/                        (gitignore) 페이지 JSON 1311개 + images/ 437개, 124MB
.env                           (gitignore) NOTION_TOKEN
```

`embed.go`가 루트에 있는 이유: `//go:embed`는 상위 디렉토리를 못 가리킨다
(`../migrations` 불가). `migrations/`를 루트에 두려면 embed 선언도 루트여야 한다.

```sh
go test ./...                                  # 전체 테스트
go run ./cmd/blog -db blog.db -addr :8080      # 서버
CGO_ENABLED=0 go build -o blog ./cmd/blog      # 단일 바이너리
```

## 마이그레이션

- 파일명은 `001_init.sql` 형식. 번호는 사전순이 아니라 **숫자순**으로 적용된다.
- **down 마이그레이션은 만들지 않는다.** 러너가 `*.down.sql`을 발견하면 거부한다.
  되돌려야 하면 되돌리는 내용의 새 마이그레이션을 추가한다.
- **이미 적용된 마이그레이션 파일은 고치지 않는다.** 러너가 sha256을 기록해두고
  내용이 바뀌면 실행을 거부한다 (DB의 실제 스키마와 파일이 갈라지는 걸 막기 위해).
- 각 마이그레이션은 트랜잭션 안에서 돌고, 실패하면 DDL과 적용 기록이 함께 롤백된다.

## SQLite에서 조심할 것

- **`_time_format=sqlite`를 DSN에서 빼지 마라.** 이게 없으면 드라이버가 `time.Time`을
  Go의 `String()` 형식으로 저장하고, SQLite의 `date()`/`datetime()`/`strftime()`이
  그걸 파싱하지 못해 **에러 없이 NULL**을 돌려준다. 발행일 기준 쿼리가 조용히 빈 결과를
  낸다. `internal/db/time_test.go`가 이걸 지키고 있다.
- **`foreign_keys`는 SQLite 기본값이 OFF다.** 커넥션 단위 설정이라 DSN으로 건다.
  `Open`이 실제로 켜졌는지 확인하고 아니면 에러를 낸다.
- 테스트에서 `:memory:`를 쓰지 마라. 커넥션 풀이 커넥션마다 별개의 인메모리 DB를
  만들어서 마이그레이션을 건 DB와 조회하는 DB가 달라진다. `t.TempDir()`에 파일로 만든다.
- **CHECK 제약은 나중에 못 고친다.** SQLite는 CHECK을 ALTER로 수정할 수 없어서
  테이블을 새로 만들어 옮겨야 하는데, 그건 아래 "삭제 금지" 원칙과 부딪힌다.
  값이 늘어날 수 있는 컬럼(`posts.source` 등)에는 CHECK을 걸지 않고 애플리케이션에서
  검증한다. `posts.status`는 세 값으로 확정이라 CHECK을 걸었다.

## 데이터 현황

- 노션 페이지 **1311개** 덤프 완료. 이미지 **437개** 로컬 저장 완료
  (`scripts/dump/images/{sha256}.{ext}`, 내용 해시 기준 dedup).
- **전부 이관 대상이다.** 일부만 골라 옮기는 게 아니다. 공개 여부는 삭제가 아니라
  `status` 컬럼으로 가린다.
- `notion-page-status.csv`가 계산해둔 초기 status: `draft` 365개(블록 5개 미만인
  stub이거나 제목 없음), `unlisted` 918개(그 외 전부). `published`는 나중에 수동 지정한다.
- CSV의 제목 필드에 콤마가 들어간 행이 있다. 단순 `cut -d,`로 파싱하면 컬럼이 밀린다.

## 지켜야 할 것

**스키마**
- 스키마 변경은 **반드시 마이그레이션 파일로** 한다. 돌아가는 DB에 직접 `ALTER TABLE`을
  치지 않는다.
- **컬럼이나 테이블을 삭제하는 마이그레이션은 만들지 않는다.** 안 쓰게 된 컬럼은
  쓰지 않는 상태로 남겨둔다. 이름을 바꿔야 하면 새 컬럼을 추가하고 백필한다.

**`scripts/dump/` 는 수정하거나 삭제하지 않는다**
- 재수집에 **43분** 걸린다. 노션 API가 느리고 이미지를 전부 다시 받아야 한다.
- 읽기만 한다. 이관 코드가 이 디렉토리에 쓰지 않게 한다. 정리/청소 명목으로도
  건드리지 않는다. 파생물이 필요하면 다른 경로에 새로 만든다.

**작업 완료 기준**
- 테스트가 실패하는 상태에서 완료라고 말하지 않는다. 깨진 테스트가 남았으면
  그 사실과 출력을 그대로 보고한다.

**의존성**
- 새 의존성을 추가할 땐 **왜 필요한지 명시한다.** 표준 라이브러리로 되는 일이면
  표준 라이브러리로 한다. 특히 프론트엔드 쪽 의존성은 "빌드 스텝 없음" 원칙과
  충돌하는지 먼저 따진다.
- 현재 직접 의존성은 하나뿐이다:
  - `modernc.org/sqlite` — SQLite 드라이버. cgo 없이 순수 Go로 빌드돼서
    `CGO_ENABLED=0`으로 정적 단일 바이너리와 크로스 컴파일이 된다.
    `mattn/go-sqlite3`는 cgo가 필요해서 쓰지 않는다.
  - 마이그레이션 러너는 외부 라이브러리(golang-migrate 등) 없이 `internal/db`에
    직접 구현했다. 이 프로젝트의 규칙(down 금지, 적용된 파일 수정 금지)을
    러너가 강제해야 하는데 범용 도구는 그 반대를 기본값으로 삼는다.
