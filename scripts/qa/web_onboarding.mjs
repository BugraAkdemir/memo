export default async (page,ui)=>{
 console.log('QA: route setup');let logged=false;let onboardingCalls=0;const afterCalls=[];
 await page.route('**/api/**',async route=>{
  const path=new URL(route.request().url()).pathname;
  let data={};let status=200;
  if(path==='/api/setup/status')data={needs_setup:false,auth_mode:'password',loopback:false,install_id:'memo-web-qa'};
  else if(path==='/api/auth/login'){logged=true;data={session_token:'qa-local-only-token',role:'admin'};}
  else if(path==='/api/onboarding'){onboardingCalls++;if(!logged){status=401;data={error:'unauthorized'}}else data={completed:true};}
  else if(!logged){status=401;data={error:'unauthorized'};}
  else {afterCalls.push(path);if(path==='/api/version')data={version:'4.6.0'};else if(path==='/api/models'||path==='/api/chats'||path==='/api/providers'||path==='/api/calendar/events'||path==='/api/skills'||path==='/api/tasklists'||path==='/api/tasks/running')data=[];else if(path==='/api/messages')data=[];}
  await route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
 });
 await page.reload({waitUntil:'domcontentloaded'});console.log('QA: reloaded');
 await page.locator('flt-semantics-placeholder').waitFor({timeout:15000});
 await page.locator('flt-semantics-placeholder').evaluate(el=>el.click());console.log('QA: accessibility enabled');
 await page.getByRole('textbox').first().waitFor({timeout:20000});
 const before=await ui.snapshot();
 const fields=[...before.matchAll(/@(e\d+) textbox/g)].map(m=>'@'+m[1]);
 const sign=before.match(/@(e\d+) button "(?:Sign in|Giriş yap)"/);
 if(fields.length<2||!sign)throw new Error('No password login form: '+before);
 await ui.fill(fields[0],'qa-user');await ui.fill(fields[1],'qa-password');
 await ui.click('@'+sign[1]);
 await page.waitForFunction(()=>!document.querySelector('input[type="password"]'),{},{timeout:15000});
 await page.waitForTimeout(1000);
 const after=await ui.snapshot({full:true});
 if(onboardingCalls<2)throw new Error('Onboarding was not refreshed: '+onboardingCalls);
 if(/Step 1 \/ 5|Make Memo yours|Language & Appearance/.test(after))throw new Error('Wizard shown on configured server');
 return {onboardingCalls,logged,before,after,afterCalls};
}
