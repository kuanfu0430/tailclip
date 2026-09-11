#!/usr/bin/env python3
"""以真正 Cloudflare HTTPS 與 Mac 原生捷徑驗證 B，使用隔離設定與合成資料。"""
import argparse,json,plistlib,subprocess,tempfile,urllib.request
from pathlib import Path
from build_simple import build_simple
from native_test import Suite,ICLOUD
from sign import command
ROOT=Path(__file__).resolve().parent
QA_PATH='TailBlink-QA-Simple-20260907/config.json'
def prepare(output):
 output.mkdir(parents=True,exist_ok=True)
 for d in ('send','pull'):
  workflow=build_simple(d,config_path=QA_PATH)
  workflow['WFWorkflowName']=f'TailBlink-QA-Simple-{d.title()}'
  for a in workflow['WFWorkflowActions']:
   if a['WFWorkflowActionIdentifier'].endswith('.notification'):
    p=a['WFWorkflowActionParameters'];a['WFWorkflowActionIdentifier']='is.workflow.actions.output';a['WFWorkflowActionParameters']={'WFOutput':p['WFNotificationActionBody'],'UUID':p['UUID']}
  source=output/f'{d}-unsigned.shortcut';target=output/f'TailBlink-QA-Simple-{d.title()}.shortcut';source.write_bytes(plistlib.dumps(workflow,fmt=plistlib.FMT_BINARY,sort_keys=True))
  command(['shortcuts','sign','--mode','anyone','--input',str(source),'--output',str(target)])
class SimpleSuite(Suite):
 def __init__(self,args,temp):
  self.args,self.temp=args,Path(temp);self.state=json.loads(args.state.read_text());self.config=ICLOUD/QA_PATH;self.config.parent.mkdir(parents=True,exist_ok=True);self.pairing=self.state['pairing'];self.passed=[]
 def renew(self):
  urllib.request.urlopen(urllib.request.Request(self.state['url']+'/pairing',data=b''),timeout=5).close();self.state=json.loads(self.args.state.read_text());self.pairing=self.state['pairing']
 def run(self):
  self.config.unlink(missing_ok=True);self.fixture('配對取回 🐾');self.clipboard('手機保留')
  assert '已從' in self.run_shortcut('pull',json.dumps(self.pairing));assert self.clipboard()=='配對取回 🐾';saved=json.loads(self.config.read_text());assert len(saved['token'])==43 and saved['base_url']==self.pairing['base_url'];self.passed.append('配對、保存讀回與同次取回');print('PASS '+self.passed[-1],flush=True)
  def send(text,share=True):
   self.fixture('桌面原文');self.clipboard(json.dumps(self.pairing) if share else text)
   assert '已傳到' in self.run_shortcut('send',text if share else None);assert self.fixture()['text']==text
  self.check('分享優先於剪貼簿已使用票券',lambda:send('中文 🐾\n"引號" \\ CRLF\r\n'))
  self.check('剪貼簿再次傳送',lambda:send('再次傳送 🐾',False))
  self.check('1 MiB 傳送',lambda:send('🐾'*(1048576//4)))
  self.fixture('保留桌面');self.clipboard('保留手機');assert '超過 1 MiB' in self.run_shortcut('send','a'*1048577);assert self.fixture()['text']=='保留桌面';self.passed.append('超量不改寫桌面')
  for text in ('取回多行 🐾\n第二行',''):
   self.fixture(text);self.clipboard('手機原文');message=self.run_shortcut('pull');assert ('已從' if text else '沒有文字') in message;assert self.clipboard()==(text or '手機原文')
  self.passed.append('取回與空值保留手機')
  before=self.config.read_bytes();self.clipboard('保持原文');assert '配對已失效' in self.run_shortcut('pull',json.dumps(self.pairing));assert self.config.read_bytes()==before and self.clipboard()=='保持原文';self.passed.append('重放票券保留設定與剪貼簿')
  self.renew();self.fixture('桌面原文');self.clipboard(json.dumps(self.pairing));assert '配對完成' in self.run_shortcut('send');assert self.fixture()['text']=='桌面原文';self.passed.append('備援剪貼簿配對只配對不傳送')
  assert self.clipboard()==''
  self.renew();self.fixture('');self.clipboard(json.dumps(self.pairing));assert '沒有文字' in self.run_shortcut('pull');assert self.clipboard()==''
  self.fixture('空取回後再次成功');assert '已從' in self.run_shortcut('pull');assert self.clipboard()=='空取回後再次成功';self.passed.append('備援配對空取回後不重放已消耗票券')
  self.renew();self.fixture('');self.clipboard('連結配對保留手機原文');assert '沒有文字' in self.run_shortcut('pull',json.dumps(self.pairing));assert self.clipboard()=='連結配對保留手機原文';self.passed.append('連結配對空取回保留原有文字')
  self.clipboard('');assert '沒有可傳送' in self.run_shortcut('send');self.passed.append('空傳送停止')
  saved=json.loads(self.config.read_text());self.fixture('桌面原文');self.clipboard('保留');assert '配對資料' in self.run_shortcut('send',json.dumps(saved));assert self.fixture()['text']=='桌面原文';self.passed.append('憑證不作一般文字傳送')
  for key,value in [('base_url','https://evil.example/v1'),('version',2),('token','')]:
   self.config.write_text(json.dumps({**saved,key:value}));self.clipboard('保留');assert '設定無效' in self.run_shortcut('pull');assert self.clipboard()=='保留'
  self.passed.append('無效設定在 HTTP 前拒絕');self.config.write_text(json.dumps(saved))
  urllib.request.urlopen(urllib.request.Request(self.state['url']+'/revoke',data=b''),timeout=5).close();self.clipboard('保留');assert '配對已失效' in self.run_shortcut('pull');assert self.clipboard()=='保留';self.passed.append('撤銷配對後拒絕且保留剪貼簿')
def main():
 p=argparse.ArgumentParser();p.add_argument('mode',choices=['prepare','run']);p.add_argument('--output',type=Path,default=ROOT.parent/'build/shortcuts/simple-native');p.add_argument('--state',type=Path,default=ROOT.parent/'build/simple-state.json');p.add_argument('--send-name',default='TailBlink-QA-Simple-Send');p.add_argument('--pull-name',default='TailBlink-QA-Simple-Pull');args=p.parse_args()
 if args.mode=='prepare':prepare(args.output);return
 with tempfile.TemporaryDirectory(prefix='tailblink-simple-qa-') as tmp:
  exe=Path(tmp)/'clipboard';backup=Path(tmp)/'clipboard.plist';command(['swiftc',str(ROOT/'testing/clipboard.swift'),'-o',str(exe)]);command([str(exe),'save',str(backup)]);suite=SimpleSuite(args,tmp)
  try:suite.run()
  finally:
   command([str(exe),'restore',str(backup)]);args.output.mkdir(parents=True,exist_ok=True);(args.output/'report.json').write_text(json.dumps({'passed':suite.passed,'scope':'macOS Shortcuts + Cloudflare HTTPS + memory clipboard','not_tested':['physical iPhone','Windows clipboard','Linux Wayland']},ensure_ascii=False,indent=2)+'\n')
if __name__=='__main__':main()
