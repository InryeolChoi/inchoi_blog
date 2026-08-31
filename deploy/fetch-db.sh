#!/usr/bin/env bash
# 로컬(맥)에서 돌린다. **서버의 DB를 내려받아 backups/에 둔다.**
#
#   ./deploy/fetch-db.sh
#
# **이것이 평소 방향이다.** admin 화면에서 글을 쓰기 시작한 뒤로 정본은
# 서버의 /var/lib/blog/blog.db이고, 로컬 blog.db는 이관 파이프라인을 돌리는
# 작업본일 뿐이다. 올리는 쪽(deploy/upload-db.sh)은 예외적인 일이다.
#
# **로컬 blog.db를 덮지 않는다.** backups/에 날짜 이름으로 떨어뜨린다.
# 덮어쓰는 것은 사람이 보고 직접 한다 — 그 한 번이 이관 작업본을 날리는 수다.
set -euo pipefail

ZONE=${ZONE:-us-west1-a}
PROJECT=${PROJECT:-statcode}
INSTANCE=${INSTANCE:-playground}
DEST=${DEST:-backups}

cd "$(dirname "$0")/.."
mkdir -p "$DEST"

command -v sqlite3 >/dev/null || { echo "로컬에 sqlite3이 없다: brew install sqlite" >&2; exit 1; }

ssh_() {
  gcloud compute ssh "$INSTANCE" --zone="$ZONE" --project="$PROJECT" \
    --tunnel-through-iap --command="$1" | tr -d '\r'
}

# 지문은 **행을 뽑아 해싱한다.** 파일 해시로는 두 DB가 같은지 알 수 없다 —
# SQLite는 열기만 해도 본체를 바꾸고, WAL에 변경이 남아 있으면 내용이 다른데도
# 같게 나온다. 2026-08-24 첫 배포에서 실제로 그렇게 속았다.
# (sqlite3의 기본 출력 구분자가 "|"라 따로 이어붙일 필요가 없다)
FP_SQL="select id,slug,status,updated_at from posts order by id"

echo "== 서버에서 새 백업을 뜬다 =="
# 있는 것 중 최신을 가져오는 게 아니라 **지금 뜬 것**을 가져온다. 어제 것을
# 받아놓고 오늘 것이라 믿는 일이 없어야 한다.
ssh_ 'sudo systemctl start blog-backup && sudo journalctl -u blog-backup -n 5 --no-pager | tail -3'

echo
echo "== 내려받을 것을 고르고 서버에서 지문을 잰다 =="
# /var/backups/blog는 0700 blog:blog라 SSH 계정이 못 읽는다. /tmp로 한 벌
# 옮기고 소유를 넘긴 뒤 받는다.
OUTPUT=$(ssh_ "set -e
  # **glob을 sudo 안에서 푼다.** /var/backups/blog는 0700 blog:blog라
  # SSH 계정 쪽에서는 패턴이 아무것에도 안 맞고, 그대로 sudo ls에 넘어가
  # \"No such file\"이 난다. 실제로 여기서 한 번 걸렸다.
  f=\$(sudo sh -c 'ls -1t /var/backups/blog/blog-*.db.gz | head -1')
  sudo cp \"\$f\" /tmp/blog-fetch.db.gz
  sudo chown \"\$(id -u):\$(id -g)\" /tmp/blog-fetch.db.gz
  gunzip -c /tmp/blog-fetch.db.gz > /tmp/blog-fetch-plain.db
  fp=\$(sqlite3 'file:/tmp/blog-fetch-plain.db?immutable=1' '$FP_SQL' | sha256sum | cut -c1-16)
  rm -f /tmp/blog-fetch-plain.db
  echo \"\$(basename \"\$f\") \$fp\"" | tail -1)

REMOTE=${OUTPUT%% *}
REMOTE_FP=${OUTPUT##* }
if [[ -z $REMOTE || -z $REMOTE_FP || $REMOTE == "$REMOTE_FP" ]]; then
  echo "서버에서 파일 이름과 지문을 못 읽었다: [$OUTPUT]" >&2
  exit 1
fi
echo "$REMOTE  지문 $REMOTE_FP"

echo
echo "== 받는다 =="
gcloud compute scp "$INSTANCE:/tmp/blog-fetch.db.gz" "$DEST/$REMOTE" \
  --zone="$ZONE" --project="$PROJECT" --tunnel-through-iap
ssh_ 'rm -f /tmp/blog-fetch.db.gz' >/dev/null

OUT="$DEST/${REMOTE%.gz}"
gunzip -f -c "$DEST/$REMOTE" > "$OUT"

echo
echo "== 확인 =="
CHECK=$(sqlite3 "file:$OUT?immutable=1" "pragma integrity_check" | head -1)
[[ $CHECK == ok ]] || { echo "무결성 검사 실패: $CHECK" >&2; exit 1; }

LOCAL_FP=$(sqlite3 "file:$OUT?immutable=1" "$FP_SQL" | shasum -a 256 | cut -c1-16)
if [[ $LOCAL_FP != "$REMOTE_FP" ]]; then
  echo "지문이 다르다. 서버=$REMOTE_FP 받은것=$LOCAL_FP" >&2
  exit 1
fi

q() { sqlite3 "file:$OUT?immutable=1" "$1"; }
echo "  $OUT"
echo "  posts=$(q 'select count(*) from posts')" \
     "images=$(q 'select count(*) from images')" \
     "웹에서쓴글=$(q "select count(*) from posts where notion_page_id is null")"
echo "  지문 $LOCAL_FP (서버와 일치)"
echo
echo "backups/는 gitignore다. 오래된 것은 사람이 지운다."
