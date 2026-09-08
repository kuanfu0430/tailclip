"""簡易捷徑的可重建性、授權邊界與分享優先回歸。"""
import hashlib,json,plistlib,re,sys,unittest
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
from build_simple import build_simple,SimpleWorkflow,BASE_PATTERN,CONFIG_PATH
from test_build import walk
class SimpleTests(unittest.TestCase):
 def test_references_and_explicit_dictionary(self):
  for direction in ('send','pull'):
   actions=build_simple(direction)['WFWorkflowActions'];seen={};groups=[]
   for a in actions:
    p=a['WFWorkflowActionParameters'];kind=a['WFWorkflowActionIdentifier']
    for value in walk(p):
     if value.get('Type')=='ActionOutput':self.assertIn(value['OutputUUID'],seen)
    if kind.endswith('.getvalueforkey'):self.assertTrue(seen[p['WFInput']['Value']['OutputUUID']]['WFWorkflowActionIdentifier'].endswith('.detect.dictionary'))
    self.assertNotIn(p['UUID'],seen);seen[p['UUID']]=a
    if kind.endswith('.conditional'):
     if p['WFControlFlowMode']==0:groups.append(p['GroupingIdentifier'])
     else:
      self.assertEqual(groups[-1],p['GroupingIdentifier'])
      if p['WFControlFlowMode']==2:groups.pop()
    self.assertNotIn('tailscale',kind.lower())
   self.assertEqual(groups,[])
 def test_share_input_wins_over_stale_clipboard_ticket(self):
  for d in ('send','pull'):
   w=SimpleWorkflow(d);a=next(a for a in build_simple(d)['WFWorkflowActions'] if a['WFWorkflowActionParameters']['UUID']==w.uid('pair-source'))
   self.assertIn('ExtensionInput',str(a['WFWorkflowActionParameters']['WFInput']))
 def test_signed_source_manifest(self):
  root=Path(__file__).resolve().parents[1];manifest=json.loads((root/'dist/simple-manifest.json').read_text())
  for d in ('send','pull'):
   name=f'TailClip-Simple-{d.title()}.shortcut';raw=plistlib.dumps(build_simple(d),fmt=plistlib.FMT_BINARY,sort_keys=True)
   self.assertEqual(hashlib.sha256(raw).hexdigest(),manifest[name]['source_sha256'])
   self.assertEqual(hashlib.sha256((root/'dist'/name).read_bytes()).hexdigest(),manifest[name]['artifact_sha256'])
 def test_consumed_clipboard_ticket_cleared_only_after_verified_pairing(self):
  for direction in ('send','pull'):
   w=SimpleWorkflow(direction);actions=build_simple(direction)['WFWorkflowActions']
   index={a['WFWorkflowActionParameters']['UUID']:i for i,a in enumerate(actions)}
   gate=actions[index[w.uid('clear-pairing-clipboard')]]['WFWorkflowActionParameters']
   self.assertEqual(gate['WFCondition'],101)
   self.assertIn('ExtensionInput',str(gate['WFInput']))
   clear=actions[index[w.uid('clear-consumed-ticket')]]
   self.assertEqual(clear['WFWorkflowActionIdentifier'],'is.workflow.actions.setclipboard')
   self.assertTrue(clear['WFWorkflowActionParameters']['WFLocalOnly'])
   self.assertLess(index[w.uid('readback-match')],index[w.uid('clear-pairing-clipboard')])
   self.assertLess(index[w.uid('readback-failed/end')],index[w.uid('clear-consumed-ticket')])
   self.assertLess(index[w.uid('clear-consumed-ticket')],index[w.uid('transfer/request')])
 def test_boundary_and_isolation(self):
  pattern=BASE_PATTERN.replace(r'\z',r'\Z');good='https://quiet-river.trycloudflare.com/v1'
  self.assertIsNotNone(re.fullmatch(pattern,good))
  for bad in (good+'/',good+'?x=1',good.replace('https:','http:'),good.replace('.com/','.com.evil/'),good.replace('quiet-river','user@quiet-river'),good.replace('.com/', '.com:443/')):self.assertIsNone(re.fullmatch(pattern,bad))
  self.assertNotEqual(CONFIG_PATH,'TailClip/config.json')
if __name__=='__main__':unittest.main()
