#!/usr/bin/env bash
# /opt/blog/backup-db.sh — 인스턴스에서 돈다. blog-backup.timer가 하루 한 번 부른다.
#
# **서버를 멈추지 않는다.** `VACUUM INTO`는 읽기 트랜잭션 하나로 일관된 스냅샷을
# 뜨므로 글을 쓰는 도중에 돌아도 반쪽짜리 파일이 나오지 않는다. 그냥 `cp`는
# 그렇지 않다 — WAL에만 있는 변경을 두고 본체만 베끼면 옛 DB가 나온다.
# (2026-08-24 첫 배포에서 실제로 그렇게 틀렸다. deploy/README.md 참고)
#
# `.backup` 대신 `VACUUM INTO`인 이유: 결과가 압축된 새 파일 하나라
# `-wal`/`-shm` 짝을 따로 챙길 필요가 없다.
set -euo pipefail

DB=${BLOG_DB:-/var/lib/blog/blog.db}
DIR=${BLOG_BACKUP_DIR:-/var/backups/blog}
KEEP=${BLOG_BACKUP_KEEP:-7}

if [[ $EUID -ne 0 ]]; then
  echo "root로 돌려야 한다: sudo bash $0" >&2
  exit 1
fi
if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3이 없다. deploy/setup-backup.sh를 먼저 돌려라." >&2
  exit 1
fi
[[ -f $DB ]] || { echo "$DB가 없다." >&2; exit 1; }

install -d -o blog -g blog -m 0700 "$DIR"

TS=$(date -u +%Y%m%dT%H%M%SZ)
OUT="$DIR/blog-$TS.db"

# **blog 사용자로 뜬다.** root로 열면 SQLite가 -shm을 건드릴 때 소유가
# 어긋날 수 있고, 결과 파일도 root 것이 되어 blog가 못 읽는다.
runuser -u blog -- sqlite3 "$DB" "vacuum into '$OUT'"

# **뜬 것이 진짜 열리는지 여기서 본다.** 못 여는 백업은 백업이 아닌데,
# 그걸 복구하는 날에 알게 되면 이미 늦다.
CHECK=$(sqlite3 "file:$OUT?immutable=1" "pragma integrity_check" 2>&1 | head -1)
if [[ $CHECK != ok ]]; then
  echo "무결성 검사 실패: $CHECK" >&2
  rm -f "$OUT"
  exit 1
fi

# 내용 지문. **파일 해시로는 두 DB가 같은지 알 수 없다** — SQLite는 열기만
# 해도 본체를 바꾸고, WAL에 변경이 남아 있으면 내용이 다른데도 같게 나온다.
# 행을 뽑아 해싱해야 한다.
FP=$(sqlite3 "file:$OUT?immutable=1" \
  "select id||'|'||slug||'|'||status||'|'||updated_at from posts order by id" \
  | sha256sum | cut -c1-16)
POSTS=$(sqlite3 "file:$OUT?immutable=1" "select count(*) from posts")
IMAGES=$(sqlite3 "file:$OUT?immutable=1" "select count(*) from images")

gzip -f "$OUT"
chown blog:blog "$OUT.gz"
SIZE=$(stat -c %s "$OUT.gz")

echo "backup: $OUT.gz ($SIZE bytes) posts=$POSTS images=$IMAGES fp=$FP"

# 오래된 것부터 지운다. **지우기 전에 남은 개수를 센다** — 이번 백업이
# 실패했는데 정리만 돌아서 있던 것까지 사라지는 일이 없어야 한다.
mapfile -t OLD < <(ls -1t "$DIR"/blog-*.db.gz 2>/dev/null | tail -n "+$((KEEP + 1))")
for f in "${OLD[@]:-}"; do
  [[ -n $f ]] || continue
  echo "prune: $f"
  rm -f "$f"
done

df -h "$DIR" | tail -1
