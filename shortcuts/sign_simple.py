#!/usr/bin/env python3
"""只簽署兩支簡易捷徑，既有 Tailscale 成品保持原樣。"""
import argparse, hashlib, json, plistlib, tempfile
from pathlib import Path
from build_simple import build_simple
from sign import command, semantic, unpack

ROOT=Path(__file__).resolve().parent

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--verify',action='store_true');args=parser.parse_args()
    manifest={}
    for direction in ('send','pull'):
        workflow=build_simple(direction);raw=plistlib.dumps(workflow,fmt=plistlib.FMT_BINARY,sort_keys=True)
        artifact=ROOT/'dist'/f'TailClip-Simple-{direction.title()}.shortcut'
        if not args.verify:
            with tempfile.TemporaryDirectory() as tmp:
                source=Path(tmp)/'unsigned.shortcut';candidate=Path(tmp)/'signed.shortcut';source.write_bytes(raw)
                command(['shortcuts','sign','--mode','anyone','--input',str(source),'--output',str(candidate)])
                assert semantic(unpack(candidate))==semantic(workflow)
                artifact.write_bytes(candidate.read_bytes())
        assert semantic(unpack(artifact))==semantic(workflow)
        manifest[artifact.name]={'source_sha256':hashlib.sha256(raw).hexdigest(),'artifact_sha256':hashlib.sha256(artifact.read_bytes()).hexdigest(),'actions':len(workflow['WFWorkflowActions'])}
        print('已驗證',artifact.name,len(workflow['WFWorkflowActions']))
    path=ROOT/'dist'/'simple-manifest.json'
    if args.verify:assert json.loads(path.read_text())==manifest
    else:path.write_text(json.dumps(manifest,indent=2)+'\n')
if __name__=='__main__':main()
