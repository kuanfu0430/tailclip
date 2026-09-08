#!/usr/bin/env python3
"""Cloudflare 簡易捷徑；保留原 Tailscale builder 與成品不變。"""
import argparse
import json
import plistlib
from pathlib import Path
from build import Workflow, attachment, rich, fields, TOKEN_PATTERN, build

CONFIG_PATH = 'TailClip-Simple/config.json'
BASE_PATTERN = r'\Ahttps://[a-z0-9]+(?:-[a-z0-9]+)*\.trycloudflare\.com/v1\z'
PAIR_PATTERN = r'(?s)\A\s*\{(?=.*"transport"\s*:\s*"cloudflare")(?=.*"ticket"\s*:).*\}\s*\z'
CONFIG_PATTERN = r'(?s)\A\s*\{(?=.*"base_url"\s*:)(?=.*"transport"\s*:\s*"cloudflare").*\}\s*\z'
HELP = '尚未連接或設定無效。請用相機掃描電腦上的新 QR，按「連接並取回」。'

class SimpleWorkflow(Workflow):
    def uid(self, label):
        return super().uid('simple-v1/' + label)

    def request(self, label, base, token, path, payload=None):
        result = super().request(label, base, token, path, payload)
        if path == '/pair':
            action = next(a for a in self.actions if a['WFWorkflowActionParameters']['UUID'] == self.uid(label+'/request'))
            action['WFWorkflowActionParameters']['WFHTTPHeaders'] = fields({})
            action['WFWorkflowActionParameters']['WFJSONValues'] = fields({'version':'1','ticket':rich(payload)})
        return result


def build_simple(direction, *, config_path=CONFIG_PATH, base_pattern=BASE_PATTERN):
    w=SimpleWorkflow(direction)
    w.action('comment','about',WFCommentActionText='TailClip 簡易連線｜用相機掃電腦 QR 完成連接。重啟隧道後須重掃。無需 Tailscale 或 VPN。')
    share=attachment({'Type':'ExtensionInput'})
    share_text=w.action('detect.text','share-text',WFInput=share)
    clip=w.action('getclipboard','clipboard')
    clip_text=w.action('detect.text','clipboard-text',WFInput=clip)
    w.begin('pair-source',share)
    w.text('pair-share',share_text)
    w.otherwise('pair-source')
    w.text('pair-clip',clip_text)
    candidate=w.end('pair-source')
    pairing=w.match('pairing',candidate,PAIR_PATTERN)
    w.begin('config-source',pairing)
    w.text('new-config',candidate)
    w.otherwise('config-source')
    saved=w.action('documentpicker.open','load-config',WFGetFilePath=config_path,WFFileErrorIfNotFound=False,WFShowFilePicker=False,WFFileStorageService='iCloud Drive')
    w.begin('missing-config',saved,101);w.stop('missing-config',HELP);w.end('missing-config')
    w.action('detect.text','saved-text',WFInput=saved)
    text=w.end('config-source')
    w.require('shape',text,CONFIG_PATTERN,HELP)
    config=w.action('detect.dictionary','config',WFInput=text)
    version=w.value('version',config,'version');w.require('version-check',version,r'\A1\z',HELP)
    transport=w.value('transport',config,'transport');w.require('transport-check',transport,r'\Acloudflare\z',HELP)
    base=w.value('base',config,'base_url');w.require('base-check',base,base_pattern,HELP)
    w.begin('pair',pairing)
    ticket=w.value('ticket',config,'ticket');w.require('ticket-check',ticket,TOKEN_PATTERN,HELP)
    response=w.request('pair-request',base,'','/pair',ticket)
    token=w.value('new-token',response,'token');w.require('new-token-check',token,TOKEN_PATTERN,HELP)
    response_version=w.value('response-version',response,'version');w.require('response-version-check',response_version,r'\A1\z',HELP)
    # 以已驗證的原始目標保存；不採納伺服器回傳的另一個 URL。
    config_text=w.text('paired-config','{"version":1,"transport":"cloudflare","base_url":"',base,'","token":"',token,'"}')
    named=w.action('setitemname','name-config',WFInput=config_text,WFName='config.json',WFDontIncludeFileExtension=False)
    w.action('documentpicker.save','save-config',WFInput=named,WFAskWhereToSave=False,WFSaveFileOverwrite=True,WFFileStorageService='iCloud Drive',WFFileDestinationPath=config_path)
    read=w.action('documentpicker.open','read-config',WFGetFilePath=config_path,WFFileErrorIfNotFound=False,WFShowFilePicker=False)
    w.begin('save-failed',read,101);w.stop('save-failed','無法保存設定，請確認 iCloud Drive 權限，再回電腦產生新 QR。');w.end('save-failed')
    read_dict=w.action('detect.dictionary','read-dictionary',WFInput=read)
    read_token=w.value('read-token',read_dict,'token')
    # 正則全字比對讀回 token；token 已限為 base64url，不含 regex 字元。
    matches=w.action('text.match','readback-match',text=rich(read_token),WFMatchTextPattern=rich(r'\A',token,r'\z'),WFMatchTextCaseSensitive=True,ShowWhenRun=False)
    w.begin('readback-failed',matches,101);w.stop('readback-failed','設定讀回不符，請回電腦產生新 QR 後重試。');w.end('readback-failed')
    # 僅清除成功備援配對所使用的剪貼簿票券，避免空取回後再次交換已消耗票券。
    # 從連結／分享輸入配對時保留手機原有文字；失敗配對亦不修改剪貼簿。
    w.begin('clear-pairing-clipboard',share,101)
    empty=w.text('consumed-ticket','')
    w.action('setclipboard','clear-consumed-ticket',WFInput=empty,WFLocalOnly=True)
    w.end('clear-pairing-clipboard')
    if direction=='send':w.stop('paired','配對完成。請複製文字後執行「TailClip：簡易傳送」。')
    w.text('paired-token',token)
    w.otherwise('pair')
    old_token=w.value('old-token',config,'token');w.require('old-token-check',old_token,TOKEN_PATTERN,HELP)
    w.text('saved-token',old_token)
    token=w.end('pair')
    if direction=='send':
        w.begin('choose-payload',share)
        w.text('shared-payload',share_text)
        w.otherwise('choose-payload')
        w.text('copied-payload',clip_text)
        payload=w.end('choose-payload')
        w.require('empty-payload',payload,r'(?s)\A.+\z','沒有可傳送的文字。')
        # 不傳送兩種入口的配對憑證或本次保存設定。
        secret=w.match('secret',payload,r'(?s)\A\s*\{(?=.*"base_url"\s*:)(?=.*"(?:token|ticket)"\s*:).*\}\s*\z')
        w.begin('reject-secret',secret);w.stop('secret','這是配對資料，已停止傳送。請先複製要傳送的文字。');w.end('reject-secret')
        response=w.request('transfer',base,token,'/clipboard/text',payload)
    else:
        response=w.request('transfer',base,token,'/clipboard/text')
        received=w.value('received',response,'text')
        w.require('empty-result',received,r'(?s)\A.+\z','電腦剪貼簿沒有文字。')
        w.action('setclipboard','received-clipboard',WFInput=received,WFLocalOnly=True)
    device=w.value('device',response,'device_name')
    w.action('notification','success',WFNotificationActionTitle='TailClip',WFNotificationActionBody=rich('已傳到「' if direction=='send' else '已從「',device,'」' if direction=='send' else '」取回'),WFNotificationActionSound=False)
    workflow=build(direction)
    workflow['WFWorkflowName']='TailClip：簡易傳送' if direction=='send' else 'TailClip：簡易取回'
    workflow['WFWorkflowActions']=w.actions
    workflow['WFWorkflowHasShortcutInputVariables']=True
    for a in w.actions:
        p=a['WFWorkflowActionParameters']
        if a['WFWorkflowActionIdentifier'].endswith('.notification') and p.get('UUID') in [w.uid(x+'/invalid-response/notice') for x in ('pair-request','transfer')]:
            p['WFNotificationActionBody']=rich('電腦回應無效，請確認 TailClip 隧道仍運作；重啟後請掃描新 QR。')
    return workflow

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=Path('build/shortcuts/simple'));args=parser.parse_args();args.output.mkdir(parents=True,exist_ok=True)
    for direction in ('send','pull'):
        (args.output/f'TailClip-Simple-{direction.title()}-unsigned.shortcut').write_bytes(plistlib.dumps(build_simple(direction),fmt=plistlib.FMT_BINARY,sort_keys=True))
