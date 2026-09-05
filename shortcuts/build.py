#!/usr/bin/env python3
"""可重建的 iOS 原生捷徑；只在建置電腦執行 Python，手機不需額外 App。"""
from __future__ import annotations

import argparse
import json
import plistlib
import uuid
from pathlib import Path

ROOT = Path(__file__).resolve().parent
CONFIG_PATH = "TailClip/config.json"
BASE_PATTERN = r"\Ahttps://[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+\.ts\.net/tailclip/v1\z"
TOKEN_PATTERN = r"\A[A-Za-z0-9_-]{43}\z"
# 僅識別配對意圖，真正的欄位驗證在 JSON 解析後進行。
PAIRING_PATTERN = r'(?s)\A\s*\{(?=.*"base_url"\s*:)(?=.*"token"\s*:).*\}\s*\z'
PAIRING_HELP = "配對資料無效或尚未設定。請回到電腦掃描 TailClip QR，按「複製配對資料」，再執行本捷徑。"


def attachment(value: dict) -> dict:
    return {"Value": value, "WFSerializationType": "WFTextTokenAttachment"}


def rich(*parts) -> dict:
    text, attachments = "", {}
    for part in parts:
        if isinstance(part, dict):
            offset = len(text.encode("utf-16-le")) // 2
            attachments[f"{{{offset}, 1}}"] = part["Value"]
            text += "\ufffc"
        else:
            text += str(part)
    value = {"string": text}
    if attachments:
        value["attachmentsByRange"] = attachments
    return {"Value": value, "WFSerializationType": "WFTextTokenString"}


def fields(items: dict) -> dict:
    return {"WFSerializationType": "WFDictionaryFieldValue", "Value": {
        "WFDictionaryFieldValueItems": [
            {"WFItemType": 0, "WFKey": rich(key),
             "WFValue": value if isinstance(value, dict) else rich(value)}
            for key, value in items.items()
        ]}}


class Workflow:
    def __init__(self, direction: str):
        self.direction = direction
        self.actions = []

    def uid(self, label: str) -> str:
        return str(uuid.uuid5(uuid.NAMESPACE_URL, f"tailclip/alpha6/{self.direction}/{label}")).upper()

    def action(self, kind: str, label: str, **params) -> dict:
        identifier = kind if kind.startswith("io.") else "is.workflow.actions." + kind
        params["UUID"] = self.uid(label)
        self.actions.append({"WFWorkflowActionIdentifier": identifier,
                             "WFWorkflowActionParameters": params})
        output_names = {"detect.dictionary": "辭典", "getvalueforkey": "辭典值",
                        "url": "URL", "downloadurl": "URL內容", "conditional": "如果結果",
                        "documentpicker.open": "檔案", "getclipboard": "剪貼板",
                        "text.match": "符合項目"}
        return attachment({"Type": "ActionOutput", "OutputUUID": params["UUID"],
                           "OutputName": output_names.get(kind, "文字")})

    def text(self, label, *parts):
        return self.action("gettext", label, WFTextActionText=rich(*parts))

    def value(self, label, dictionary, key):
        return self.action("getvalueforkey", label, WFInput=dictionary, WFDictionaryKey=key)

    def match(self, label, value, pattern):
        return self.action("text.match", label, text=rich(value), WFMatchTextPattern=pattern,
                           WFMatchTextCaseSensitive=True, ShowWhenRun=False)

    def begin(self, label, value, condition=100):
        self.action("conditional", label, WFInput={"Type": "Variable", "Variable": value},
                    WFCondition=condition, WFControlFlowMode=0,
                    GroupingIdentifier=self.uid(label))

    def otherwise(self, label):
        self.action("conditional", label + "/else", WFControlFlowMode=1,
                    GroupingIdentifier=self.uid(label))

    def end(self, label):
        return self.action("conditional", label + "/end", WFControlFlowMode=2,
                           GroupingIdentifier=self.uid(label))

    def stop(self, label, *message):
        self.action("notification", label + "/notice", WFNotificationActionTitle="TailClip",
                    WFNotificationActionBody=rich(*message), WFNotificationActionSound=False)
        self.action("exit", label + "/stop")

    def require(self, label, value, pattern, message=PAIRING_HELP):
        matches = self.match(label + "/match", value, pattern)
        self.begin(label, matches, 101)
        self.stop(label, message)
        self.end(label)

    def request(self, label, base, token, path, payload=None):
        url_text = self.text(label + "/address", base, path)
        url = self.action("url", label + "/url", WFURLActionURL=rich(url_text))
        params = dict(WFURL=rich(url), WFHTTPMethod="GET", ShowHeaders=True,
                      WFHTTPHeaders=fields({"Authorization": rich("Bearer ", token)}))
        if payload is not None:
            params.update(WFHTTPMethod="POST", WFHTTPBodyType="JSON",
                          WFJSONValues=fields({"text": rich(payload)}))
        response = self.action("downloadurl", label + "/request", **params)
        parsed = self.action("detect.dictionary", label + "/response", WFInput=response)
        ok = self.value(label + "/ok", parsed, "ok")
        # 明確轉為布林，用單一 native boolean 條件，false 分支一律停止。
        boolean = json.loads(json.dumps(ok))
        boolean["Value"]["Aggrandizements"] = [
            {"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFBooleanContentItem"}]
        self.begin(label + "/success", boolean, 4)
        self.action("nothing", label + "/continue")
        self.otherwise(label + "/success")
        error = self.value(label + "/error", parsed, "error.message")
        self.begin(label + "/error-message", error)
        self.stop(label + "/api-error", error)
        self.otherwise(label + "/error-message")
        self.stop(label + "/invalid-response", "電腦回應無效。請確認 Tailscale 與 TailClip 已啟動；若配對失效，請重新掃描 QR 並複製配對資料。")
        self.end(label + "/error-message")
        self.end(label + "/success")
        return parsed


def build(direction: str, *, config_path=CONFIG_PATH, base_pattern=BASE_PATTERN,
          tailscale=True) -> dict:
    if direction not in ("send", "pull"):
        raise ValueError(direction)
    w = Workflow(direction)
    w.action("comment", "about", WFCommentActionText=
             "TailClip alpha.6｜傳送／取回純文字。首次使用請先在電腦配對頁複製配對資料。\n"
             "重新配對：複製新的配對資料再執行。連線錯誤：確認兩台裝置的 Tailscale 與電腦 TailClip 已啟動。")
    clip = w.action("getclipboard", "clipboard")
    clip_text = w.action("detect.text", "clipboard-text", WFInput=clip)
    pairing = w.match("pairing-candidate", clip_text, PAIRING_PATTERN)
    w.begin("choose-config", pairing)
    w.text("new-config", clip_text)
    w.otherwise("choose-config")
    config_file = w.action("documentpicker.open", "load-config", WFGetFilePath=config_path,
                           WFFileErrorIfNotFound=False, WFFileStorageService="iCloud Drive",
                           WFShowFilePicker=False)
    w.begin("missing-config", config_file, 101)
    w.stop("missing-config", PAIRING_HELP)
    w.end("missing-config")
    w.action("detect.text", "saved-config-text", WFInput=config_file)
    config_text = w.end("choose-config")
    # 語法損壞的舊檔也可先從剪貼簿重新配對，無需手動刪檔。
    w.require("config-shape", config_text, PAIRING_PATTERN)
    config = w.action("detect.dictionary", "config", WFInput=config_text)
    version = w.value("version", config, "version")
    w.require("version-valid", version, r"\A1\z")
    base = w.value("base", config, "base_url")
    w.require("base-valid", base, base_pattern)
    token = w.value("token", config, "token")
    w.require("token-valid", token, TOKEN_PATTERN)
    if tailscale:
        w.action("io.tailscale.ipn.ios.ConnectIntent", "connect", ShowWhenRun=False,
                 AppIntentDescriptor={"TeamIdentifier": "W5364U7YZB", "BundleIdentifier": "io.tailscale.ipn.ios",
                                      "Name": "Tailscale", "AppIntentIdentifier": "ConnectIntent"})
        w.action("delay", "connect-wait", WFDelayTime=2)
    w.begin("pair", pairing)
    w.request("status", base, token, "/status")
    # Save File 會依輸入的內容型別改副檔名；先明確命名，否則 config.json 會變 config.txt。
    named_config = w.action("setitemname", "name-config", WFInput=config_text,
                            WFName="config.json", WFDontIncludeFileExtension=False)
    w.action("documentpicker.save", "save-config", WFInput=named_config,
             WFAskWhereToSave=False, WFSaveFileOverwrite=True,
             WFFileStorageService="iCloud Drive", WFFileDestinationPath=config_path)
    saved_config = w.action("documentpicker.open", "verify-saved-config", WFGetFilePath=config_path,
                            WFFileErrorIfNotFound=False, WFShowFilePicker=False)
    w.begin("save-failed", saved_config, 101)
    w.stop("save-failed", "無法保存配對設定。請確認 iCloud Drive 已啟用，並允許捷徑存取 Shortcuts 檔案夾後再試。")
    w.end("save-failed")
    empty = w.text("clear-pairing", "")
    w.action("setclipboard", "clear-clipboard", WFInput=empty, WFLocalOnly=True)
    if direction == "send":
        w.stop("paired", "配對完成。請複製要傳送的文字，或從分享選單再次執行「TailClip：傳送」。")
    w.end("pair")
    if direction == "send":
        shortcut_input = attachment({"Type": "ExtensionInput"})
        w.begin("choose-payload", shortcut_input)
        w.action("detect.text", "shared-text", WFInput=shortcut_input)
        w.otherwise("choose-payload")
        w.text("copied-text", clip_text)
        payload = w.end("choose-payload")
        # 原生 Text 動作即使內容為空仍可能有一個項目；has no value 無法判斷字串長度。
        w.require("empty-payload", payload, r"(?s)\A.+\z",
                  "沒有可傳送的文字。請先複製文字，或從分享選單執行。")
        credential_payload = w.match("credential-payload", payload, PAIRING_PATTERN)
        w.begin("reject-credential", credential_payload)
        w.stop("reject-credential", "這是配對資料，已停止傳送。請回到配對頁複製資料，再直接執行捷徑完成配對。")
        w.end("reject-credential")
        response = w.request("transfer", base, token, "/clipboard/text", payload)
    else:
        response = w.request("transfer", base, token, "/clipboard/text")
        received = w.value("received-text", response, "text")
        w.begin("empty-result", received, 101)
        w.stop("empty-result", "電腦剪貼簿沒有文字。")
        w.end("empty-result")
        w.action("setclipboard", "received-clipboard", WFInput=received, WFLocalOnly=True)
    device = w.value("device-name", response, "device_name")
    w.action("notification", "success", WFNotificationActionTitle="TailClip",
             WFNotificationActionBody=rich("已傳到「" if direction == "send" else "已從「", device,
                                           "」" if direction == "send" else "」取回"),
             WFNotificationActionSound=False)
    return {
        "WFWorkflowName": "TailClip：傳送" if direction == "send" else "TailClip：取回",
        "WFWorkflowClientVersion": "3036.0.4.2", "WFWorkflowMinimumClientVersion": 1106,
        "WFWorkflowMinimumClientVersionString": "1106",
        "WFWorkflowIcon": {"WFWorkflowIconStartColor": 431817727, "WFWorkflowIconGlyphNumber": 61440},
        "WFWorkflowActions": w.actions,
        "WFWorkflowInputContentItemClasses": ["WFStringContentItem", "WFURLContentItem"],
        "WFWorkflowTypes": ["ActionExtension"] if direction == "send" else [],
        "WFWorkflowHasShortcutInputVariables": direction == "send",
        "WFWorkflowImportQuestions": [], "WFQuickActionSurfaces": [],
        "WFWorkflowOutputContentItemClasses": [], "WFWorkflowHasOutputFallback": False,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT.parent / "build" / "shortcuts")
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    for direction in ("send", "pull"):
        path = args.output / f"TailClip-{direction.title()}-unsigned.shortcut"
        path.write_bytes(plistlib.dumps(build(direction), fmt=plistlib.FMT_BINARY, sort_keys=True))
        print(path)


if __name__ == "__main__":
    main()
