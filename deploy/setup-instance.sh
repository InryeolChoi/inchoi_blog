#!/usr/bin/env bash
# 인스턴스에서 한 번만 돌린다. 몇 번을 다시 돌려도 결과가 같다.
#
#   gcloud compute scp deploy/setup-instance.sh deploy/blog.service \
#     playground:/tmp/ --zone=us-west1-a --tunnel-through-iap
#   gcloud compute ssh playground --zone=us-west1-a --tunnel-through-iap \
#     --command='sudo bash /tmp/setup-instance.sh'
#
# 이 스크립트는 **DB를 만들지도 지우지도 않는다.** blog.db는 사람이 따로
# 올린다(아래 deploy/README.md 참고). 여기서 만들면 빈 DB가 정본 자리에
# 앉아버린다.
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "root로 돌려야 한다: sudo bash $0" >&2
  exit 1
fi

# 로그인할 일이 없는 서비스 계정이다. 홈 디렉토리도 셸도 주지 않는다.
if ! id -u blog >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin blog
  echo "사용자 blog 생성"
else
  echo "사용자 blog 이미 있음"
fi

install -d -o root -g root -m 0755 /opt/blog
install -d -o blog -g blog -m 0755 /var/lib/blog

# 유닛 파일은 이 저장소가 정본이다. 인스턴스에서 손으로 고치면 다음 배포와
# 갈라진다 — 고칠 일이 있으면 저장소에서 고치고 이 스크립트를 다시 돌린다.
if [[ ! -f /tmp/blog.service ]]; then
  echo "/tmp/blog.service가 없다. deploy/blog.service를 먼저 올려라." >&2
  exit 1
fi
install -o root -g root -m 0644 /tmp/blog.service /etc/systemd/system/blog.service
systemctl daemon-reload
systemctl enable blog

echo
echo "준비 끝. 남은 것은 둘이다:"
echo "  1) /var/lib/blog/blog.db 올리기 (아직 없으면 서버가 빈 DB를 만든다)"
echo "  2) 바이너리 올리기 — GitHub Actions가 한다"
echo
if [[ -f /var/lib/blog/blog.db ]]; then
  echo "DB: 있음 ($(du -h /var/lib/blog/blog.db | cut -f1))"
else
  echo "DB: 없음"
fi
if [[ -x /opt/blog/blog ]]; then
  echo "바이너리: 있음"
  systemctl restart blog
  systemctl --no-pager status blog | head -5
else
  echo "바이너리: 없음 (지금 시작하면 실패한다 — 배포를 기다린다)"
fi
