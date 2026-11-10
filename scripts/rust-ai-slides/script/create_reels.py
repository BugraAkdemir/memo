"""Cut existing approved film into four vertical edits; reuse its actual audio."""
import json,subprocess,pathlib,math
ROOT=pathlib.Path(__file__).resolve().parents[3]
PROJECT=ROOT/'scripts/rust-ai-slides'
OUT=ROOT/'build_output/marketing/python-rust-reels'
SOURCE=ROOT/'build_output/marketing/python-rust-ai-tr-1080p.mp4'
TIMELINE=json.loads((PROJECT/'src/film-timeline.json').read_text())['beats']
CAPTIONS=json.loads((PROJECT/'src/captions.json').read_text())
BY_ID={x['id']:x for x in TIMELINE}
# Source captions retain every approved script word in order. Use word ranges
# as well as time ranges so an early ASR cue cannot leak the next beat's text.
BEAT_WORDS={};cursor=0
for beat in TIMELINE:
 words=beat['tr'].split();BEAT_WORDS[beat['id']]=(cursor,cursor+len(words));cursor+=len(words)
CUE_WORDS=[];cursor=0
for cue in CAPTIONS:
 words=cue['text'].split();CUE_WORDS.append((cue,cursor,cursor+len(words),words));cursor+=len(words)
assert ' '.join(b['tr'] for b in TIMELINE).split()==' '.join(c['text'] for c in CAPTIONS).split()

EDITS=[
 {'slug':'01-pythonun-altinda-ne-var','title':'Python görünüyor. Altında ne var?','groups':[['s05-p0','s05-p1','s06-p0','s06-p1']]},
 {'slug':'02-pythonun-icindeki-rust','title':'Python yazıyorsun. İçinde Rust var.','groups':[['s10-p0','s10-p1','s11-p0','s11-p1']]},
 {'slug':'03-hizli-dil-yavas-uygulama','title':'Daha hızlı dil, daha hızlı cevap?','groups':[['s14-example'],['s15-p1','s16-p0','s16-p1']]},
 {'slug':'04-rust-ogrenmeli-misin','title':'Rust öğrenmeli misin?','groups':[['s21-p0','s21-p1','s21-p2','s22-p0']]},
]
def run(args,log=None):
 if log:
  with open(log,'w') as f:r=subprocess.run(args,stdout=f,stderr=subprocess.STDOUT)
 else:r=subprocess.run(args,check=False)
 if r.returncode:raise RuntimeError(f'Command failed ({r.returncode}): {args[0]} — see {log}')
def stamp(ms,ass=False):
 ms=round(ms);s,ms=divmod(ms,1000);m,s=divmod(s,60);h,m=divmod(m,60)
 return f'{h}:{m:02}:{s:02}.{ms//10:02}' if ass else f'{h:02}:{m:02}:{s:02},{ms:03}'
def escaped(s):return s.replace('\\','').replace('{','').replace('}','').replace('\n',r'\N')
def prepare():
 OUT.mkdir(parents=True,exist_ok=True)
 for index,edit in enumerate(EDITS):
  folder=OUT/edit['slug'];folder.mkdir(exist_ok=True);offset=0;segments=[];cues=[]
  for group in edit['groups']:
   a=BY_ID[group[0]];z=BY_ID[group[-1]]
   # Keep a short breath around actual speech, trim long scene lead/tail holds.
   first=a['from']+max(0,a['lead']-3)
   last=min(z['from']+z['duration'],z['from']+z['lead']+math.ceil(z['voiceSec']*30)+4)
   seg={'startFrame':first,'endFrame':last,'outputFrame':offset,'beats':group};segments.append(seg)
   start=first/30*1000;end=last/30*1000
   word_start=BEAT_WORDS[group[0]][0];word_end=BEAT_WORDS[group[-1]][1]
   for c,a_word,z_word,words in CUE_WORDS:
    lo=max(a_word,word_start);hi=min(z_word,word_end)
    if lo>=hi:continue
    span=c['endMs']-c['startMs']
    cue_start=c['startMs']+span*(lo-a_word)/len(words)
    cue_end=c['startMs']+span*(hi-a_word)/len(words)
    if cue_end<=start or cue_start>=end:raise ValueError('Selected speech outside caption timing; inspect boundary')
    cues.append({'text':' '.join(words[lo-a_word:hi-a_word]),'startMs':max(cue_start,start)-start+offset/30*1000,'endMs':min(cue_end,end)-start+offset/30*1000})
   offset+=last-first
  expected=' '.join(BY_ID[ident]['tr'] for group in edit['groups'] for ident in group).split()
  assert ' '.join(c['text'] for c in cues).split()==expected,'Caption word loss'
  edit.update(index=index,segments=segments,totalFrames=offset,durationSeconds=offset/30)
  (folder/'captions.json').write_text(json.dumps(cues,ensure_ascii=False,indent=2)+'\n')
  (folder/'captions.srt').write_text('\n\n'.join(f"{i}\n{stamp(c['startMs'])} --> {stamp(c['endMs'])}\n{c['text']}" for i,c in enumerate(cues,1))+'\n')
  header='''[Script Info]
ScriptType: v4.00+
PlayResX: 1080
PlayResY: 1920
WrapStyle: 0
ScaledBorderAndShadow: yes
[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Speech,Manrope,64,&H00F4F0E6,&H006CD3EF,&H00101719,&H00101719,-1,0,0,0,100,100,-0.4,0,1,3,0,5,85,105,0,1
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
'''
  events=[]
  for c in cues:
   # Short source cues, centered in a dedicated portrait speech area.
   events.append(f"Dialogue: 0,{stamp(c['startMs'],True)},{stamp(c['endMs'],True)},Speech,,0,0,0,,{{\\pos(530,1265)\\fad(60,40)}}{escaped(c['text'])}")
  (folder/'captions.ass').write_text(header+'\n'.join(events)+'\n')
  (folder/'edit.json').write_text(json.dumps(edit,ensure_ascii=False,indent=2)+'\n')
  (folder/'props.json').write_text(json.dumps({'index':index}))
 (OUT/'edits.json').write_text(json.dumps(EDITS,ensure_ascii=False,indent=2)+'\n')
 return EDITS

def render(edits, reuse_cuts=False):
 for edit in edits:
  p=OUT/edit['slug'];print('EDIT',edit['slug'],edit['durationSeconds'],flush=True)
  run([str(PROJECT/'node_modules/.bin/remotion'),'still','src/index.ts','ReelKapak',str(p/'background.png'),'--props='+str(p/'props.json')],p/'background-render.log')
  pieces=[]
  for i,seg in enumerate(edit['segments']):
   dest=p/f'piece-{i}.mkv';pieces.append(dest)
   # Accurate seek with re-encode, retain original normalized speech and sound design.
   if not (reuse_cuts and dest.exists()):run(['ffmpeg','-v','error','-nostdin','-threads','1','-ss',str(seg['startFrame']/30),'-i',str(SOURCE),'-t',str((seg['endFrame']-seg['startFrame'])/30),'-map','0:v:0','-map','0:a:0','-vf','crop=1920:810:0:130,scale=1000:422:flags=lanczos,setsar=1','-r','30','-c:v','libx264','-threads','2','-preset','fast','-crf','18','-c:a','pcm_s16le','-f','matroska','-y',str(dest)],p/f'cut-{i}.log')
  listing=p/'concat.txt';listing.write_text(''.join("file '"+str(x)+"'\n" for x in pieces))
  duration=edit['durationSeconds']
  # A transparent progress fill and small fades complement the actual motion footage.
  # Draw an actually time-driven progress bar using a separate overlay.
  fc=f"[0:v]fps=30,format=yuv420p[bg];[1:v]crop=1000:400:0:0,setpts=PTS-STARTPTS[clip];[bg][clip]overlay=40:598:shortest=1,ass='{p/'captions.ass'}':fontsdir='{PROJECT/'public/art/fonts'}',drawbox=x=80:y=1745:w=900:h=4:color=0x294034:t=fill[canvas];color=c=0x60b9f2:s=900x4:r=30[bar];[canvas][bar]overlay=x='80-900+900*min(t/{duration},1)':y=1745:shortest=1,drawbox=x=0:y=1745:w=80:h=4:color=0x101719:t=fill,format=yuv420p[v]"
  run(['ffmpeg','-v','error','-nostdin','-loop','1','-i',str(p/'background.png'),'-f','concat','-safe','0','-i',str(listing),'-filter_complex_threads','1','-filter_complex',fc,'-map','[v]','-map','1:a:0','-t',str(duration),'-r','30','-c:v','libx264','-threads','2','-preset','veryfast','-crf','19','-c:a','aac','-b:a','192k','-ar','48000','-movflags','+faststart','-metadata','title='+edit['title'],'-metadata','artist=Buğra Akdemir','-y',str(OUT/(edit['slug']+'.mp4'))],p/'render.log')
  run(['ffmpeg','-v','error','-nostdin','-ss','2','-i',str(OUT/(edit['slug']+'.mp4')),'-frames:v','1','-update','1','-y',str(OUT/(edit['slug']+'-kapak.jpg'))])
  print('READY',edit['slug'],flush=True)
if __name__=='__main__':
 import argparse,os
 parser=argparse.ArgumentParser();parser.add_argument('--prepare-only',action='store_true');parser.add_argument('--index',type=int);parser.add_argument('--reuse-cuts',action='store_true');args=parser.parse_args()
 edits=prepare()
 if not args.prepare_only:
  os.chdir(PROJECT);render(edits if args.index is None else [edits[args.index]],args.reuse_cuts)
