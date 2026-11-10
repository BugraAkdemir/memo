"""Keep real Whisper segment times; correct spelling against the TTS source.

Sequence alignment restores names such as Rust, PyTorch and Candle without
inventing new audio timestamps. The master narration WAV is the caption source.
"""
import bisect
import difflib
import json
import pathlib
import re
import unicodedata

ROOT=pathlib.Path(__file__).resolve().parent.parent
REPO=ROOT.parent.parent
out=REPO/'build_output/rust-ai-video'
timeline=json.loads((ROOT/'src/film-timeline.json').read_text())
asr=json.loads((out/'caption-asr.json').read_text())
expected=' '.join(b['tr'] for b in timeline['beats']).split()
segments=asr['segments']
observed=[];ranges=[]
for segment in segments:
    start=len(observed);observed.extend(segment['text'].split());ranges.append((start,len(observed)))
def norm(word):
    word=word.casefold().replace('ı','i')
    return ''.join(c for c in unicodedata.normalize('NFD',word) if c.isalnum())
matcher=difflib.SequenceMatcher(None,list(map(norm,expected)),list(map(norm,observed)),autojunk=False)
ratio=matcher.ratio()
assert ratio>.80, f'Transcript agreement requires review: {ratio:.3f}'
boundaries={0:0,len(observed):len(expected)}
for tag,i1,i2,j1,j2 in matcher.get_opcodes():
    if j2==j1:
        boundaries[j1]=i2
    else:
        for j in range(j1,j2+1): boundaries[j]=round(i1+(j-j1)*(i2-i1)/(j2-j1))
caps=[]
for segment,(a,b) in zip(segments,ranges):
    words=expected[boundaries[a]:boundaries[b]]
    if not words:continue
    text=' '.join(words)
    start=round(segment['start']*1000);end=round(segment['end']*1000)
    if end<=start:continue
    caps.append({'text':' '+text,'startMs':start,'endMs':end,'timestampMs':None,'confidence':None})
assert ' '.join(c['text'].strip() for c in caps)==' '.join(expected),'Source words lost during alignment'
assert all(caps[i]['endMs']<=caps[i+1]['startMs'] for i in range(len(caps)-1))
(ROOT/'src/captions.json').write_text(json.dumps(caps,ensure_ascii=False,indent=2)+'\n')
def stamp(ms):
    h,ms=divmod(ms,3600000);m,ms=divmod(ms,60000);s,ms=divmod(ms,1000)
    return f'{h:02}:{m:02}:{s:02},{ms:03}'
srt='\n\n'.join(f"{i+1}\n{stamp(c['startMs'])} --> {stamp(c['endMs'])}\n{c['text'].strip()}" for i,c in enumerate(caps))+'\n'
(out/'python-rust-ai-tr.srt').write_text(srt)
print(f'CAPTION_ALIGNMENT {ratio:.3f}; {len(caps)} timed cues; {len(expected)} source words preserved')
print('LONG_CUES',[(i,c['text']) for i,c in enumerate(caps) if len(c['text'])>65])
