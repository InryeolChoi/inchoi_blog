#!/usr/bin/env bash
# admin(글쓰기 화면)을 켠다. **인스턴스에서 사람이 직접 돌린다.**
#
#   gcloud compute scp deploy/enable-admin.sh playground:/tmp/ \
#     --zone=us-west1-a --project=statcode --tunnel-through-iap
#   gcloud compute ssh playground --zone=us-west1-a --project=statcode \
#     --tunnel-through-iap -- -t 'sudo bash /tmp/enable-admin.sh'
#
# **client secret을 물어보는 자리가 여기다.** 명령줄 인자로도, 환경변수로도
# 받지 않는다 — 인자는 ps에 보이고 셸 히스토리에 남는다. 키보드에서 곧장
# /etc/blog/admin.env(0600 root:root)로 들어가고 화면에도 안 찍힌다.
#
# 먼저 GitHub에 OAuth 앱을 만들어야 한다:
#   Settings → Developer settings → OAuth Apps → New OAuth App
#     Application name:           blog admin (아무거나)
#     Homepage URL:               https://inquieto.dev
#     Authorization callback URL: https://inquieto.dev/admin/auth/callback
#
# callback 주소는 **정확히** 저것이어야 한다. 서버가 redirect_uri를 직접
# 만들어 보내지 않고 GitHub에 등록된 것을 쓰기 때문이다(internal/admin/auth.go).
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "root로 돌려야 한다: sudo bash $0" >&2
  exit 1
fi
if [[ ! -t 0 ]]; then
  echo "이 스크립트는 물어볼 것이 있어서 터미널이 필요하다." >&2
  echo "gcloud compute ssh ... -- -t 'sudo bash /tmp/enable-admin.sh'" >&2
  exit 1
fi

ENV=/etc/blog/admin.env
[[ -f $ENV ]] || { echo "$ENV가 없다. setup-caddy.sh를 먼저 돌려라." >&2; exit 1; }

echo "GitHub OAuth 앱의 값을 넣는다. callback은"
echo "  https://inquieto.dev/admin/auth/callback"
echo "로 등록돼 있어야 한다."
echo

read -r -p "Client ID: " CID
read -r -s -p "Client secret (화면에 안 보인다): " CSEC; echo
read -r -p "허용할 GitHub 계정 [InryeolChoi]: " LOGINS
LOGINS="${LOGINS:-InryeolChoi}"

[[ -n $CID   ]] || { echo "Client ID가 비었다." >&2; exit 1; }
[[ -n $CSEC  ]] || { echo "Client secret이 비었다." >&2; exit 1; }
# **비면 전부 차단이다.** 여기서 걸러 "로그인은 되는데 아무도 못 들어오는"
# 상태를 만들지 않는다.
[[ -n $LOGINS ]] || { echo "허용 계정이 비었다. 비면 아무도 못 들어온다." >&2; exit 1; }

# 세션 키는 setup-caddy.sh가 만들어둔 것을 그대로 쓴다. 다시 만들면
# 살아 있는 로그인이 전부 풀린다.
KEY="$(grep -E '^BLOG_SESSION_KEY=' "$ENV" | cut -d= -f2-)"
if [[ ${#KEY} -lt 32 ]]; then
  echo "세션 키가 없거나 짧다. 새로 만든다."
  KEY="$(openssl rand -hex 32)"
fi

cp -a "$ENV" "$ENV.prev"
umask 077
cat > "$ENV" <<EOF
# admin(글쓰기 화면) 설정. **이 파일은 저장소에 없다. 이 기계에만 있다.**
# enable-admin.sh가 마지막으로 쓴 것: $(date -Is)
BLOG_GITHUB_CLIENT_ID=$CID
BLOG_GITHUB_CLIENT_SECRET=$CSEC

# 들어올 수 있는 GitHub 계정. 쉼표로 여럿.
# **비면 "전부 허용"이 아니라 "전부 차단"이다.**
BLOG_ADMIN_LOGINS=$LOGINS

# 세션 쿠키 서명 키. 바꾸면 로그인이 전부 풀린다.
BLOG_SESSION_KEY=$KEY

# admin 스위치. 지우면 admin이 닫히고 사이트는 그대로 돈다.
BLOG_ADMIN_FLAG=-admin
EOF
chmod 0600 "$ENV"; chown root:root "$ENV"
rm -f "$ENV.prev"

echo
echo "== 재시작 =="
systemctl restart blog
sleep 2
if ! systemctl is-active --quiet blog; then
  echo "blog가 안 떴다. 설정이 모자라면 일부러 안 뜬다(cmd/blog의 adminAuth):" >&2
  journalctl -u blog -n 30 --no-pager >&2
  exit 1
fi

echo "== 확인 =="
printf "  /admin/login  : %s (200이어야 한다)\n" \
  "$(curl -sS -o /dev/null -w '%{http_code}' -H 'Host: inquieto.dev' http://127.0.0.1/admin/login)"
printf "  /admin        : %s (303 = 로그인으로 보냄)\n" \
  "$(curl -sS -o /dev/null -w '%{http_code}' -H 'Host: inquieto.dev' http://127.0.0.1/admin)"
printf "  /api/admin/posts: %s (401 = 막힘)\n" \
  "$(curl -sS -o /dev/null -w '%{http_code}' -H 'Host: inquieto.dev' http://127.0.0.1/api/admin/posts)"
echo
echo "이제 https://inquieto.dev/admin 으로 들어간다."
echo "끄려면: sudo sed -i 's/^BLOG_ADMIN_FLAG=/#BLOG_ADMIN_FLAG=/' $ENV && sudo systemctl restart blog"
