"""Replay UI/timing measurements without repeating real provider calls."""
import json,os,shutil,subprocess,threading,time
from longevity import DATA,OUT,Fixture,WebProxy,Bench,seed,http
web=http.server.ThreadingHTTPServer(('127.0.0.1',18080),WebProxy);threading.Thread(target=web.serve_forever,daemon=True).start()
fake=http.server.ThreadingHTTPServer(('127.0.0.1',18082),Fixture);threading.Thread(target=fake.serve_forever,daemon=True).start()
b=Bench('/tmp/memo-performance-run/memo-perf')
try:
 years=[int(x) for x in os.environ.get('BENCH_REPLAY_STAGES','1,2,3').split(',')]
 for year in years:
  if os.environ.get('BENCH_REPLAY_KEEP_DATA')!='1':
   for name in ['sessions','memory','profile','stats','calendar']:shutil.rmtree(DATA/name)
   b.start(f'ui-replay-{year}-bootstrap');b.stop();seed(year,0)
  b.start(f'ui-replay-{year}')
  env={**os.environ,'BENCH_STAGE':str(year),'BENCH_OUTPUT':str(OUT)}
  subprocess.run(['python3','scripts/performance/probe.py'],env=env,check=True,timeout=90)
  p=subprocess.run(['node','scripts/performance/browser.cjs'],env=env,capture_output=True,text=True,timeout=150)
  (OUT/f'stage-{year}-ui-replay-driver.log').write_text(p.stdout+'\n'+p.stderr)
  if p.returncode:raise RuntimeError(p.stderr[-1500:])
  d=json.loads((OUT/f'stage-{year}-ui.json').read_text());print('UI replay',year,'persisted; visual review pending',d.get('response_persisted'),flush=True)
  if not d.get('response_persisted'):raise RuntimeError('Response not persisted; UI measurement incomplete')
  b.stop()
finally:
 b.stop();fake.shutdown();web.shutdown()
