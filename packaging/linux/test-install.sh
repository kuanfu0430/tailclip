#!/usr/bin/env bash
# 只在可丟棄的 Debian 13 容器內執行；模擬桌面依賴，不操作真實服務。
set -Eeuo pipefail
[[ ${TAILCLIP_INSTALL_TEST:-} == 1 ]] || exit 2
source /etc/os-release
[[ $ID:$VERSION_ID == debian:13 ]] || exit 2
root=$(mktemp -d /home/tailclip-install-test.XXXXXX)
trap 'rm -rf -- "$root"' EXIT
mkdir -p "$root/mock" "$root/package"
cp "$(dirname "$0")/install.sh" "$(dirname "$0")/tailclip.service" "$(dirname "$0")/uninstall.sh" "$root/package/"
export TEST_HOME="$root" TEST_LOG="$root/calls" WAYLAND_DISPLAY=wayland-0 XDG_RUNTIME_DIR="$root/runtime"
cat > "$root/mock/getent" <<'EOF'
#!/bin/sh
printf 'tester:x:1000:1000::%s:/bin/bash\n' "$TEST_HOME"
EOF
cat > "$root/mock/systemctl" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$TEST_LOG"
EOF
for tool in ss wl-copy wl-paste; do
  printf '#!/bin/sh\nexit 0\n' > "$root/mock/$tool"
done
printf '#!/bin/sh\nprintf '\''{"service":"tailclip","ok":true}\\n'\''\n' > "$root/mock/curl"
printf '#!/bin/sh\nexit 99\n' > "$root/mock/sudo"
printf '#!/bin/sh\nprintf "binary:%%s\\n" "$*" >> "$TEST_LOG"\n' > "$root/package/tailclip"
printf 'tailclip-cloudflared-123456789abc\n' > "$root/package/CLOUDFLARED.txt"
printf '#!/bin/sh\nexit 0\n' > "$root/package/tailclip-cloudflared-123456789abc"
chmod +x "$root/mock/"* "$root/package/tailclip" "$root/package/tailclip-cloudflared-123456789abc"
export PATH="$root/mock:/usr/bin:/bin"
command -v tailscale && exit 3
bash "$root/package/install.sh"
test -x "$root/.local/bin/tailclip"
test -x "$root/.local/bin/tailclip-cloudflared-123456789abc"
cmp "$root/package/tailclip-cloudflared-123456789abc" "$root/.local/bin/tailclip-cloudflared-123456789abc"
test -r "$root/.config/systemd/user/tailclip.service"
grep -qx -- '--user restart tailclip.service' "$TEST_LOG"
grep -qx 'binary:open' "$TEST_LOG"
# 再跑一次驗證更新流程，不要求 Tailscale 或額外互動。
bash "$root/package/install.sh"
[[ $(grep -c 'binary:open' "$TEST_LOG") == 2 ]]
# 移除及再次移除均不得因沒有 companion glob 而中止。
printf 'y\n' | bash "$root/package/uninstall.sh"
test ! -e "$root/.local/bin/tailclip"
test ! -e "$root/.local/bin/tailclip-cloudflared-123456789abc"
printf 'y\n' | bash "$root/package/uninstall.sh"
printf 'Debian 13 無 Tailscale 安裝／更新／解除安裝檢查通過。\n'
