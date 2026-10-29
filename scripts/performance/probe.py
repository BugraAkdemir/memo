"""Supplemental serial API/TTFT probe that distinguishes SSE metadata from visible text."""
import json, os, pathlib, time, urllib.request
stage=os.environ.get('BENCH_STAGE') or str(json.loads(pathlib.Path('data/performance-benchmark/awaiting-ui.json').read_text())['stage'])
root=pathlib.Path('data/performance-benchmark');rows=[]
for chat in ['bench-eval','bench-medium','bench-long']:
 req=urllib.request.Request('http://127.0.0.1:18090/api/send/stream',json.dumps({'chat_id':chat,'message':'BENCH_FAKE Supplemental first visible token timing probe.'}).encode(),{'Content-Type':'application/json'})
 t=time.perf_counter();first=None;events=[]
 with urllib.request.urlopen(req,timeout=30) as r:
  for line in r:
   if not line.startswith(b'data: '):continue
   d=json.loads(line[6:]);elapsed=(time.perf_counter()-t)*1000;d['benchmark_elapsed_ms']=elapsed;events.append(d)
   if d.get('content') and d.get('finish_reason','') in ('','stop') and first is None:first=elapsed
 rows.append({'chat':chat,'ttft_visible_text_ms':first,'total_ms':(time.perf_counter()-t)*1000,'events':events})
(root/f'stage-{stage}-supplement.json').write_text(json.dumps(rows,indent=2));print([(x['chat'],round(x['ttft_visible_text_ms'],1)) for x in rows])
