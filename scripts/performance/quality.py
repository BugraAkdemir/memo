"""Three bounded Gemini calls, same held-out questions, real project-scoped retrieval output.
The expected answers are only used by the scorer; they are NOT included in the model prompt.
"""
import json,pathlib,time,urllib.request
from longevity import QUERIES,DATA,OUT
MODEL='gemini-3.8-flash-high'
answers=[]
for year in (1,2,3):
 deadline=time.monotonic()+600
 while time.monotonic()<deadline:
  p=OUT/'results.json'
  rows=json.loads(p.read_text()) if p.exists() else []
  matches=[r for r in rows if r['manifest']['years']==year]
  if matches:break
  time.sleep(.5)
 else:raise TimeoutError('missing benchmark stage')
 stage=matches[0];contexts=[]
 for key,q,expected in QUERIES:
  rag=next(x for x in stage['rag'] if x['scope']=='project' and x['query']==q)
  contexts.append({'question_id':key,'question':q,'retrieved_memory':[x['content'] for x in rag['results']]})
 prompt='Answer each question using ONLY its retrieved_memory. Return a JSON object keyed by question_id; values must be the exact code, region or currency, or UNKNOWN if absent. Prefer current facts over explicitly superseded facts. Do not use tools.\n'+json.dumps(contexts)
 st=json.loads((DATA/'cliproxy/state.json').read_text())
 payload={'model':MODEL,'messages':[{'role':'user','content':prompt}],'max_tokens':400,'temperature':0,'stream':False}
 req=urllib.request.Request(f"http://127.0.0.1:{st['port']}/v1/chat/completions",json.dumps(payload).encode(),{'Content-Type':'application/json','Authorization':'Bearer '+st['api_key']})
 t=time.perf_counter()
 try:
  with urllib.request.urlopen(req,timeout=90) as r:reply=json.load(r)
  text=reply['choices'][0]['message']['content'];clean=text.strip()
  if clean.startswith('```'):clean=clean.split('\n',1)[1].rsplit('```',1)[0]
  parsed=json.loads(clean)
  score={key:str(parsed.get(key,'')).strip().casefold()==(expected or 'UNKNOWN').casefold() for key,q,expected in QUERIES}
  result={'year':year,'model':MODEL,'duration_ms':(time.perf_counter()-t)*1000,'response':text,'answers':parsed,'checks':score,'correct':sum(score.values()),'total':len(score),'usage':reply.get('usage'),'retrieval_scope':'benchmark-project','contexts':contexts}
 except Exception as e:result={'year':year,'error':str(e),'duration_ms':(time.perf_counter()-t)*1000}
 answers.append(result);(OUT/'quality.json').write_text(json.dumps(answers,indent=2));print('quality',year,result.get('correct'),result.get('answers',result.get('error')),flush=True)
