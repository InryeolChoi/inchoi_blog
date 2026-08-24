# 배포

바이너리 하나와 SQLite 파일 하나다. 컨테이너도 리버스 프록시도 없다 —
e2-micro(1GB)에서 프로세스를 하나라도 덜 띄우는 편이 낫다.

| | |
|---|---|
| 프로젝트 | `statcode` |
| 인스턴스 | `playground` (us-west1-a, e2-micro, Ubuntu 22.04, amd64) |
| 외부 주소 | `http://35.230.119.252` (`default-allow-http` + `http-server` 태그로 이미 열려 있다) |
| 바이너리 | `/opt/blog/blog` — **GitHub Actions가 관리한다** |
| DB | `/var/lib/blog/blog.db` — **사람이 관리한다. CI는 절대 안 건드린다** |
| 서비스 | `systemd`의 `blog` (`deploy/blog.service`) |

**CI가 DB를 안 올리는 이유:** `blog.db`가 정본이고 앞으로 admin 화면이 거기에
글을 쓴다. 배포마다 DB를 덮으면 그 사이에 쓴 글이 통째로 사라진다. 로컬
파이프라인(`import` → `relink` → `categorize` → `regroup`)의 결과를 올려야 할
때는 아래 "DB 올리기"를 사람이 직접 한다.

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

### 1-6. DB 올리기

```sh
# 로컬 blog.db가 최신인지 먼저 확인한다 (파이프라인을 두 바퀴 돌려 0건)
go test ./...

# ① 이 DB는 WAL 모드다. 최근 변경이 blog.db-wal에만 있을 수 있으므로
#    반드시 checkpoint부터 한다. 서버가 떠 있으면 먼저 끈다.
sqlite3 blog.db "pragma wal_checkpoint(truncate);"   # 0|0|0 이 나와야 한다
ls -l blog.db-wal                                    # 0 바이트여야 한다

# ② 파일 해시가 아니라 **내용**을 적어둔다 (아래 "왜 해시로 확인하면 안 되나")
sqlite3 blog.db "select id||'|'||slug||'|'||status from posts order by id" | shasum -a 256
sqlite3 blog.db "select id||'|'||slug||'|'||coalesce(parent_id,-1) from categories order by id" | shasum -a 256

# ③ 올린다
gcloud compute scp blog.db playground:/tmp/blog.db \
  --zone=us-west1-a --project=statcode --tunnel-through-iap

gcloud compute ssh playground --zone=us-west1-a --project=statcode \
  --tunnel-through-iap --command='set -eux
    sudo systemctl stop blog
    sudo cp -a /var/lib/blog/blog.db /var/lib/blog/blog.db.prev
    sudo rm -f /var/lib/blog/blog.db-wal /var/lib/blog/blog.db-shm
    sudo install -o blog -g blog -m 0644 /tmp/blog.db /var/lib/blog/blog.db
    rm -f /tmp/blog.db
    sudo systemctl start blog'
```

- **서버를 멈추고 바꾼다.** 도는 중에 갈아끼우면 열려 있던 커넥션이 옛 파일을
  들고 있고, 남은 `-wal`이 새 본체와 짝이 안 맞는다.
- **옛 `-wal`/`-shm`을 반드시 지운다.** 본체만 갈아끼우고 남겨두면 SQLite가
  다른 DB의 WAL을 새 본체에 얹으려 든다.

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
  달라야 할 때 같게 나온다. 위 ②처럼 **행을 뽑아 해싱**하거나, 가장 확실하게는
  올린 뒤 화면을 로컬과 비교한다:

```sh
diff <(curl -s http://127.0.0.1:8080/) <(curl -s http://35.230.119.252/)
```

---

## 2. 그 뒤로는

`main`에 push하면 `.github/workflows/deploy.yml`이 돈다:
테스트 → 빌드(`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`) → scp → rename으로
갈아끼우기 → `systemctl restart` → 실제로 200이 나오는지 확인.

수동으로 다시 올리려면 GitHub의 Actions 탭에서 `deploy` → Run workflow.

### 자주 보는 것

```sh
S="gcloud compute ssh playground --zone=us-west1-a --project=statcode --tunnel-through-iap --command"

$S 'sudo systemctl status blog'
$S 'sudo journalctl -u blog -n 100 --no-pager'
$S 'sudo journalctl -u blog -f'          # 따라가기
$S 'curl -sI http://127.0.0.1/'
$S 'free -m; df -h /'
```

### 되돌리기

바이너리를 한 벌만 들고 있어서 **이전 버전으로 즉시 되돌릴 수는 없다.**
되돌리려면 저장소에서 revert하고 다시 배포한다. 그게 몇 분 걸리는 게
문제가 될 만큼 바뀌면 `/opt/blog/blog.prev`를 남기도록 워크플로를 고친다.

---

## 3. 아직 안 한 것

- **HTTPS.** 지금은 `http://35.230.119.252`뿐이다. 도메인을 정하면 Caddy를
  앞에 두고(자동 인증서) `blog.service`의 `-addr`를 `127.0.0.1:8080`으로,
  `AmbientCapabilities`를 빼면 된다. e2-micro에 프로세스가 하나 늘어난다.
- **방화벽 정리.** `default-allow-ssh`(22, 0.0.0.0/0)와 `test-jupyter`,
  `test-rstudio`, `test-vscode-server`, `test-http`가 전부 0.0.0.0/0으로
  열려 있다. IAP SSH가 생겼으니 22번은 닫아도 되고, 안 쓰는 `test-*`는
  지워도 된다.
- **백업.** DB가 인스턴스 안에만 있다. 디스크 스냅샷이든 `sqlite3 .backup`을
  버킷에 올리든 정해야 한다. admin 화면이 생겨 사람이 직접 글을 쓰기
  시작하면 그때부터는 **로컬 blog.db가 더 이상 정본이 아니라서** 미룰 수 없다.
- **robots.txt / sitemap.xml.**
