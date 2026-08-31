#!/usr/bin/env bash
# 인스턴스에서 돌린다. **몇 번을 다시 돌려도 결과가 같다.**
#
#   gcloud compute scp deploy/setup-backup.sh deploy/backup-db.sh \
#     deploy/blog-backup.service deploy/blog-backup.timer \
#     playground:/tmp/ --zone=us-west1-a --project=statcode --tunnel-through-iap
#   gcloud compute ssh playground --zone=us-west1-a --project=statcode \
#     --tunnel-through-iap --command='sudo bash /tmp/setup-backup.sh'
#
# 하는 일:
#   1. sqlite3을 깐다 (백업과 진단 양쪽에 쓴다)
#   2. /opt/blog/backup-db.sh와 systemd 유닛을 저장소 것으로 맞춘다
#   3. timer를 켜고 **지금 한 번 돌려서 실제로 되는지 본다**
#
# **DB도 바이너리도 Caddy도 건드리지 않는다.**
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "root로 돌려야 한다: sudo bash $0" >&2
  exit 1
fi

need() {
  [[ -f "/tmp/$1" ]] || { echo "/tmp/$1이 없다. deploy/에서 먼저 올려라." >&2; exit 1; }
}
need backup-db.sh
need blog-backup.service
need blog-backup.timer

# ---------------------------------------------------------------- 1. sqlite3

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "== sqlite3 설치 =="
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y sqlite3
else
  echo "== sqlite3 이미 있음: $(sqlite3 --version) =="
fi

# VACUUM INTO는 SQLite 3.27부터다. Ubuntu 22.04는 3.37이라 넉넉하지만,
# **없는 채로 timer만 켜두면 매일 조용히 실패한다.**
if ! sqlite3 :memory: "vacuum into '/tmp/.blog-vacuum-probe.db'" 2>/dev/null; then
  echo "이 sqlite3은 VACUUM INTO를 모른다: $(sqlite3 --version)" >&2
  exit 1
fi
rm -f /tmp/.blog-vacuum-probe.db

# ---------------------------------------------------------------- 2. 설치

install -d -o blog -g blog -m 0700 /var/backups/blog
install -o root -g root -m 0755 /tmp/backup-db.sh /opt/blog/backup-db.sh
install -o root -g root -m 0644 /tmp/blog-backup.service /etc/systemd/system/blog-backup.service
install -o root -g root -m 0644 /tmp/blog-backup.timer   /etc/systemd/system/blog-backup.timer
rm -f /tmp/backup-db.sh /tmp/blog-backup.service /tmp/blog-backup.timer

systemctl daemon-reload

# ---------------------------------------------------------------- 3. 켜고 확인

systemctl enable --now blog-backup.timer

echo "== 지금 한 번 돌린다 =="
# **여기서 실패하면 스크립트도 실패한다.** timer만 켜두고 "됐다"고 하면
# 처음 실패를 하루 뒤에, 그것도 아무도 안 볼 때 알게 된다.
systemctl start blog-backup
systemctl show -p Result --value blog-backup.service | grep -qx success || {
  echo "첫 백업이 실패했다:" >&2
  journalctl -u blog-backup -n 40 --no-pager >&2
  exit 1
}

echo
echo "== 확인 =="
journalctl -u blog-backup -n 20 --no-pager | tail -8
systemctl list-timers blog-backup.timer --no-pager
ls -la /var/backups/blog/
