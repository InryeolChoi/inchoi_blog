# 배포

바이너리 하나와 SQLite 파일 하나, 그 앞에 Caddy 하나다. 컨테이너도 런타임도
없다.

| | |
|---|---|
| 프로젝트 | `statcode` |
| 인스턴스 | `playground` (us-west1-a, e2-micro, Ubuntu 22.04, amd64) |
| 외부 주소 | **`https://inquieto.dev`** (A 레코드 → 35.230.119.252) |
| 앞단 | Caddy — 80/443을 듣고 `127.0.0.1:8080`으로 넘긴다. 인증서는 자동 (`deploy/Caddyfile`) |
| 바이너리 | `/opt/blog/blog` — **GitHub Actions가 관리한다** |
| DB | `/var/lib/blog/blog.db` — **여기가 정본이다.** CI는 절대 안 건드린다 |
| DB 백업 | `/var/backups/blog/` — `blog-backup.timer`가 하루 한 번 (`deploy/backup-db.sh`) |
| 서비스 | `systemd`의 `blog` (`deploy/blog.service`)와 `caddy` |
| admin 설정 | `/etc/blog/admin.env` (0600 root:root) — **저장소에 없다. 이 기계에만 있다** |

**밖에서 닿는 것은 80과 443과 22뿐이다.** blog 자신은 `127.0.0.1:8080`에만
붙으므로 방화벽 규칙과 무관하게 밖에서 직접 못 부른다. Caddy의 관리 API도
`127.0.0.1:2019`라 마찬가지다.

**정본은 서버의 `/var/lib/blog/blog.db`다** (2026-08-31에 뒤집었다). admin
화면이 열려 있어서 글이 여기에 쓰인다. 그래서 평소 방향은 **서버 → 로컬**이고
(`deploy/fetch-db.sh`), 올리는 쪽은 예외적인 일이다 — 로컬에서 이관
파이프라인(`import` → `relink` → `categorize` → `regroup`)을 다시 돌렸을 때뿐이고,
그때도 `deploy/upload-db.sh`가 **덮으면 사라지는 것이 있는지 먼저 본다.**

**CI는 DB를 안 올린다.** 배포마다 덮으면 그 사이에 쓴 글이 통째로 사라진다.

---

## 1. 한 번만 하는 것

### 1-1. API 켜기

```sh
gcloud services enable compute.googleapis.com oslogin.googleapis.com iap.googleapis.com \
  --project=statcode
```

### 1-2. OS Login 켜기

CI가 SSH 키를 프로젝트 메타데이터에 심는 대신 OS Login으로 들어온다. 권한을
IAM에서 주고 회수할 수 있고, 키가 인스턴스 메타데이터에 쌓이지 않는다.

```sh
gcloud compute instances add-metadata playground \
  --zone=us-west1-a --project=statcode \
  --metadata=enable-oslogin=TRUE
```

### 1-3. 배포용 서비스 계정

```sh
# 만들기
gcloud iam service-accounts create blog-deployer \
  --project=statcode \
  --display-name="blog deploy (GitHub Actions)"

SA=blog-deployer@statcode.iam.gserviceaccount.com

# 인스턴스를 찾아볼 수 있어야 한다 (scp/ssh가 먼저 describe를 한다)
gcloud projects add-iam-policy-binding statcode \
  --member="serviceAccount:$SA" --role="roles/compute.viewer"

# OS Login으로 들어가서 sudo까지 한다 (systemctl restart, /opt/blog 쓰기)
gcloud projects add-iam-policy-binding statcode \
  --member="serviceAccount:$SA" --role="roles/compute.osAdminLogin"

# IAP 터널로 22번에 닿는다 (allow-iap-ssh, 35.235.240.0/20)
gcloud projects add-iam-policy-binding statcode \
  --member="serviceAccount:$SA" --role="roles/iap.tunnelResourceAccessor"

# 인스턴스가 서비스 계정으로 돌기 때문에, 거기 SSH하려면 그 계정을
# "쓸 수 있는" 권한이 따로 필요하다. 프로젝트 전체가 아니라 그 계정에만 준다.
gcloud iam service-accounts add-iam-policy-binding \
  266552334168-compute@developer.gserviceaccount.com \
  --project=statcode \
  --member="serviceAccount:$SA" --role="roles/iam.serviceAccountUser"
```

### 1-4. 키 만들어 GitHub Secret에 넣기

```sh
gcloud iam service-accounts keys create /tmp/blog-deployer.json \
  --iam-account=blog-deployer@statcode.iam.gserviceaccount.com \
  --project=statcode

# 저장소에 넣는다 (gh CLI가 있으면)
gh secret set GCP_SA_KEY --repo InryeolChoi/inchoi_blog < /tmp/blog-deployer.json

# 넣었으면 로컬 파일은 지운다. 이 키는 이 인스턴스에 sudo로 들어갈 수 있다.
rm -f /tmp/blog-deployer.json
```

`gh`가 없으면 GitHub → Settings → Secrets and variables → Actions →
New repository secret, 이름 `GCP_SA_KEY`, 값은 JSON 파일 내용 전부.

- 조직 정책 `iam.disableServiceAccountKeyCreation`이 걸려 있으면 키 생성이
  막힌다. 그때는 Workload Identity Federation(키 없는 방식)으로 가야 하고,
  워크플로의 `auth` 단계만 바뀐다.
- **키를 지우고 싶어지면** `gcloud iam service-accounts keys list --iam-account=$SA`로
  보고 `keys delete`로 지운다. 그러면 배포만 멈추고 서비스는 계속 돈다.

### 1-5. 인스턴스 준비

```sh
gcloud compute scp deploy/setup-instance.sh deploy/blog.service \
  playground:/tmp/ --zone=us-west1-a --project=statcode --tunnel-through-iap

gcloud compute ssh playground --zone=us-west1-a --project=statcode \
  --tunnel-through-iap --command='sudo bash /tmp/setup-instance.sh'
```

`blog` 사용자, `/opt/blog`, `/var/lib/blog`, systemd 유닛을 만든다. 몇 번을
다시 돌려도 결과가 같다. **DB는 만들지 않는다.**

### 1-6. Caddy와 HTTPS

```sh
gcloud compute scp deploy/setup-caddy.sh deploy/Caddyfile deploy/blog.service \
  deploy/caddy.service.d/limits.conf \
  playground:/tmp/ --zone=us-west1-a --project=statcode --tunnel-through-iap

gcloud compute ssh playground --zone=us-west1-a --project=statcode \
  --tunnel-through-iap --command='sudo bash /tmp/setup-caddy.sh'
```

Caddy를 깔고, `blog`를 `127.0.0.1:8080`으로 내리고, `/etc/blog/admin.env`를
만든다(세션 키는 거기서 생성한다). 몇 번을 다시 돌려도 결과가 같다.

- **DNS A 레코드가 먼저 붙어 있어야 한다.** Caddy는 HTTP-01로 검증하므로
  `inquieto.dev`가 이 기계로 와야 인증서가 나온다. 올리기 전에 확인:
  `dig +short @8.8.8.8 inquieto.dev A`
- **`www`는 Caddyfile에 없다.** 지금 `www.inquieto.dev`는 Porkbun 파킹을
  가리킨다. 적으면 그 이름의 검증이 실패하고 **발급 전체가 막힌다.** 쓰려면
  A 레코드를 먼저 옮긴다.
- **설치 중에 Caddy가 저절로 뜨지 않게 막는다**(`policy-rc.d`). 그 시점에는
  blog가 아직 80번을 물고 있어서, 뜨자마자 바인드에 실패하고 postinst가
  0이 아닌 값을 돌려주면 스크립트가 통째로 죽는다.
- 사이트가 멈추는 것은 blog를 8080으로 내리고 Caddy를 켜는 사이 몇 초뿐이다.

### 1-7. admin 켜기 (선택)

**글쓰기 화면은 기본으로 꺼져 있다.** 켜려면 GitHub OAuth 앱이 필요하다.

```
GitHub → Settings → Developer settings → OAuth Apps → New OAuth App
  Homepage URL:               https://inquieto.dev
  Authorization callback URL: https://inquieto.dev/admin/auth/callback
```

callback은 **정확히** 저것이어야 한다. 서버가 `redirect_uri`를 직접 만들어
보내지 않고 GitHub에 등록된 것을 쓴다(`internal/admin/auth.go`).

```sh
gcloud compute scp deploy/enable-admin.sh playground:/tmp/ \
  --zone=us-west1-a --project=statcode --tunnel-through-iap

# -t: 터미널을 붙인다. 스크립트가 물어보는 것이 있다.
gcloud compute ssh playground --zone=us-west1-a --project=statcode \
  --tunnel-through-iap -- -t 'sudo bash /tmp/enable-admin.sh'
```

- **client secret을 인자로도 환경변수로도 받지 않는다.** 인자는 `ps`에 보이고
  셸 히스토리에 남는다. 스크립트가 물어보고, 입력은 화면에 안 찍히며,
  곧장 `/etc/blog/admin.env`(0600 root:root)로 들어간다.
- **저장소에도, GitHub Secret에도 넣지 않는다.** CI는 이 파일을 모른다.
- 끄려면 `BLOG_ADMIN_FLAG`를 주석 처리하고 `systemctl restart blog`.
  사이트는 그대로 돈다.
- **허용 계정이 비면 "전부 허용"이 아니라 "전부 차단"이다.**

### 1-8. DB 백업 켜기

**정본이 서버로 넘어온 이상 백업은 미룰 수 없다.** 하루 한 번
`VACUUM INTO`로 스냅샷을 뜨고 gzip해서 `/var/backups/blog/`에 7벌까지 둔다.

```sh
gcloud compute scp deploy/setup-backup.sh deploy/backup-db.sh \
  deploy/blog-backup.service deploy/blog-backup.timer \
  playground:/tmp/ --zone=us-west1-a --project=statcode --tunnel-through-iap

gcloud compute ssh playground --zone=us-west1-a --project=statcode \
  --tunnel-through-iap --command='sudo bash /tmp/setup-backup.sh'
```

- **서버를 멈추지 않는다.** `VACUUM INTO`는 읽기 트랜잭션 하나로 일관된
  스냅샷을 뜨므로 글을 쓰는 도중에 돌아도 반쪽짜리가 나오지 않는다.
  그냥 `cp`는 그렇지 않다 — 아래 "WAL 때문에 실제로 한 번 틀렸다" 참고.
- **`.backup`이 아니라 `VACUUM INTO`인 이유**는 결과가 새 파일 하나라
  `-wal`/`-shm` 짝을 따로 챙길 필요가 없어서다. 덤으로 압축된다
  (82.6MB → 78.2MB, gzip 뒤 68.6MB).
- **뜬 백업을 그 자리에서 열어본다.** `pragma integrity_check`가 `ok`가
  아니면 지우고 실패한다. 못 여는 백업은 백업이 아닌데, 그걸 복구하는
  날에 알게 되면 이미 늦다.
- **`setup-backup.sh`는 timer만 켜고 끝내지 않고 지금 한 번 돌려본다.**
  안 그러면 첫 실패를 하루 뒤에, 그것도 아무도 안 볼 때 알게 된다.
- 지우는 것은 **새 백업이 성공한 뒤에만** 한다.
- `sqlite3`을 여기서 깐다. 백업에도 쓰고 진단에도 쓴다.

**7벌은 인스턴스 안에만 있다.** 디스크가 통째로 날아가면 같이 날아간다.
**기계 밖으로 꺼내는 것은 `deploy/fetch-db.sh`다** (아래 §2). 자동으로
버킷에 올리는 길도 있지만(GCS + 서비스 계정), 지금은 사람이 내려받는 것으로
둔다 — 쓰는 사람이 하나고, 내려받은 것이 곧 로컬 작업본이라 어차피 필요하다.

---

## 2. 그 뒤로는

`main`에 push하면 `.github/workflows/deploy.yml`이 돈다:
테스트 → 빌드(`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`) → scp → rename으로
갈아끼우기 → `systemctl restart` → 실제로 200이 나오는지 확인.

수동으로 다시 올리려면 GitHub의 Actions 탭에서 `deploy` → Run workflow.

### DB — 정본은 서버다

**2026-08-31에 방향을 뒤집었다.** admin 화면이 열려 있어서 글이 서버의
`/var/lib/blog/blog.db`에 쓰인다. 로컬 `blog.db`는 이관 파이프라인을 돌리는
작업본일 뿐이다.

#### 평소: 내려받기

```sh
./deploy/fetch-db.sh          # → backups/blog-<UTC>.db(.gz)
```

- 있는 것 중 최신을 가져오는 게 아니라 **지금 새로 뜬 것**을 가져온다.
  어제 것을 받아놓고 오늘 것이라 믿는 일이 없어야 한다.
- **로컬 `blog.db`를 덮지 않는다.** `backups/`(gitignore)에 날짜 이름으로
  떨어뜨린다. 덮어쓰는 것은 사람이 보고 직접 한다 — 그 한 번이 이관
  작업본을 날리는 수다.
- 받은 뒤 `integrity_check`와 **행 지문**이 서버 것과 맞는지 본다.

#### 예외: 올리기

로컬에서 `import` → `relink` → `categorize` → `regroup`을 다시 돌렸을 때만이다.

```sh
./deploy/upload-db.sh
```

**묻지 않고 덮지 않는다.** 순서가 이렇다:

1. 로컬 WAL을 접고(`wal_checkpoint(truncate)`) `integrity_check`
2. **서버에서 백업을 먼저 뜬다**
3. 올려놓기만 하고, 아직 안 바꾼다
4. `deploy/upload-guard.sql`로 **덮으면 사라지는 것이 있는지 본다.**
   한 줄이라도 나오면 멈춘다

   | 표시 | 뜻 |
   |---|---|
   | `LOST` | 웹에서 쓴 글이 사라진다 (`notion_page_id`가 NULL) |
   | `STALE` | 서버 쪽이 더 최근이다 — 그 뒤에 웹에서 고쳤다는 뜻 |
   | `LOSTIMG` | 웹에서 올린 이미지가 사라진다 |

5. 그제서야 **서버를 멈추고** 갈아끼운다. 도는 중에 바꾸면 열려 있던
   커넥션이 옛 파일을 들고 있고, 남은 `-wal`이 새 본체와 짝이 안 맞는다.
   **옛 `-wal`/`-shm`을 반드시 지운다.**
6. 서버 지문이 올린 것과 같은지, 사이트가 200인지 본다

가드가 멈추면 **먼저 내려받아 합친 뒤에** 올린다. 가드를 끄는 스위치는
두지 않았다 — 끄고 싶어지는 순간이 바로 잃는 순간이다.

#### WAL 때문에 실제로 한 번 틀렸다 (2026-08-24, 첫 배포)

첫 배포에서 **2026-08-23 이전 구조의 DB가 올라갔다.** 사이트가 `웹 프로그래밍`을
보여주고 사이드바 글 수가 990이 아니라 988이었다.

- `blog.db`는 **WAL 모드**고, 그날의 변경(`서버 & API` / `클라이언트 & UI` 분리)이
  아직 `blog.db-wal`에 있었다. scp는 `blog.db` 본체만 가져가므로 **옛 내용이
  올라갔다.**
- **그런데 sha256은 맞아떨어졌다.** 양쪽 다 본체만 해싱했기 때문이다. 그 뒤에
  로컬에서 서버를 한 번 띄우자 SQLite가 WAL을 checkpoint하면서 본체가 바뀌었고,
  그제서야 해시가 갈라졌다.
- **그래서 파일 해시로 "같은 DB냐"를 판정하면 안 된다.** 열기만 해도 바뀌고,
  달라야 할 때 같게 나온다. 지금 스크립트들은 전부 **행을 뽑아 해싱한다**:

  ```sh
  sqlite3 "file:blog.db?immutable=1" \
    "select id,slug,status,updated_at from posts order by id" | shasum -a 256
  ```

  가장 확실한 것은 화면을 견주는 것이다:

  ```sh
  diff <(curl -s http://127.0.0.1:8080/) <(curl -s https://inquieto.dev/)
  ```

### 자주 보는 것

```sh
S="gcloud compute ssh playground --zone=us-west1-a --project=statcode --tunnel-through-iap --command"

$S 'sudo systemctl status blog caddy'
$S 'sudo journalctl -u blog -n 100 --no-pager'
$S 'sudo journalctl -u caddy -n 100 --no-pager'   # 인증서 발급/갱신도 여기
$S 'sudo journalctl -u blog -f'          # 따라가기
$S 'curl -sI http://127.0.0.1:8080/'              # blog 자신
$S 'curl -sI -H "Host: inquieto.dev" http://127.0.0.1/'   # Caddy를 거쳐서
$S 'sudo ss -tlnp'
$S 'free -m; df -h /'

$S 'systemctl list-timers blog-backup.timer --no-pager'   # 다음 백업
$S 'sudo journalctl -u blog-backup -n 20 --no-pager'      # 지난 백업
$S 'sudo ls -la /var/backups/blog/'
$S 'sudo systemctl start blog-backup'                     # 지금 한 벌 더
```

```sh
# 밖에서
curl -sI https://inquieto.dev/
echo | openssl s_client -servername inquieto.dev -connect inquieto.dev:443 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates
```

**인증서 갱신은 Caddy가 알아서 한다.** 만료 30일 전부터 시도하고, 실패하면
journal에 남는다. 사람이 할 일은 없지만, 도메인을 옮기거나 DNS를 바꾸면
그때는 여기를 본다.

### 되돌리기

바이너리를 한 벌만 들고 있어서 **이전 버전으로 즉시 되돌릴 수는 없다.**
되돌리려면 저장소에서 revert하고 다시 배포한다. 그게 몇 분 걸리는 게
문제가 될 만큼 바뀌면 `/opt/blog/blog.prev`를 남기도록 워크플로를 고친다.

---

## 3. 아직 안 한 것

- **`www.inquieto.dev`.** 지금 Porkbun 파킹을 가리킨다. A 레코드를 옮기고
  Caddyfile에 `www.inquieto.dev { redir https://inquieto.dev{uri} }`를
  더하면 된다. **DNS를 먼저 옮기지 않고 Caddyfile부터 고치면 발급이 막힌다.**
- **방화벽 정리.** `default-allow-ssh`(22, 0.0.0.0/0)가 남아 있다. IAP SSH가
  있으니 닫아도 되지만, 닫으면 `--tunnel-through-iap` 말고는 들어갈 길이
  없어진다. 익숙해지면 지운다.
- **백업을 기계 밖으로 자동으로 꺼내기.** 지금은 인스턴스 안에 7벌이 있고,
  밖으로 나가는 것은 사람이 `deploy/fetch-db.sh`를 돌릴 때뿐이다. 디스크가
  통째로 날아가면 그 사이 것은 같이 날아간다. GCS 버킷에 올리려면 서비스
  계정과 수명주기 규칙을 먼저 정해야 한다.
- **robots.txt / sitemap.xml.**
