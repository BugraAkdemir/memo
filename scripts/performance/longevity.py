#!/usr/bin/env python3
"""Opt-in destructive benchmark fixture runner. Only --replace-test-data permits data replacement.
No load workers: a single backend, serial probes, deterministic seed and bounded real calls.
"""
import argparse, collections, hashlib, http.server, json, math, os, pathlib, random, re
import shutil, signal, socket, sqlite3, statistics, struct, subprocess, threading, time, urllib.request, urllib.error

REPO = pathlib.Path(__file__).resolve().parents[2]
DATA = REPO / 'data'
OUT = DATA / 'performance-benchmark'
NOW = 1793271600  # fixed synthetic timeline, 2026-10-29
QUERIES = [
 ('cedar', 'What is the Cedar recovery code?', 'CEDAR-7294'),
 ('atlas', 'What is the current Atlas deployment region?', 'eu-north-1'),
 ('orchid', 'What is the Orchid invoice currency?', 'CHF'),
 ('unknown', 'What is the Zephyr vault password?', None),
]

def write_json(p, v):
 p.parent.mkdir(parents=True, exist_ok=True)
 p.write_text(json.dumps(v, ensure_ascii=False, indent=2))

def embedding(text):
 """Controlled lexical fixture, NOT a semantic embedding model."""
 v = [0.0] * 768
 for word in re.findall(r'\w+', text.lower()):
  h = hashlib.sha256(word.encode()).digest(); v[int.from_bytes(h[:4], 'little') % 768] += 1
 n = math.sqrt(sum(x*x for x in v)) or 1
 return [x/n for x in v]

def visible_text(event):
 return event.get('content','') if event.get('finish_reason','') in ('','stop') else ''

class Fixture(http.server.BaseHTTPRequestHandler):
 protocol_version = 'HTTP/1.1'
 calls = []; embedding_calls = 0; real_calls = 0; model = ''; max_real = 12
 def log_message(self, *a): pass
 def reply(self, value, status=200):
  b=json.dumps(value).encode();self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
 def do_GET(self):
  if self.path.endswith('/models'): self.reply({'data':[{'id':'benchmark-hash-768','object':'model'}]})
  elif self.path.endswith('/health'): self.reply({'status':'ok'})
  else: self.reply({'status':'ok','model_path':'benchmark-fake-local'})
 def do_POST(self):
  req=json.loads(self.rfile.read(int(self.headers.get('Content-Length','0'))))
  if self.path.endswith('/embeddings'):
   Fixture.embedding_calls+=1;inp=req.get('input','');inp=inp if isinstance(inp,str) else inp[0]
   self.reply({'data':[{'embedding':embedding(inp),'index':0}], 'model':'benchmark-hash-768','usage':{'prompt_tokens':len(inp)//3}});return
  msgs=req.get('messages',[]);last=next((m.get('content','') for m in reversed(msgs) if m.get('role')=='user'),'')
  last=last if isinstance(last,str) else json.dumps(last)
  real=last.startswith('BENCH_REAL_EVAL') and Fixture.model
  row={'time':time.time(),'real':bool(real),'stream':req.get('stream',False),'messages':msgs,'model':req.get('model'),'input_chars':sum(len(json.dumps(m.get('content',''),ensure_ascii=False)) for m in msgs)}
  Fixture.calls.append(row)
  if real:
   if Fixture.real_calls>=Fixture.max_real:self.reply({'error':{'message':'benchmark real-call budget exhausted'}},429);return
   Fixture.real_calls+=1
   st=json.loads((DATA/'cliproxy/state.json').read_text())
   payload={**req,'model':Fixture.model,'max_tokens':300,'temperature':0}
   request=urllib.request.Request(f"http://127.0.0.1:{st['port']}/v1/chat/completions",json.dumps(payload).encode(),{'Authorization':'Bearer '+st['api_key'],'Content-Type':'application/json'})
   try:
    with urllib.request.urlopen(request,timeout=100) as r:
     self.send_response(r.status);self.send_header('Content-Type',r.headers.get('Content-Type','application/json'));self.send_header('Connection','close');self.end_headers();self.close_connection=True
     lines=[]
     for line in r:
      self.wfile.write(line);self.wfile.flush()
      if line.startswith(b'data: '):
       try:
        event=json.loads(line[6:]);
        if event.get('usage'):row['usage']=event['usage']
       except (ValueError,TypeError):pass
      lines.append(line)
     row['response_bytes']=sum(map(len,lines))
   except Exception as e:
    row['error']=str(e);self.close_connection=True
   return
  text='Benchmark fixture response. '+('Stream paragraph with **Markdown** and a short list.\n\n'*240 if 'BENCH_STREAM' in last else 'Synthetic response, not a quality score.')
  usage={'prompt_tokens':row['input_chars']//3,'completion_tokens':len(text)//3,'total_tokens':(row['input_chars']+len(text))//3}
  row['usage']=usage
  if not req.get('stream'):
   self.reply({'id':'fixture','model':'benchmark-fake-local','choices':[{'message':{'role':'assistant','content':text},'finish_reason':'stop'}],'usage':usage});return
  self.send_response(200);self.send_header('Content-Type','text/event-stream');self.send_header('Connection','close');self.end_headers();self.close_connection=True
  try:
   for i in range(0,len(text),64):
    self.wfile.write(('data: '+json.dumps({'choices':[{'delta':{'content':text[i:i+64]},'finish_reason':None}]})+'\n\n').encode());self.wfile.flush()
    if 'BENCH_STREAM' in last:time.sleep(.01)
   self.wfile.write(('data: '+json.dumps({'choices':[{'delta':{},'finish_reason':'stop'}],'usage':usage})+'\n\ndata: [DONE]\n\n').encode());self.wfile.flush()
  except (BrokenPipeError,ConnectionResetError):pass

class WebProxy(http.server.SimpleHTTPRequestHandler):
 def __init__(self,*a,**kw):super().__init__(*a,directory=str(REPO/'frontend/build/web'),**kw)
 def log_message(self,*a):pass
 def do_GET(self):
  if self.path.startswith('/api/'):self.proxy()
  else:super().do_GET()
 def do_POST(self):self.proxy()
 def do_PUT(self):self.proxy()
 def do_DELETE(self):self.proxy()
 def proxy(self):
  body=self.rfile.read(int(self.headers.get('Content-Length','0'))) if self.command!='GET' else None
  req=urllib.request.Request('http://127.0.0.1:18090'+self.path,body,{'Content-Type':'application/json'},method=self.command)
  try:
   with urllib.request.urlopen(req,timeout=120) as r:
    self.send_response(r.status);self.send_header('Content-Type',r.headers.get('Content-Type','application/json'));self.send_header('Connection','close');self.end_headers();self.close_connection=True
    if 'event-stream' in r.headers.get('Content-Type',''):
     for line in r:self.wfile.write(line);self.wfile.flush()
    else:self.wfile.write(r.read())
  except urllib.error.HTTPError as e:
   self.send_response(e.code);self.end_headers();self.wfile.write(e.read())
  except (BrokenPipeError,ConnectionResetError):pass
  except urllib.error.URLError:
   self.send_response(503);self.end_headers()

class Bench:
 def __init__(self,binary):self.binary=binary;self.process=None;self.log=None
 def api(self,path,body=None):
  request=urllib.request.Request('http://127.0.0.1:18090'+path,None if body is None else json.dumps(body).encode(),{'Content-Type':'application/json'})
  with urllib.request.urlopen(request,timeout=120) as r:return json.load(r)
 def start(self,label):
  self.log=open(OUT/(label+'.log'),'w');t=time.perf_counter()
  self.process=subprocess.Popen([self.binary,'-data',str(DATA)],cwd=DATA,stdout=self.log,stderr=self.log,env={**os.environ,'GOMAXPROCS':'2','MEMO_TELEMETRY':'off'},start_new_session=True)
  deadline=time.monotonic()+90
  while time.monotonic()<deadline:
   if self.process.poll() is not None:raise RuntimeError('backend exited: '+str(OUT/(label+'.log')))
   try:self.api('/api/status');break
   except (urllib.error.URLError,TimeoutError):time.sleep(.05)
  else:raise TimeoutError('backend readiness')
  ready=(time.perf_counter()-t)*1000
  while time.monotonic()<deadline:
   if 'Memory store ready' in (OUT/(label+'.log')).read_text():break
   time.sleep(.05)
  else:raise TimeoutError('memory readiness')
  return {'process_to_api_ms':ready,'process_to_memory_ms':(time.perf_counter()-t)*1000,'rss_kb':self.rss()}
 def stop(self):
  if self.process:
   self.process.send_signal(signal.SIGTERM)
   try:self.process.wait(timeout=15)
   except subprocess.TimeoutExpired:os.killpg(self.process.pid,signal.SIGKILL);self.process.wait()
   self.process=None;self.log.close()
 def rss(self):
  return int(next(x.split()[1] for x in pathlib.Path(f'/proc/{self.process.pid}/status').read_text().splitlines() if x.startswith('VmRSS:')))
 def stream(self,chat,message):
  request=urllib.request.Request('http://127.0.0.1:18090/api/send/stream',json.dumps({'chat_id':chat,'message':message}).encode(),{'Content-Type':'application/json'})
  t=time.perf_counter();first=None;events=[];text='';before=len(Fixture.calls);emb=Fixture.embedding_calls
  with urllib.request.urlopen(request,timeout=120) as r:
   for line in r:
    if not line.startswith(b'data: '):continue
    event=json.loads(line[6:]);event['benchmark_elapsed_ms']=(time.perf_counter()-t)*1000;events.append(event)
    if visible_text(event):
     if first is None:first=(time.perf_counter()-t)*1000
     text+=visible_text(event)
  return {'ttft_ms':first,'total_ms':(time.perf_counter()-t)*1000,'text':text,'events':events,'embedding_calls':Fixture.embedding_calls-emb,'provider_calls':Fixture.calls[before:]}

def connect(path):
 db=sqlite3.connect(path,timeout=30);db.execute('PRAGMA journal_mode=WAL');return db

def seed(year,previous):
 """Offline bulk fixture writes only; the actual application is STOPPED here."""
 t=time.perf_counter();days=365*year;old=365*previous
 samples=json.loads((OUT/'ultrachat-sample.json').read_text())['rows']
 sessions=DATA/'sessions';sessions.mkdir(exist_ok=True)
 for day in range(old,days):
  stamp=time.strftime('%Y-%m-%d %H:%M',time.gmtime(NOW-(days-day)*86400))
  for j in range(10):
   i=day*10+j;sample=samples[i%len(samples)]['row']['messages'];msgs=[]
   for k in range(40):
    source=sample[k%len(sample)];role='user' if k%2==0 else 'assistant'
    content=f'Synthetic diary {day}, session {j}, step {k}. '+source['content'][:1200]
    msgs.append({'role':role,'content':content,'timestamp':stamp})
   write_json(sessions/f'bench-{i:05}.json',{'id':f'bench-{i:05}','title':f'Synthetic year diary {i}','created_at':stamp,'updated_at':stamp,'messages':msgs})
  if day%100==0:print('seed sessions day',day,flush=True)
 for name,count in [('bench-long',5000*year),('bench-medium',100),('bench-eval',0)]:
  msgs=[{'role':'user' if k%2==0 else 'assistant','content':f'Historical synthetic turn {k}. '+('Discuss project milestone, database latency and weekly planning. '*16),'timestamp':'2026-10-28 12:00'} for k in range(count)]
  write_json(sessions/(name+'.json'),{'id':name,'title':name,'created_at':'2023-10-30 12:00','updated_at':'2026-10-29 12:00','messages':msgs})
 db=connect(DATA/'memory/memory.db');db.enable_load_extension(True);db.load_extension(str(REPO/'binaries/linux/cpu/vec0.so'))
 template=('Synthetic archive garden travel budget release meeting database recipe cycling music report. '*6)
 rows=[]
 for i in range(old*100,days*100):
  ts=time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime(NOW-(days-i//100)*86400))
  topic=['garden','travel','budget','release','meeting','database','recipe','cycling','music','report'][i%10]
  content=f'Archive item {i} topic {topic}. '+template+f'Project note reference {i%997}.'
  # Mixed pools, including >20k candidates in the benchmark project at stage 3.
  project='benchmark-project' if i%2==0 else ('' if i%3 else 'other-project')
  rows.append((f'noise-{i:07}',content,ts,'Archive question '+topic,'Synthetic archived answer',struct.pack('<768f',*embedding(content)),2+i%3,'conversation',project))
  if len(rows)==250:
   insert_memories(db,rows);rows=[]
 if rows:insert_memories(db,rows)
 # Identical known-answer needles at all stages, both old and recent, in separate scopes.
 for scope in ['', 'benchmark-project','other-project']:
  facts=[('cedar','Cedar recovery code is CEDAR-7294.',3,'2023-10-30T12:00:00Z'),('atlas-old','Atlas deployment region was us-east-1, superseded.',3,'2024-01-01T12:00:00Z'),('atlas','Current Atlas deployment region is eu-north-1, replacing us-east-1.',5,'2026-10-28T12:00:00Z'),('orchid','Orchid invoice currency is CHF.',4,'2025-05-01T12:00:00Z')]
  for key,content,importance,ts in facts:
   uid=f'needle-{scope}-{key}'
   if scope=='other-project':content=content.replace('CEDAR-7294','WRONG-OTHER-PROJECT').replace('CHF','JPY').replace('eu-north-1','ap-east-1')
   # Insert once: old project needles deliberately fall outside the 20k recent candidate window later.
   if db.execute('SELECT 1 FROM memories WHERE uuid=?',(uid,)).fetchone():continue
   insert_memories(db,[(uid,content,ts,content,'',struct.pack('<768f',*embedding(content)),importance,'explicit' if importance==5 else 'conversation',scope)])
 db.execute("INSERT OR REPLACE INTO _metadata VALUES ('vec_migration_done','1')");db.execute("INSERT OR REPLACE INTO _metadata VALUES ('fts_migration_done','1')");db.commit()
 count=db.execute('SELECT count(*) FROM memories').fetchone()[0];vec=db.execute('SELECT count(*) FROM vec_memories').fetchone()[0];fts=db.execute('SELECT count(*) FROM memories_fts').fetchone()[0];assert count==vec==fts,(count,vec,fts)
 db.execute('PRAGMA wal_checkpoint(TRUNCATE)');db.close()
 obs=connect(DATA/'profile/observations.db')
 batch=[]
 for i in range(old*1000,days*1000):
  stamp=time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime(NOW-(days-i//1000)*86400+(i%24)*3600))
  batch.append((stamp,i%7,(i%24)*3600,15,'synthetic-'+str(i%20),['chat','agent','intent','session','whatsapp'][i%5],120,json.dumps({'synthetic':True,'sequence':i})))
  if len(batch)==1000:obs.executemany('INSERT INTO observations(timestamp,day_of_week,time_of_day_seconds,session_length_min,topic,activity_type,word_count,metadata) VALUES (?,?,?,?,?,?,?,?)',batch);batch=[]
 if batch:obs.executemany('INSERT INTO observations(timestamp,day_of_week,time_of_day_seconds,session_length_min,topic,activity_type,word_count,metadata) VALUES (?,?,?,?,?,?,?,?)',batch)
 obs.commit();obs.execute('PRAGMA wal_checkpoint(TRUNCATE)');obs.close()
 stats=connect(DATA/'stats/usage.db')
 stats.executemany('INSERT INTO usage_events(ts,provider,model,prompt_tokens,completion_tokens,duration_secs,tokens_per_second,category) VALUES (?,?,?,?,?,?,?,?)',((NOW-(days-i//200)*86400,'synthetic-provider-'+str(i%12),'fake-local-'+str(i%8),2000+i%12000,300+i%500,5.,60.,['chat','learning','fact_extraction','dream'][i%4]) for i in range(old*200,days*200)))
 stats.commit();stats.execute('PRAGMA wal_checkpoint(TRUNCATE)');stats.close()
 cal=connect(DATA/'calendar/events.db')
 cal.executemany('INSERT OR REPLACE INTO events(id,title,start_time,description,created_at,reminder_sent) VALUES (?,?,?,?,?,1)',((f'bench-event-{i}',f'Synthetic appointment {i}',NOW-(days-i//10)*86400,'Fixture event',NOW-(days-i//10)*86400) for i in range(old*10,days*10)))
 cal.commit();cal.close()
 patterns=[{'id':f'benchmark-{i}','activity_type':f'synthetic-habit-{i}','time_peak_seconds':(i%24)*3600,'std_dev_seconds':1800,'confidence':.8,'days_active':[True]*7,'total_count':i+3,'first_seen':'2023-10-30T12:00:00Z','last_seen':'2026-10-29T12:00:00Z','weighted_score':.7,'declared':True} for i in range(days)]
 write_json(DATA/'profile/patterns.json',{'version':1,'updated_at':'2026-10-29T12:00:00Z','patterns':patterns})
 write_json(DATA/'profile/pending.json',{'id':'benchmark-pending','message':'Synthetic planning suggestion','pattern_id':'benchmark-0','created_at':'2026-10-29T12:00:00Z','responded':False})
 providers=json.loads((DATA/'providers.json').read_text())
 providers['providers']=[p for p in providers['providers'] if not p.get('name','').startswith('benchmark-inactive-')]
 for i in range(12*year):
  providers['providers'].append({'type':'custom','name':f'benchmark-inactive-{i}','base_url':'http://127.0.0.1:18082/v1','model':f'benchmark-fake-local-{i}','enabled':False})
 write_json(DATA/'providers.json',providers)
 for i in range(12*year):
  p=DATA/'models'/f'benchmark-fake-local-{i}.gguf';p.parent.mkdir(exist_ok=True);p.write_bytes(b'BENCHMARK PLACEHOLDER NOT A VALID GGUF\n'+b'\0'*4096)
 db=sqlite3.connect(DATA/'memory/memory.db');assert db.execute('PRAGMA integrity_check').fetchone()[0]=='ok';db.close()
 files=list(sessions.glob('*.json'));size=sum(p.stat().st_size for p in DATA.rglob('*') if p.is_file() and 'performance-benchmark' not in p.parts)
 return {'years':year,'days':days,'sessions':len(files),'messages':days*10*40+5000*year+100,'memories':count,'observations_seeded':days*1000,'usage_events_seeded':days*200,'calendar_events':days*10,'patterns_seeded':days,'fake_models':12*year,'fixture_bytes_including_existing_models':size,'generation_ms':(time.perf_counter()-t)*1000}

def insert_memories(db,rows):
 for r in rows:
  cur=db.execute('INSERT INTO memories(uuid,content,timestamp,user_msg,assist_msg,embedding,importance,source,project_id) VALUES (?,?,?,?,?,?,?,?,?)',r)
  db.execute('INSERT INTO vec_memories(rowid,embedding) VALUES (?,?)',(cur.lastrowid,r[5]))
  db.execute('INSERT INTO memories_fts(rowid,content,user_msg,assist_msg) VALUES (?,?,?,?)',(cur.lastrowid,r[1],r[3],r[4]))
 db.commit()

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--replace-test-data',action='store_true');ap.add_argument('--binary',default='/tmp/memo-performance-run/memo-perf');ap.add_argument('--stages',type=int,default=3);ap.add_argument('--gemini-model',default='gemini-3.8-flash-high');ap.add_argument('--browser-command');args=ap.parse_args()
 if not args.replace_test_data:ap.error('explicit --replace-test-data required')
 if not 1<=args.stages<=3:ap.error('stages must be 1..3')
 OUT.mkdir(exist_ok=True)
 for port in (18080,18082,18083,18084,18090):
  with socket.socket() as sock:sock.bind(('127.0.0.1',port))
 corpus=OUT/'ultrachat-sample.json'
 if not corpus.exists():
  url='https://datasets-server.huggingface.co/rows?dataset=HuggingFaceH4%2Fultrachat_200k&config=default&split=train_sft&offset=0&length=100'
  corpus.write_bytes(urllib.request.urlopen(url,timeout=30).read())
  write_json(OUT/'source.json',{'url':url,'license':'MIT','sha256':hashlib.sha256(corpus.read_bytes()).hexdigest()})
 signal.signal(signal.SIGTERM,lambda *_: (_ for _ in ()).throw(KeyboardInterrupt()))
 cfg=REPO/'config/config.yaml';saved=cfg.read_bytes() if cfg.exists() else None
 backup=OUT/'config-before-run.private'
 if saved is not None:
  backup.write_bytes(saved);backup.chmod(0o600)
 web=http.server.ThreadingHTTPServer(('127.0.0.1',18080),WebProxy);threading.Thread(target=web.serve_forever,daemon=True).start()
 server=http.server.ThreadingHTTPServer(('127.0.0.1',18082),Fixture);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();b=Bench(args.binary);results=[]
 try:
  # Only the user-designated test stores are replaced. Authentication and model weights stay available.
  (DATA/'providers.json').unlink(missing_ok=True)
  (DATA/'telegram.json').unlink(missing_ok=True)
  for name in ['sessions','memory','profile','stats','calendar']:
   p=DATA/name
   if p.exists():shutil.rmtree(p)
  b.start('bootstrap');b.stop()
  for year in range(1,args.stages+1):
   if year>1:
    for name in ['sessions','memory','profile','stats','calendar']:
     shutil.rmtree(DATA/name)
    b.start(f'stage-{year}-bootstrap');b.stop()
   print('STAGE',year,'SEED',flush=True);manifest=seed(year,0);write_json(OUT/f'stage-{year}-manifest.json',manifest)
   stage={'manifest':manifest,'cold':[],'api':{},'fake_turns':[],'real_turns':[]}
   for rep in range(3):
    stage['cold'].append(b.start(f'stage-{year}-cold-{rep}'))
    if rep<2:b.stop()
   for path in ['/api/status','/api/chats','/api/messages','/api/context?chat_id=bench-long','/api/stats/usage']:
    values=[];nbytes=[]
    for _ in range(15):
     t=time.perf_counter();d=b.api(path);values.append((time.perf_counter()-t)*1000);nbytes.append(len(json.dumps(d)))
    stage['api'][path]={'ms':values,'median_ms':statistics.median(values),'max_ms':max(values),'response_bytes_median':statistics.median(nbytes)}
   for chat in ['bench-eval','bench-medium','bench-long']:
    b.api('/api/chats/switch',{'id':chat});before=b.api('/api/context?chat_id='+chat)
    turn=b.stream(chat,'BENCH_FAKE Measure the response overhead.');turn['chat']=chat;turn['context_before']=before;turn['context_after']=b.api('/api/context?chat_id='+chat);stage['fake_turns'].append(turn)
   Fixture.model=args.gemini_model
   for key,q,expected in QUERIES:
    turn=b.stream('bench-eval','BENCH_REAL_EVAL Answer only from remembered personal facts. If the fact is unavailable, say UNKNOWN. Answer in at most two sentences. '+q)
    turn['question_id']=key;turn['expected']=expected;turn['context_after']=b.api('/api/context?chat_id=bench-eval');stage['real_turns'].append(turn)
    print('STAGE',year,'GEMINI',key,'ms',round(turn['total_ms']),'response',turn['text'][:160],flush=True)
   Fixture.model=''
   stage['rss_after_turns_kb']=b.rss()
   if args.browser_command:
    env={**os.environ,'BENCH_STAGE':str(year),'BENCH_OUTPUT':str(OUT)}
    r=subprocess.run(args.browser_command,shell=True,cwd=REPO,env=env,capture_output=True,text=True,timeout=270)
    stage['browser']={'exit_code':r.returncode,'stdout':r.stdout,'stderr':r.stderr}
   b.stop()
   p=subprocess.run([args.binary,'-mode','rag','-data',str(DATA)],input=json.dumps([q for _,q,_ in QUERIES]),capture_output=True,text=True,cwd=DATA,env={**os.environ,'GOMAXPROCS':'2'},timeout=90)
   (OUT/f'stage-{year}-rag.log').write_text(p.stderr)
   if p.returncode:raise RuntimeError(p.stderr[-1500:])
   stage['rag']=json.loads(p.stdout);results.append(stage);write_json(OUT/'results.json',results)
   print('STAGE',year,'COMPLETE','cold_ms',[round(x['process_to_api_ms']) for x in stage['cold']],'memories',manifest['memories'],flush=True)
 finally:
  b.stop();server.shutdown();server.server_close();web.shutdown();web.server_close()
  if saved is None:cfg.unlink(missing_ok=True)
  else:cfg.write_bytes(saved)
  backup.unlink(missing_ok=True)
  write_json(OUT/'provider-calls.json',Fixture.calls)
 print('RESULTS',OUT/'results.json',flush=True)
if __name__=='__main__':main()
