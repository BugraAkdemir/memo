// Real Flutter web UI exercise. RAF cadence is a browser responsiveness proxy, not native GPU FPS.
const {createRequire}=require('module');const fs=require('fs');const path=require('path');
const base=process.env.BENCH_NODE_ROOT||'/home/bugra/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/';
const {chromium}=createRequire(base)('playwright');
const root=process.env.BENCH_OUTPUT||path.resolve('data/performance-benchmark');
const stage=process.env.BENCH_STAGE||JSON.parse(fs.readFileSync(path.join(root,'awaiting-ui.json'),'utf8')).stage;
(async()=>{
 const b=await chromium.launch({headless:true,executablePath:process.env.BENCH_CHROMIUM||'/usr/bin/chromium',args:['--no-sandbox']});
 try{
  const p=await b.newPage({viewport:{width:1440,height:1000}});const errors=[];p.on('pageerror',e=>errors.push(e.message));
  await p.addInitScript(()=>{for(const key of ['memo_setup_complete','memo_launchpad_seen','memo_tour_seen'])localStorage.setItem('flutter.'+key,'true');localStorage.setItem('flutter.memo_locale','"en"')});
  const nav=Date.now();await p.goto('http://127.0.0.1:18080');
  const a11y=p.getByRole('button',{name:'Enable accessibility'});await a11y.waitFor();await a11y.dispatchEvent('click');
  const localOnly=p.getByText("No, I'll only use it on this device",{exact:true});
  try{await localOnly.waitFor({timeout:7000});await localOnly.click()}catch{}
  const box=p.locator('textarea[aria-label="Type your message... (/)"]');await box.waitFor({timeout:45000});
  await p.waitForFunction(()=>!document.body.innerText.includes("Can't connect to the server")&&!document.body.innerText.includes("I'll only use it on this device"),{},{timeout:30000});await p.waitForTimeout(1500);
  const skipEngine=p.getByText('Skip (Install Later from Settings)',{exact:true});try{await skipEngine.waitFor({timeout:5000});await skipEngine.click()}catch{}
  const readyMs=Date.now()-nav;const snapshot=await p.locator('body').innerText();
  const apiOpenStart=Date.now();await p.evaluate(async()=>{await fetch('/api/chats/switch',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:'bench-eval'})})});
  // Reload so the real notifier loads the selected session through normal API requests.
  await p.reload();await a11y.waitFor();await a11y.dispatchEvent('click');await box.waitFor({timeout:30000});try{await skipEngine.waitFor({timeout:5000});await skipEngine.click()}catch{}
  const jump=p.getByRole('button',{name:'Jump to latest message',exact:true});if(await jump.count())await jump.click();
  const openedAt=Date.now();
  await p.evaluate(()=>{window.benchFrames=[];window.benchLong=[];window.benchRunning=true;let prev;function frame(t){if(prev!==undefined)window.benchFrames.push(t-prev);prev=t;if(window.benchRunning)requestAnimationFrame(frame)}requestAnimationFrame(frame);new PerformanceObserver(xs=>window.benchLong.push(...xs.getEntries().map(e=>({start:e.startTime,duration:e.duration})))).observe({type:'longtask',buffered:false})});
  await p.waitForTimeout(1500);const idle=await p.evaluate(()=>{const a=window.benchFrames;window.benchFrames=[];return a});
  await box.click();await p.waitForTimeout(250);await p.keyboard.insertText('BENCH_STREAM Render the synthetic Markdown stream for the UI benchmark.');await p.waitForTimeout(250);await p.getByRole('button',{name:'',exact:true}).last().click();await p.waitForTimeout(500);if(await jump.count())await jump.click();
  const completed = await p.waitForFunction(async()=>{
   const r=await fetch('/api/messages?chat_id=bench-eval');const rows=await r.json();
   return rows.some(m=>m.role==='assistant' && m.content.includes('Stream paragraph'));
  },{},{timeout:30000}).then(()=>true).catch(()=>false);
  await p.waitForTimeout(500);
  const raw=await p.evaluate(()=>{window.benchRunning=false;return {intervals:window.benchFrames,longTasks:window.benchLong,paints:performance.getEntriesByType('paint').map(x=>({name:x.name,start:x.startTime}))}});
  function summarize(a){const s=[...a].sort((x,y)=>x-y);return {samples:a.length,mean_interval_ms:a.reduce((x,y)=>x+y,0)/a.length,p95_interval_ms:s[Math.floor((s.length-1)*.95)],over_33ms:a.filter(x=>x>33.34).length,cadence_fps:1000/(a.reduce((x,y)=>x+y,0)/a.length)}}
  const result={stage:Number(stage),platform:'Linux Chromium headless Flutter web release',metric:'requestAnimationFrame cadence; NOT native rendered FPS',navigation_to_composer_ms:readyMs,chat_open_window:{start_unix_ms:apiOpenStart,end_unix_ms:openedAt},stream_window:{start_unix_ms:openedAt,end_unix_ms:Date.now()},response_persisted:completed,response_rendered:null,visual_review_required:true,idle:summarize(idle),stream:summarize(raw.intervals),long_tasks:raw.longTasks,paints:raw.paints,errors,snapshot_before:snapshot,snapshot_after:(await p.locator('body').innerText()).slice(-3000)};
  await p.screenshot({path:path.join(root,`stage-${stage}-ui.png`)});fs.writeFileSync(path.join(root,`stage-${stage}-ui.json`),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }finally{await b.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
