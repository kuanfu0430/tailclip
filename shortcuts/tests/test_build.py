"""檢查可追溯的成品及脆弱的原生資料流；實際執行另由 native_test.py 驗證。"""
import hashlib
import json
import plistlib
import re
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from build import BASE_PATTERN, CONFIG_PATH, ROOT, TOKEN_PATTERN, build, rich


def walk(value):
    if isinstance(value, dict):
        yield value
        for item in value.values():
            yield from walk(item)
    elif isinstance(value, list):
        for item in value:
            yield from walk(item)


class BuildTests(unittest.TestCase):
    def test_all_references_point_back_to_real_actions_and_conditions_balance(self):
        for direction in ("send", "pull"):
            seen, groups = set(), []
            for action in build(direction)["WFWorkflowActions"]:
                params = action["WFWorkflowActionParameters"]
                for item in walk(params):
                    if item.get("Type") == "ActionOutput":
                        self.assertIn(item["OutputUUID"], seen)
                    self.assertNotIn("VariableName", item, "命名變數不得藏在文字 token 中")
                self.assertNotIn(params["UUID"], seen)
                seen.add(params["UUID"])
                if action["WFWorkflowActionIdentifier"].endswith(".conditional"):
                    mode, group = params["WFControlFlowMode"], params["GroupingIdentifier"]
                    if mode == 0:
                        self.assertIn(params["WFCondition"], (4, 100, 101))
                        self.assertIn("WFInput", params)
                        groups.append(group)
                    else:
                        self.assertEqual(groups[-1], group)
                        if mode == 2:
                            groups.pop()
            self.assertEqual(groups, [])

    def test_dictionary_url_and_network_inputs_are_explicit(self):
        for direction in ("send", "pull"):
            actions = build(direction)["WFWorkflowActions"]
            by_id = {a["WFWorkflowActionParameters"]["UUID"]: a for a in actions}
            for index, action in enumerate(actions):
                kind, params = action["WFWorkflowActionIdentifier"], action["WFWorkflowActionParameters"]
                if kind.endswith(".getvalueforkey"):
                    source = by_id[params["WFInput"]["Value"]["OutputUUID"]]
                    self.assertTrue(source["WFWorkflowActionIdentifier"].endswith(".detect.dictionary"))
                if kind.endswith(".text.match"):
                    # 本機 Apple action metadata 的輸入鍵是小寫 text，WFText 會被忽略。
                    self.assertEqual(params["text"]["WFSerializationType"], "WFTextTokenString")
                    self.assertFalse(params["ShowWhenRun"])
                if kind.endswith(".downloadurl"):
                    source_uuid = params["WFURL"]["Value"]["attachmentsByRange"]["{0, 1}"]["OutputUUID"]
                    self.assertTrue(by_id[source_uuid]["WFWorkflowActionIdentifier"].endswith(".url"))
                    self.assertTrue(actions[index + 1]["WFWorkflowActionIdentifier"].endswith(".detect.dictionary"))
                    if params["WFHTTPMethod"] == "POST":
                        self.assertEqual(params["WFHTTPBodyType"], "JSON")

    def test_pairing_save_has_filename_and_readback_before_clearing(self):
        for direction in ("send", "pull"):
            actions = build(direction)["WFWorkflowActions"]
            index = next(i for i, a in enumerate(actions) if a["WFWorkflowActionIdentifier"].endswith(".documentpicker.save"))
            name, save, read = (a["WFWorkflowActionParameters"] for a in actions[index - 1:index + 2])
            self.assertEqual(name["WFName"], "config.json")
            self.assertFalse(name["WFDontIncludeFileExtension"])
            self.assertEqual(save["WFInput"]["Value"]["OutputUUID"], name["UUID"])
            self.assertEqual(save["WFFileDestinationPath"], CONFIG_PATH)
            self.assertEqual(read["WFGetFilePath"], CONFIG_PATH)
            self.assertFalse(save["WFAskWhereToSave"])

    def test_url_boundary_and_token_format(self):
        pattern = BASE_PATTERN.replace(r"\z", r"\Z")
        good = "https://work-pc.example.ts.net/tailclip/v1"
        self.assertIsNotNone(re.fullmatch(pattern, good))
        for bad in ("", "/status", "https://ts.net/tailclip/v1", good + "\n", good + "/",
                    good + "?redirect=evil", good.replace("https:", "http:"),
                    good.replace("example.ts.net", "example.ts.net.evil.com"),
                    good.replace("work-pc", "user@work-pc"), good.replace("work-pc", "-bad"),
                    good.replace(".ts.net", ".ts.net:1234")):
            self.assertIsNone(re.fullmatch(pattern, bad), bad)
        token_pattern = TOKEN_PATTERN.replace(r"\z", r"\Z")
        self.assertIsNotNone(re.fullmatch(token_pattern, "A" * 43))
        for bad in ("", "A" * 42, "A" * 44, "A" * 42 + "=", "A" * 43 + "\n"):
            self.assertIsNone(re.fullmatch(token_pattern, bad))

    def test_ios_actions_and_clipboard_local_only(self):
        for direction in ("send", "pull"):
            workflow = build(direction)
            self.assertEqual(workflow["WFWorkflowInputContentItemClasses"], ["WFStringContentItem", "WFURLContentItem"])
            for action in workflow["WFWorkflowActions"]:
                kind, params = action["WFWorkflowActionIdentifier"], action["WFWorkflowActionParameters"]
                self.assertFalse(any(word in kind for word in ("applescript", "shellscript", "javascript", "ssh")))
                if kind.startswith("io."):
                    self.assertEqual(kind, "io.tailscale.ipn.ios.ConnectIntent")
                if kind.endswith(".setclipboard"):
                    self.assertTrue(params["WFLocalOnly"])

    def test_token_ranges_use_utf16_offsets(self):
        token = {"Value": {"Type": "ExtensionInput"}, "WFSerializationType": "WFTextTokenAttachment"}
        value = rich("測試🐾", token)["Value"]
        self.assertEqual(list(value["attachmentsByRange"]), ["{4, 1}"])

    def test_signed_artifacts_match_current_sources(self):
        manifest = json.loads((ROOT / "dist" / "manifest.json").read_text())
        for direction in ("send", "pull"):
            workflow = build(direction)
            filename = f"TailClip-{direction.title()}.shortcut"
            source = plistlib.dumps(workflow, fmt=plistlib.FMT_BINARY, sort_keys=True)
            artifact = (ROOT / "dist" / filename).read_bytes()
            self.assertEqual(artifact[:4], b"AEA1")
            self.assertEqual(manifest[filename]["source_sha256"], hashlib.sha256(source).hexdigest())
            self.assertEqual(manifest[filename]["artifact_sha256"], hashlib.sha256(artifact).hexdigest())
            self.assertEqual(manifest[filename]["actions"], len(workflow["WFWorkflowActions"]))


if __name__ == "__main__":
    unittest.main()
