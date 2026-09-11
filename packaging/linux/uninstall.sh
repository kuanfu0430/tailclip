#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'TailBlink 解除安裝失敗：%s\n' "$1" >&2
  exit 1
}

read -r -p '將移除 TailBlink 的 Serve path、自啟、設定與日誌，確定繼續？[y/N] ' answer
case "${answer}" in
  y|Y|yes|YES) ;;
  *) printf '已取消。\n'; exit 0 ;;
esac

user_name="$(id -un)"
user_home="$(getent passwd "${user_name}" | cut -d: -f6)"
[[ -n "${user_home}" && "${user_home}" == /home/* ]] || fail "無法安全確認使用者家目錄。"
binary_target="${user_home}/.local/bin/tailblink"
service_target="${user_home}/.config/systemd/user/tailblink.service"
config_dir="${XDG_CONFIG_HOME:-${user_home}/.config}/tailblink"
state_dir="${XDG_STATE_HOME:-${user_home}/.local/state}/tailblink"

if [[ -x "${binary_target}" ]] && command -v tailscale >/dev/null; then
  sudo "${binary_target}" serve-remove || fail "Serve path 未移除，本機檔案保持不變。"
fi

systemctl --user disable --now tailblink.service >/dev/null 2>&1 || true
rm -f -- "${service_target}"
systemctl --user daemon-reload
rm -f -- "${binary_target}"
# 僅移除 TailBlink 專用且帶 hash 的隧道檔，不碰系統 cloudflared。
for companion in "${user_home}"/.local/bin/tailblink-cloudflared-*; do
  if [[ "$(basename -- "$companion")" =~ ^tailblink-cloudflared-[a-f0-9]{12}$ ]]; then
    rm -f -- "$companion"
  fi
done

case "${config_dir}" in
  */tailblink) rm -rf -- "${config_dir}" ;;
  *) fail "拒絕移除不安全的設定路徑。" ;;
esac
case "${state_dir}" in
  */tailblink) rm -rf -- "${state_dir}" ;;
  *) fail "拒絕移除不安全的日誌路徑。" ;;
esac

printf 'TailBlink 已解除安裝。wl-clipboard 與 Tailscale 保留不動。\n'
