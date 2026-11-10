"""Author the film from the approved script, adding short everyday analogies.

No credentials are stored here. The main narration remains outside the visuals.
"""
import json
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parent.parent
REPO = ROOT.parent.parent
plan = (REPO / 'docs/plans/PLAN_python_rust_ai_video.md').read_text()
chapters = re.findall(r'Konuşma:\n(.*?)(?=\n(?:Kaynak|## ))', plan, re.S)
assert len(chapters) == 10
texts = [[p.strip() for p in c.strip().split('\n\n') if p.strip()] for c in chapters]
mapping = [[0,1],[2,3,4],[5,6],[7,8,9],[10,11],[12,13],[14,15,16],[17,18],[19,20],[21,22]]

# Distribute whole sentences, never arbitrary word cuts, over visual scenes.
groups = {}
for chapter, scene_ids in enumerate(mapping):
    sentences = re.split(r'(?<=[.!?])\s+', ' '.join(texts[chapter]))
    buckets = [[] for _ in scene_ids]
    for i,sentence in enumerate(sentences):
        buckets[min(len(scene_ids)-1, i*len(scene_ids)//len(sentences))].append(sentence)
    for scene_id,bucket in zip(scene_ids,buckets): groups[scene_id]=' '.join(bucket)

additions = {
2: 'Bir restoran düşün. Garson siparişi alıyor, mutfak yemeği pişiriyor. Garsonu değiştirince fırının daha hızlı pişirmesini bekler misin? Burada da işleri ayrı düşünmek gerekiyor.',
8: 'Ben şimdi anahtar örneğini göstereyim. Aynı odayı iki kişi aynı anda değiştirmek istiyor. Kimin ne zaman içeri girebildiği belli değilse işler karışabilir. Rust burada bazı erişim kurallarını daha baştan kontrol ediyor. Bu, sahiplik ve ödünç alma için basitleştirilmiş bir benzetme.',
14: 'Bir de trafikte olduğunu düşün. Araban çok hızlı ama önünde tek şeritli dar bir köprü var. Daha güçlü bir motor, o köprünün kapasitesini artırmıyor. Önce beklemenin nedenini bulmalısın.',
18: 'Şimdi küçük bir atölyedeyiz. Makine çalışıyor ama bir parça sorun çıkarıyor. İlk iş bütün makineyi sökmek mi olmalı? Belki sadece o parçayı denemek yeterli. Yazılımda da önce küçük bir alanı değiştirmek daha kontrollü bir deney sağlar.',
19: 'Bu köprünün bir tarafında Python uygulaman, diğer tarafında Rust ile yazdığın bir parça var. İkisini birbirine bağlayabiliyorsun. Ama köprüden veri geçirmenin de bir maliyeti olduğunu unutma.',
}
roles = {2:'classic',8:'pixel',14:'pixel',18:'classic'}
beats=[]
intro='Yapay zekâ için Python öğreniyorsun. Sonra karşına Rust çıkıyor. Peki, yanlış dili mi öğreniyorsun? Yoksa aynı uygulamanın farklı parçaları için farklı araçlardan mı konuşuyoruz? Gel, bunu bir restoran, bir trafik sıkışıklığı ve küçük bir atölye üzerinden anlatalım.'
beats.append({'id':'intro','scene':0,'part':0,'role':'narrator','tr':intro,'highlight':'Python mu, Rust mı?','mode':'coldopen'})
labels={0:'Yanlış soru, yanlış karar',1:'Önce yaptığın işi ayır',2:'Sipariş ≠ mutfak',3:'Aynı ürün, farklı katmanlar',4:'Bir PDF’nin yolculuğu',5:'Direksiyon ≠ motor',6:'Ağır iş alt katmanda',7:'Kontrol ve erken uyarı',8:'Değiştirme izni kimde?',9:'Hatasızlık garantisi yok',10:'Metin → token → model',11:'Python’ın içinde Rust',12:'Gerçek proje: Candle',13:'Model + donanım + özellik',14:'Darboğazı bul',15:'Bekleme nerede?',16:'Önce ölç, sonra değiştir',17:'Senin zamanın da değerli',18:'Önce tek parçayı dene',19:'Python + Rust',20:'Büyük göç yerine küçük deney',21:'Karar ihtiyacına bağlı',22:'Hangi problemi çözüyorum?'}
for scene in range(23):
    if scene in additions:
        beats.append({'id':f's{scene:02d}-example','scene':scene,'part':0,'role':roles.get(scene,'narrator'),'tr':additions[scene],'highlight':labels[scene],'mode':'example'})
    sentences = re.split(r'(?<=[.!?])\s+', groups[scene])
    # Keep each authored shot about 8–15 seconds; editing follows measured audio.
    chunks=[];cur=[];words=0
    for sentence in sentences:
        if words>=27:
            chunks.append(' '.join(cur));cur=[];words=0
        cur.append(sentence);words+=len(sentence.split())
    if cur: chunks.append(' '.join(cur))
    for part,text in enumerate(chunks):
        beats.append({'id':f's{scene:02d}-p{part}','scene':scene,'part':part,'role':'narrator','tr':text,'highlight':labels[scene],'mode':'detail' if part else 'overview'})

# Avoid visually showing benchmark-like results: all examples are explanations.
spec={'voices':{'narrator':'bGmbiluBMaeupKCFxoOk','classic':'bGmbiluBMaeupKCFxoOk','pixel':'cjVigY5qzO86Huf0OWal'},'beats':beats}
(ROOT/'script/narration.json').write_text(json.dumps(spec,ensure_ascii=False,indent=2)+'\n')
print('Authored',len(beats),'beats;',sum(len(b['tr'].split()) for b in beats),'spoken words')
