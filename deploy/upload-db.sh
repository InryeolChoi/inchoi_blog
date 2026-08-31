#!/usr/bin/env bash
# 로컬(맥)에서 돌린다. **로컬 blog.db를 서버로 올린다 — 예외적인 일이다.**
#
#   ./deploy/upload-db.sh
#
# **정본은 서버다.** admin 화면에서 글을 쓰기 시작한 뒤로 이 방향은 평소가
# 아니라 사고가 나는 방향이다. 평소에는 deploy/fetch-db.sh로 내려받는다.
# 여기를 쓰는 경우는 하나뿐이다: 로컬에서 이관 파이프라인
# (import → relink → categorize → regroup)을 다시 돌려야 했을 때.
#
# 그래서 **묻지 않고 덮지 않는다.** 올리기 전에 서버에서 백업을 뜨고,
# deploy/upload-guard.sql로 "이 파일로 덮으면 서버에만 있는 것이 사라지는가"를
# 확인한다. 한 줄이라도 나오면 멈춘다.
set -euo pipefail

ZONE=${ZONE:-us-west1-a}
PROJECT=${PROJECT:-statcode}
INSTANCE=${INSTANCE:-playground}
DB=${DB:-blog.db}
SITE=${SITE:-https://inquieto.dev}

cd "$(dirname "$0")/.."
[[ -f $DB ]] || { echo "$DB가 없다." >&2; exit 1; }
command -v sqlite3 >/dev/null || { echo "로컬에 sqlite3이 없다: brew install sqlite" >&2; exit 1; }

ssh_() {
  gcloud compute ssh "$INSTANCE" --zone="$ZONE" --project="$PROJECT" \
    --tunnel-through-iap --command="$1" | tr -d '\r'
}

FP_SQL="select id,slug,status,updated_at from posts order by id"

# ------------------------------------------------- 1. 올릴 것부터 성하게 만든다

echo "== 로컬 =="
# **WAL을 먼저 접는다.** 이 DB는 WAL 모드라 최근 변경이 blog.db-wal에만
# 있을 수 있다. 본체만 올리면 옛 DB가 올라간다 — 2026-08-24 첫 배포에서
# 실제로 그렇게 됐고, 파일 해시는 그걸 못 잡았다.
CP=$(sqlite3 "$DB" "pragma wal_checkpoint(truncate)")
echo "  checkpoint: $CP"
[[ ${CP%%|*} == 0 ]] || { echo "checkpoint가 안 끝났다. 서버나 도구가 이 DB를 열고 있다." >&2; exit 1; }
if [[ -s ${DB}-wal ]]; then
  echo "  ${DB}-wal이 아직 0바이트가 아니다." >&2
  exit 1
fi

CHECK=$(sqlite3 "file:$DB?immutable=1" "pragma integrity_check" | head -1)
[[ $CHECK == ok ]] || { echo "  무결성 검사 실패: $CHECK" >&2; exit 1; }

LOCAL_FP=$(sqlite3 "file:$DB?immutable=1" "$FP_SQL" | shasum -a 256 | cut -c1-16)
q() { sqlite3 "file:$DB?immutable=1" "$1"; }
echo "  posts=$(q 'select count(*) from posts')" \
     "images=$(q 'select count(*) from images')" \
     "지문 $LOCAL_FP"

# ------------------------------------------------- 2. 서버 백업이 먼저다

echo
echo "== 서버 백업 =="
ssh_ 'sudo systemctl start blog-backup && sudo journalctl -u blog-backup -n 5 --no-pager | tail -2'

# ------------------------------------------------- 3. 올려놓고, 덮기 전에 견준다

echo
echo "== 올려놓는다 (아직 안 바꾼다) =="
gcloud compute scp "$DB" "$INSTANCE:/tmp/blog-upload.db" \
  --zone="$ZONE" --project="$PROJECT" --tunnel-through-iap
gcloud compute scp deploy/upload-guard.sql "$INSTANCE:/tmp/upload-guard.sql" \
  --zone="$ZONE" --project="$PROJECT" --tunnel-through-iap

echo
echo "== 덮으면 사라지거나 되살아나는 것이 있는지 본다 =="
# blog 사용자로 읽는다. root로 열면 SQLite가 만드는 -shm의 소유가 어긋난다.
GUARD=$(ssh_ "sudo runuser -u blog -- sqlite3 /var/lib/blog/blog.db \
  \"attach 'file:/tmp/blog-upload.db?mode=ro' as newdb\" '.read /tmp/upload-guard.sql'")

if [[ -n ${GUARD//[[:space:]]/} ]]; then
  echo
  echo "$GUARD"
  echo
  echo "**올리지 않았다.**" >&2
  echo "  LOST/STALE/LOSTIMG — 서버에만 있는 것을 이 파일이 안 들고 있다." >&2
  echo "  BACK              — 서버에서 지운 글을 이 파일이 되살린다." >&2
  echo "서버가 정본이다. 먼저 ./deploy/fetch-db.sh로 내려받아 합친 뒤에 올려라." >&2
  ssh_ 'rm -f /tmp/blog-upload.db /tmp/upload-guard.sql' >/dev/null
  exit 1
fi
echo "  잃을 것 없음."

# ------------------------------------------------- 4. 바꾼다

echo
echo "== 갈아끼운다 =="
# **서버를 멈추고 바꾼다.** 도는 중에 갈아끼우면 열려 있던 커넥션이 옛 파일을
# 들고 있고, 남은 -wal이 새 본체와 짝이 안 맞는다.
ssh_ 'set -eux
  sudo systemctl stop blog
  sudo rm -f /var/lib/blog/blog.db-wal /var/lib/blog/blog.db-shm
  sudo install -o blog -g blog -m 0644 /tmp/blog-upload.db /var/lib/blog/blog.db
  rm -f /tmp/blog-upload.db /tmp/upload-guard.sql
  sudo systemctl start blog'

# ------------------------------------------------- 5. 정말 그것인지 본다

echo
echo "== 확인 =="
sleep 3
SERVER_FP=$(ssh_ "sudo runuser -u blog -- sqlite3 /var/lib/blog/blog.db \"$FP_SQL\" \
  | sha256sum | cut -c1-16" | tail -1)
if [[ $SERVER_FP != "$LOCAL_FP" ]]; then
  echo "지문이 다르다. 올린것=$LOCAL_FP 서버=$SERVER_FP" >&2
  exit 1
fi
echo "  지문 $SERVER_FP (올린 것과 일치)"

CODE=$(curl -s -o /dev/null -w '%{http_code}' "$SITE/")
echo "  $SITE/ → $CODE"
[[ $CODE == 200 ]] || { echo "사이트가 200이 아니다." >&2; exit 1; }
