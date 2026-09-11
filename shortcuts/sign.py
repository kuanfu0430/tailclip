#!/usr/bin/env python3
"""用 Apple CLI 簽署，解封驗證實際內容，再記錄來源與成品的 SHA-256。"""
from __future__ import annotations

import argparse
import hashlib
import json
import plistlib
import struct
import subprocess
import tempfile
from pathlib import Path

from build import ROOT, build


def semantic(workflow):
    # Apple 簽署器會移除顯示名稱、重寫 client build；實際動作及輸入範圍必須完全相同。
    return {key: value for key, value in workflow.items()
            if key not in ("WFWorkflowName", "WFWorkflowClientVersion")}


def command(args, data=None):
    result = subprocess.run(args, input=data, capture_output=True, timeout=60)
    if result.returncode:
        raise RuntimeError(f"{args[0]} 失敗：{result.stderr.decode(errors='replace')}")
    return result.stdout


def unpack(path: Path) -> dict:
    data = path.read_bytes()
    if data[:4] != b"AEA1":
        raise ValueError("不是已簽署的 Apple Archive")
    size = struct.unpack("<I", data[8:12])[0]
    certificate = plistlib.loads(data[12:12 + size])["SigningCertificateChain"][0]
    pem = command(["openssl", "x509", "-inform", "DER", "-pubkey", "-noout"], certificate)
    der = command(["openssl", "pkey", "-pubin", "-outform", "DER"], pem)
    public_key = der[-65:]
    if len(public_key) != 65 or public_key[0] != 4:
        raise ValueError("不支援的簽章公鑰")
    with tempfile.TemporaryDirectory(prefix="tailblink-sign-") as tmp:
        archive = Path(tmp) / "payload.aar"
        output = Path(tmp) / "output"
        output.mkdir()
        command(["aea", "decrypt", "-i", str(path), "-o", str(archive),
                 "-sign-pub-value", "hex:" + public_key.hex()])
        command(["aa", "extract", "-i", str(archive), "-d", str(output)])
        return plistlib.loads((output / "Shortcut.wflow").read_bytes())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify", action="store_true", help="只驗證，不重新簽署")
    args = parser.parse_args()
    manifest = {}
    for direction in ("send", "pull"):
        workflow = build(direction)
        raw = plistlib.dumps(workflow, fmt=plistlib.FMT_BINARY, sort_keys=True)
        artifact = ROOT / "dist" / f"TailBlink-{direction.title()}.shortcut"
        if not args.verify:
            with tempfile.TemporaryDirectory(prefix="tailblink-build-") as tmp:
                unsigned = Path(tmp) / artifact.name
                unsigned.write_bytes(raw)
                candidate = Path(tmp) / "signed.shortcut"
                command(["shortcuts", "sign", "--mode", "anyone", "--input", str(unsigned),
                         "--output", str(candidate)])
                if semantic(unpack(candidate)) != semantic(workflow):
                    raise ValueError(f"簽署後內容與來源不同：{artifact.name}")
                artifact.write_bytes(candidate.read_bytes())
        if semantic(unpack(artifact)) != semantic(workflow):
            raise ValueError(f"成品已落後 builder：{artifact.name}")
        manifest[artifact.name] = {
            "source_sha256": hashlib.sha256(raw).hexdigest(),
            "artifact_sha256": hashlib.sha256(artifact.read_bytes()).hexdigest(),
            "actions": len(workflow["WFWorkflowActions"]),
        }
        print(f"已驗證 {artifact.name}：{manifest[artifact.name]['actions']} 個原生動作")
    path = ROOT / "dist" / "manifest.json"
    if args.verify:
        if json.loads(path.read_text()) != manifest:
            raise ValueError("manifest 不符")
    else:
        path.write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    main()
