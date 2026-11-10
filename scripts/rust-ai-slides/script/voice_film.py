"""Generate real ElevenLabs narration; never estimate missing audio as success."""
import concurrent.futures
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import urllib.error
import urllib.request

ROOT=pathlib.Path(__file__).resolve().parent.parent
spec=json.loads((ROOT/'script/narration.json').read_text())
key=pathlib.Path(os.environ['ELEVEN_KEY_FILE']).read_text().strip()
out=ROOT/'public/voice/tr';out.mkdir(parents=True,exist_ok=True)
def work(beat):
    p=out/f"{beat['id']}.mp3"
    digest=hashlib.sha256((beat['tr']+spec['voices'][beat['role']]).encode()).hexdigest()
    marker=p.with_suffix('.sha256')
    if not p.exists() or (beat['id']!='intro' and (not marker.exists() or marker.read_text()!=digest)):
        req=urllib.request.Request(
            f"https://api.elevenlabs.io/v1/text-to-speech/{spec['voices'][beat['role']]}?output_format=mp3_44100_128",
            data=json.dumps({'text':beat['tr'],'model_id':'eleven_multilingual_v2','voice_settings':{'stability':0.5,'similarity_boost':0.8,'style':0.2,'use_speaker_boost':True}}).encode(),
            headers={'xi-api-key':key,'Content-Type':'application/json','Accept':'audio/mpeg'})
        try:
            with urllib.request.urlopen(req,timeout=120) as r: data=r.read()
        except urllib.error.HTTPError as e:
            try:
                detail=json.loads(e.read()).get('detail',{})
                status=detail.get('status','unknown')
            except Exception:status='unknown'
            raise RuntimeError(f"{beat['id']}: HTTP {e.code}, status {status}") from None
        p.write_bytes(data);marker.write_text(digest)
    sec=float(subprocess.check_output(['ffprobe','-v','error','-show_entries','format=duration','-of','csv=p=0',str(p)]))
    return beat['id'],{'sec':sec,'ok':True,'role':beat['role']}
result={}
try:
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        futures={pool.submit(work,b):b['id'] for b in spec['beats']}
        for future in concurrent.futures.as_completed(futures):
            ident,data=future.result();result[ident]=data
            print(f"{len(result)}/{len(spec['beats'])} {ident}: {data['sec']:.2f}s",flush=True)
except Exception as e:
    print('GENERATION_STOPPED:',str(e),file=sys.stderr);sys.exit(1)
(ROOT/'src/voice-film.json').write_text(json.dumps(result,indent=2)+'\n')
total=sum(x['sec'] for x in result.values())
print(f"VOICE_COMPLETE {len(result)}/{len(spec['beats'])}; {total:.2f}s real audio",flush=True)
assert total>=360,'Narration below required minimum; expand content before rendering.'
