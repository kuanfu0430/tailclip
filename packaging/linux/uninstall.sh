#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'TailClip 解除安裝失敗：%s\n' "$1" >&2
  exit 1
}

read -r -p '將移除 TailClip 的 Serve path、自啟、設定與日誌，確定繼續？[y/N] ' answer
case "${answer}" in
  y|Y|yes|YES) ;;
  *) printf '已取消。\n'; exit 0 ;;
esac

user_name="$(id -un)"
user_home="$(getent passwd "${user_name}" | cut -d: -f6)"
[[ -n "${user_home}" && "${user_home}" == /home/* ]] || fail "無法安全確認使用者家目錄。"
binary_target="${user_home}/.local/bin/tailclip"
service_target="${user_home}/.config/systemd/user/tailclip.service"
config_dir="${XDG_CONFIG_HOME:-${user_home}/.config}/tailclip"
state_dir="${XDG_STATE_HOME:-${user_home}/.local/state}/tailclip"

if [[ -x "${binary_target}" ]] && command -v tailscale >/dev/null; then
  sudo "${binary_target}" serve-remove || fail "Serve path 未移除，本機檔案保持不變。"
fi

systemctl --user disable --now tailclip.service >/dev/null 2>&1 || true
rm -f -- "${service_target}"
systemctl --user daemon-reload
rm -f -- "${binary_target}"

case "${config_dir}" in
  */tailclip) rm -rf -- "${config_dir}" ;;
  *) fail "拒絕移除不安全的設定路徑。" ;;
esac
case "${state_dir}" in
  */tailclip) rm -rf -- "${state_dir}" ;;
  *) fail "拒絕移除不安全的日誌路徑。" ;;
esac

printf 'TailClip 已解除安裝。wl-clipboard 與 Tailscale 保留不動。\n'
