#!/usr/bin/env bash
set -Eeuo pipefail

readonly agent_port="17733"
readonly dashboard_port="17734"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
binary_source="${script_dir}/tailclip"
service_source="${script_dir}/tailclip.service"

fail() {
  printf 'TailClip 安裝失敗：%s\n' "$1" >&2
  exit 1
}

[[ "$(uname -s)" == "Linux" ]] || fail "這支腳本只支援 Ubuntu 26.04。"
[[ "$(uname -m)" == "x86_64" ]] || fail "v0.1-alpha 只提供 x86_64 build。"
[[ -r /etc/os-release ]] || fail "找不到 /etc/os-release。"
# shellcheck disable=SC1091
source /etc/os-release
[[ "${ID:-}" == "ubuntu" && "${VERSION_ID:-}" == "26.04" ]] || fail "需要 Ubuntu 26.04，目前是 ${PRETTY_NAME:-未知系統}。"
[[ -n "${WAYLAND_DISPLAY:-}" ]] || fail "找不到 Wayland 圖形工作階段；請在 GNOME Wayland 登入後執行。"
[[ -n "${XDG_RUNTIME_DIR:-}" ]] || fail "找不到 XDG_RUNTIME_DIR；請從桌面使用者 session 執行。"
[[ -x "${binary_source}" ]] || fail "release 目錄缺少可執行的 tailclip。"
[[ -r "${service_source}" ]] || fail "release 目錄缺少 tailclip.service。"
command -v systemctl >/dev/null || fail "找不到 systemctl。"
command -v curl >/dev/null || fail "找不到 curl。"
command -v ss >/dev/null || fail "找不到 ss；請先安裝 iproute2。"
command -v grep >/dev/null || fail "找不到 grep。"
command -v getent >/dev/null || fail "找不到 getent。"
command -v python3 >/dev/null || fail "找不到 python3。"
command -v sudo >/dev/null || fail "找不到 sudo。"
command -v tailscale >/dev/null || fail "找不到 Tailscale；請先安裝並登入。"

if ! tailscale status --json >/dev/null 2>&1; then
  fail "Tailscale 尚未連線；請先執行 tailscale up 或在應用程式中按 Connect。"
fi

if ! command -v wl-copy >/dev/null || ! command -v wl-paste >/dev/null; then
	command -v apt-get >/dev/null || fail "找不到 apt-get，無法自動安裝 wl-clipboard。"
  printf '正在安裝必要套件 wl-clipboard…\n'
  sudo apt-get install -y wl-clipboard || fail "wl-clipboard 安裝失敗。"
fi

port_in_use() {
  local port="$1"
  ss -H -ltn "sport = :${port}" 2>/dev/null | grep -q .
}

if port_in_use "${agent_port}" && ! curl --silent --fail --max-time 1 "http://127.0.0.1:${agent_port}/v1/health" >/dev/null 2>&1; then
  fail "127.0.0.1:${agent_port} 已被其他程式占用；請先釋放此 port。"
fi
if port_in_use "${dashboard_port}"; then
  fail "127.0.0.1:${dashboard_port} 已被其他程式占用；請先關閉該程式。"
fi

user_name="$(id -un)"
user_home="$(getent passwd "${user_name}" | cut -d: -f6)"
[[ -n "${user_home}" && "${user_home}" == /home/* ]] || fail "無法安全確認使用者家目錄。"
binary_target="${user_home}/.local/bin/tailclip"
service_target="${user_home}/.config/systemd/user/tailclip.service"

install -d -m 0755 "$(dirname -- "${binary_target}")"
install -m 0755 "${binary_source}" "${binary_target}"
install -d -m 0755 "$(dirname -- "${service_target}")"
install -m 0644 "${service_source}" "${service_target}"

systemctl --user import-environment WAYLAND_DISPLAY XDG_RUNTIME_DIR DBUS_SESSION_BUS_ADDRESS >/dev/null 2>&1 || true
systemctl --user daemon-reload
systemctl --user enable --now tailclip.service

for _ in {1..40}; do
  if curl --silent --fail --max-time 1 "http://127.0.0.1:${agent_port}/v1/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.125
done
curl --silent --fail --max-time 1 "http://127.0.0.1:${agent_port}/v1/health" >/dev/null || fail "Agent 未能啟動；請執行 journalctl --user -u tailclip 查看原因。"

dns_name="$(tailscale status --json | python3 -c 'import json,sys; print(json.load(sys.stdin).get("Self",{}).get("DNSName","").rstrip("."))')"
[[ "${dns_name}" == *.ts.net ]] || fail "Tailscale 沒有回報有效的 ts.net 名稱。"
printf '\nTailClip 將為下列 tailnet 位址設定 HTTPS：\n  %s\n\n' "${dns_name}"
printf '完整名稱會出現在公開的 Certificate Transparency 紀錄；服務本身仍只在 tailnet 內可達。\n'
read -r -p '繼續設定 /tailclip Serve path？[y/N] ' answer
case "${answer}" in
  y|Y|yes|YES)
    sudo "${binary_target}" serve-install || fail "Tailscale Serve 或 HTTPS health 驗證失敗。"
    ;;
  *)
    fail "使用者取消 HTTPS 設定；再次執行本腳本即可繼續。"
    ;;
esac

"${binary_target}" open || fail "Agent 已安裝，但無法開啟本機設定頁。請執行 ${binary_target} open。"
printf '\nTailClip 已安裝。掃描畫面上的 QR，即可完成 iPhone 配對。\n'
