"""Observe runner's bounded UI gate; exercise one browser at a time."""
import json,os,pathlib,subprocess,time
root=pathlib.Path('data/performance-benchmark')
for stage in (2,3):
 deadline=time.monotonic()+600
 while time.monotonic()<deadline:
  try:
   if int(json.loads((root/'awaiting-ui.json').read_text())['stage'])==stage:break
  except (FileNotFoundError,ValueError):pass
  time.sleep(.5)
 else:raise TimeoutError(f'No stage {stage} UI gate')
 env={**os.environ,'BENCH_STAGE':str(stage),'BENCH_OUTPUT':str(root.resolve())}
 subprocess.run(['python3','scripts/performance/probe.py'],env=env,check=True,timeout=90)
 p=subprocess.run(['node','scripts/performance/browser.cjs'],env=env,capture_output=True,text=True,timeout=150)
 (root/f'stage-{stage}-browser-driver.log').write_text(p.stdout+'\n'+p.stderr)
 if p.returncode:
  (root/f'stage-{stage}-ui.json').write_text(json.dumps({'stage':stage,'unavailable':p.stderr[-2000:]}))
 print('UI stage',stage,'exit',p.returncode,flush=True)
