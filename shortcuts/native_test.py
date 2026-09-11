#!/usr/bin/env python3
"""macOS 原生整合驗收：先 prepare／匯入 QA 捷徑，再以 run 呼叫實際 Go testhost。"""
from __future__ import annotations

import argparse
import json
import plistlib
import subprocess
import tempfile
import urllib.request
from pathlib import Path

from build import ROOT, build
from sign import command

QA_PATH = "TailBlink-QA-20260905/config.json"
QA_PATTERN = r"\Ahttp://127\.0\.0\.1:[0-9]+/tailblink/v1\z"
ICLOUD = Path.home() / "Library/Mobile Documents/iCloud~is~workflow~my~workflows/Documents"


def prepare(output):
    output.mkdir(parents=True, exist_ok=True)
    for direction in ("send", "pull"):
        workflow = build(direction, config_path=QA_PATH, base_pattern=QA_PATTERN, tailscale=False)
        # 僅替換環境依賴：測試路徑、loopback URL、Connect；通知改為回傳可斷言的文字。
        # 驗證、檔案操作、URL、HTTP headers/body、剪貼簿及所有分支仍是原生 production actions。
        for action in workflow["WFWorkflowActions"]:
            if action["WFWorkflowActionIdentifier"].endswith(".notification"):
                params = action["WFWorkflowActionParameters"]
                action["WFWorkflowActionIdentifier"] = "is.workflow.actions.output"
                action["WFWorkflowActionParameters"] = {
                    "WFOutput": params["WFNotificationActionBody"], "UUID": params["UUID"]}
        unsigned = output / f"{direction}-unsigned.shortcut"
        signed = output / f"TailBlink-QA-{direction.title()}.shortcut"
        unsigned.write_bytes(plistlib.dumps(workflow, fmt=plistlib.FMT_BINARY, sort_keys=True))
        command(["shortcuts", "sign", "--mode", "anyone", "--input", str(unsigned), "--output", str(signed)])
        print(f"請匯入：{signed}")


class Suite:
    def __init__(self, args, temp):
        self.args, self.temp = args, Path(temp)
        self.state = json.loads(args.state.read_text())
        self.config = ICLOUD / QA_PATH
        self.config.parent.mkdir(parents=True, exist_ok=True)
        self.pairing = self.state["pairing"]
        self.passed = []

    def fixture(self, text=None):
        req = urllib.request.Request(self.state["url"] + "/fixture")
        if text is not None:
            req.data = json.dumps({"text": text}).encode()
            req.add_header("Content-Type", "application/json")
        with urllib.request.urlopen(req, timeout=10) as response:
            return json.load(response)

    @staticmethod
    def clipboard(text=None):
        if text is not None:
            subprocess.run(["pbcopy"], input=text.encode(), check=True)
        return subprocess.check_output(["pbpaste"]).decode()

    def save_config(self, data=None):
        self.config.write_text(json.dumps(data or self.pairing))

    def run_shortcut(self, direction, share=None):
        result = self.temp / "result.txt"
        result.unlink(missing_ok=True)
        cmd = ["shortcuts", "run", self.args.send_name if direction == "send" else self.args.pull_name,
               "--output-path", str(result)]
        if share is not None:
            file = self.temp / "share.txt"
            file.write_bytes(share.encode())
            cmd.extend(["--input-path", str(file)])
        proc = subprocess.run(cmd, capture_output=True, timeout=60)
        assert proc.returncode == 0, proc.stderr.decode(errors="replace")
        assert result.is_file(), "捷徑沒有回傳分支結果；請檢查原生權限提示"
        return result.read_text()

    def check(self, label, action):
        action()
        self.passed.append(label)
        print("PASS " + label, flush=True)

    def pairing_send(self):
        self.config.unlink(missing_ok=True)
        self.fixture("桌面原文")
        self.clipboard(json.dumps(self.pairing))
        assert "配對完成" in self.run_shortcut("send")
        assert json.loads(self.config.read_text()) == self.pairing
        assert self.clipboard() == ""
        assert self.fixture() == {"text": "桌面原文", "requests": ["GET /tailblink/v1/status"]}

    def send(self, text, share=True):
        self.save_config()
        self.fixture("不應保留")
        self.clipboard("分享輸入應優先" if share else text)
        assert "已傳到" in self.run_shortcut("send", text if share else None)
        assert self.fixture() == {"text": text, "requests": ["POST /tailblink/v1/clipboard/text"]}
        assert self.clipboard() == ("分享輸入應優先" if share else text)

    def pull(self, text):
        self.save_config()
        self.fixture(text)
        self.clipboard("手機原文")
        message = self.run_shortcut("pull")
        assert ("已從" if text else "沒有文字") in message
        assert self.clipboard() == (text or "手機原文")
        assert self.fixture()["requests"] == ["GET /tailblink/v1/clipboard/text"]

    def reject_send(self, text, expected, requests):
        self.save_config()
        self.fixture("桌面原文")
        self.clipboard("")
        message = self.run_shortcut("send", text)
        assert expected in message, message
        assert self.fixture() == {"text": "桌面原文", "requests": requests}

    def invalid_config(self, key, value):
        self.save_config({**self.pairing, key: value})
        self.fixture("桌面原文")
        self.clipboard("手機原文")
        assert "配對資料無效" in self.run_shortcut("pull")
        assert self.fixture()["requests"] == []
        assert self.clipboard() == "手機原文"

    def unauthorized(self, pairing=False):
        invalid = {**self.pairing, "token": "B" * 43}
        self.save_config(self.pairing if pairing else invalid)
        before = self.config.read_bytes()
        self.fixture("桌面原文")
        clip = json.dumps(invalid) if pairing else "手機原文"
        self.clipboard(clip)
        assert "配對已失效" in self.run_shortcut("pull")
        assert self.config.read_bytes() == before
        assert self.clipboard() == clip
        route = "/status" if pairing else "/clipboard/text"
        assert self.fixture() == {"text": "桌面原文", "requests": ["GET /tailblink/v1" + route]}

    def repair_and_pull(self):
        self.config.write_text("損壞的舊設定")
        self.fixture("重新配對後取回 🐾")
        self.clipboard(json.dumps(self.pairing))
        assert "已從" in self.run_shortcut("pull")
        assert self.clipboard() == "重新配對後取回 🐾"
        assert json.loads(self.config.read_text()) == self.pairing
        assert self.fixture()["requests"] == ["GET /tailblink/v1/status", "GET /tailblink/v1/clipboard/text"]

    def run(self):
        self.check("首次配對與 config.json 寫入／讀回，憑證不傳送", self.pairing_send)
        self.check("Share Sheet 中文、Emoji、引號、反斜線、CRLF", lambda: self.send('中文 🐾\n"引號" \\ 跳脫\r\n最後一行'))
        self.check("剪貼簿 fallback", lambda: self.send("剪貼簿文字 🐾", share=False))
        self.check("分享網址保持純文字", lambda: self.send("https://example.com/a?q=中文&x=1#段落"))
        self.check("取回 Unicode 多行文字", lambda: self.pull("Windows → iPhone 🐾\r\n第二行"))
        self.check("空桌面剪貼簿不覆寫手機", lambda: self.pull(""))
        self.check("空傳送不發出請求", lambda: self.reject_send(None, "沒有可傳送", []))
        self.check("分享配對憑證不外送", lambda: self.reject_send(json.dumps(self.pairing), "配對資料", []))
        self.check("1 MiB 傳送", lambda: self.send("🐾" * (1048576 // 4)))
        self.check("超過 1 MiB 不覆寫桌面", lambda: self.reject_send("a" * 1048577, "超過 1 MiB", ["POST /tailblink/v1/clipboard/text"]))
        for key, value in (("base_url", ""), ("base_url", "/status"), ("token", ""), ("version", 2)):
            self.check(f"無效設定 {key}={value!r} 在 HTTP 前停止", lambda k=key, v=value: self.invalid_config(k, v))
        self.check("失效 token 不覆寫手機", self.unauthorized)
        self.check("失敗配對不覆寫已保存設定", lambda: self.unauthorized(pairing=True))
        self.check("損壞設定以新配對資料修復並取回", self.repair_and_pull)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("prepare", "run"))
    parser.add_argument("--output", type=Path, default=ROOT.parent / "build/shortcuts/native")
    parser.add_argument("--state", type=Path, default=ROOT.parent / "build/shortcuts/native/state.json")
    parser.add_argument("--send-name", default="TailBlink-QA-Send")
    parser.add_argument("--pull-name", default="TailBlink-QA-Pull")
    args = parser.parse_args()
    if args.mode == "prepare":
        prepare(args.output)
        return
    with tempfile.TemporaryDirectory(prefix="tailblink-native-") as tmp:
        executable = Path(tmp) / "clipboard"
        command(["swiftc", str(ROOT / "testing/clipboard.swift"), "-o", str(executable)])
        backup = Path(tmp) / "clipboard.plist"
        command([str(executable), "save", str(backup)])
        suite = Suite(args, tmp)
        try:
            suite.run()
        finally:
            command([str(executable), "restore", str(backup)])
            args.output.mkdir(parents=True, exist_ok=True)
            (args.output / "report.json").write_text(json.dumps({
                "passed": suite.passed, "scope": "macOS Shortcuts + real Go API + memory clipboard",
                "not_tested": ["physical iPhone", "Windows native clipboard", "Tailscale network"]
            }, ensure_ascii=False, indent=2) + "\n")


if __name__ == "__main__":
    main()
