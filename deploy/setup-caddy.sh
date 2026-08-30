#!/usr/bin/env bash
# 인스턴스에서 돌린다. **몇 번을 다시 돌려도 결과가 같다.**
#
#   gcloud compute scp deploy/setup-caddy.sh deploy/Caddyfile deploy/blog.service \
#     deploy/caddy.service.d/limits.conf \
#     playground:/tmp/ --zone=us-west1-a --project=statcode --tunnel-through-iap
#   gcloud compute ssh playground --zone=us-west1-a --project=statcode \
#     --tunnel-through-iap --command='sudo bash /tmp/setup-caddy.sh'
#
# 하는 일:
#   1. Caddy를 apt로 깐다 (**설치 중에 자동 시작되지 않게 막는다** — 아래 참고)
#   2. /etc/caddy/Caddyfile과 메모리 drop-in을 저장소 것으로 맞춘다
#   3. /etc/blog/admin.env를 만든다 (없을 때만. 세션 키는 여기서 생성한다)
#   4. blog를 127.0.0.1:8080으로 내린다
#   5. Caddy를 켠다
#
# **DB도 바이너리도 건드리지 않는다.**
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "root로 돌려야 한다: sudo bash $0" >&2
  exit 1
fi

need() {
  if [[ ! -f "/tmp/$1" ]]; then
    echo "/tmp/$1이 없다. deploy/에서 먼저 올려라." >&2
    exit 1
  fi
}
need Caddyfile
need blog.service
need limits.conf

# ---------------------------------------------------------------- 1. 설치

if ! command -v caddy >/dev/null 2>&1; then
  echo "== Caddy 설치 =="
  export DEBIAN_FRONTEND=noninteractive
  apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl

  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    > /etc/apt/sources.list.d/caddy-stable.list
  apt-get update

  # **설치 중에 caddy가 저절로 뜨면 안 된다.** 이 시점에는 blog가 아직
  # 80번을 물고 있어서, 뜨자마자 바인드에 실패하고 postinst가 0이 아닌
  # 값을 돌려주면 set -e에 걸려 스크립트가 통째로 죽는다. policy-rc.d는
  # 데비안이 그러라고 만들어둔 자리다 — 101을 돌려주면 패키지가 서비스
  # 시작을 건너뛴다.
  cat > /usr/sbin/policy-rc.d <<'EOF'
#!/bin/sh
exit 101
EOF
  chmod +x /usr/sbin/policy-rc.d
  apt-get install -y caddy
  rm -f /usr/sbin/policy-rc.d
else
  echo "== Caddy 이미 있음 ($(caddy version | head -1)) =="
fi

# ------------------------------------------------------------- 2. 설정 파일

echo "== Caddyfile =="
install -o root -g root -m 0644 /tmp/Caddyfile /etc/caddy/Caddyfile

install -d -o root -g root -m 0755 /etc/systemd/system/caddy.service.d
install -o root -g root -m 0644 /tmp/limits.conf \
  /etc/systemd/system/caddy.service.d/limits.conf

# --------------------------------------------------------- 3. admin 설정 자리

install -d -o root -g root -m 0755 /etc/blog

if [[ ! -f /etc/blog/admin.env ]]; then
  echo "== /etc/blog/admin.env 생성 (빈 껍데기 + 새 세션 키) =="
  # **세션 키는 여기서 만든다.** 사람 손을 거치지 않고, 저장소에도 남지
  # 않고, 로그에도 안 찍힌다. 이 키로 세션 쿠키를 서명하므로 새로 만들면
  # 그때까지의 로그인이 전부 풀린다 — 그래서 있으면 덮지 않는다.
  key="$(openssl rand -hex 32)"
  cat > /etc/blog/admin.env <<EOF
# admin(글쓰기 화면) 설정. **이 파일은 저장소에 없다. 이 기계에만 있다.**
#
# 채우는 법:
#   1) GitHub → Settings → Developer settings → OAuth Apps → New OAuth App
#      Homepage URL:               https://inquieto.dev
#      Authorization callback URL: https://inquieto.dev/admin/auth/callback
#   2) 나온 Client ID와 Client secret을 아래에 적는다
#   3) 맨 아래 BLOG_ADMIN_FLAG의 주석을 푼다
#   4) sudo systemctl restart blog
#
# **셋 중 하나라도 비어 있는데 -admin을 주면 서버가 아예 안 뜬다.**
# (cmd/blog의 adminAuth — 애매하면 안 뜬다)
BLOG_GITHUB_CLIENT_ID=
BLOG_GITHUB_CLIENT_SECRET=

# 들어올 수 있는 GitHub 계정. 쉼표로 여럿.
# **비면 "전부 허용"이 아니라 "전부 차단"이다.**
BLOG_ADMIN_LOGINS=InryeolChoi

# 세션 쿠키 서명 키. setup-caddy.sh가 만들었다. 바꾸면 로그인이 전부 풀린다.
BLOG_SESSION_KEY=$key

# admin을 켜는 스위치. 위 셋을 채운 뒤에 주석을 푼다.
#BLOG_ADMIN_FLAG=-admin
EOF
  chmod 0600 /etc/blog/admin.env
  chown root:root /etc/blog/admin.env
else
  echo "== /etc/blog/admin.env 이미 있음 (건드리지 않는다) =="
  # 권한만은 매번 조인다. client secret이 들어 있는 파일이다.
  chmod 0600 /etc/blog/admin.env
  chown root:root /etc/blog/admin.env
fi

# --------------------------------------------------- 4. blog를 loopback으로

echo "== blog.service =="
install -o root -g root -m 0644 /tmp/blog.service /etc/systemd/system/blog.service
systemctl daemon-reload

# Caddyfile이 틀렸으면 여기서 멈춘다. blog를 80번에서 내린 다음에 알면
# 사이트가 뜬 채로 아무것도 안 듣는 상태가 된다.
echo "== Caddyfile 검사 =="
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile

systemctl restart blog
systemctl is-active blog >/dev/null || { journalctl -u blog -n 30 --no-pager; exit 1; }

# ------------------------------------------------------------- 5. Caddy 켜기

echo "== Caddy 시작 =="
systemctl enable caddy
systemctl restart caddy

# --------------------------------------------------------------- 확인

echo
echo "== 듣고 있는 것 =="
ss -tlnp | grep -E 'blog|caddy' || true
echo
echo "== blog 직접 (127.0.0.1:8080) =="
curl -fsS -o /dev/null -w "  %{http_code}\n" http://127.0.0.1:8080/
echo "== Caddy 거쳐서 (127.0.0.1:80) =="
curl -fsS -o /dev/null -w "  %{http_code} -> %{redirect_url}\n" \
  -H 'Host: inquieto.dev' http://127.0.0.1/
echo
echo "인증서는 Caddy가 Let's Encrypt에서 받는다. 처음에는 몇 초 걸린다:"
echo "  sudo journalctl -u caddy -n 40 --no-pager"
