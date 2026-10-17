const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium,webkit}=require('playwright');
const assets=path.resolve(__dirname,'../assets');
const today=()=>new Date(Date.now()+8*3600e3).toISOString().slice(0,10);
for(const [engine,browserType] of Object.entries({chromium,webkit}))for(const lang of ['en','zh','zh-TW','ja','de']){
 test(`${engine} ${lang}: inactive Qoder is manually checkable; prior rewards stay prior`,async(t)=>{
  const browser=await browserType.launch({headless:true});t.after(()=>browser.close());
  const page=await browser.newPage({viewport:{width:900,height:900},reducedMotion:'reduce'});
  const asked=[],errors=[];page.on('pageerror',e=>errors.push(e.message));
  const settings={lang,theme:'light',qoderCheckin:true,workbuddyCheckin:true,pluginCheckins:{'qoder-cn':true}};
  const make=(provider,name,by,outcome,credit)=>({provider,name,user:name,plan:'Pro',windows:[{name:'Credits',used:1}],checkins:true,checkinBy:by,checkin:{day:today(),outcome,credit}});
  const cards=[make('qoder','Qoder live','qoder','inactive'),make('plugin:qoder-cn','Qoder CN plugin','plugin:qoder-cn','inactive'),make('workbuddy','WorkBuddy prior','','done',50),make('workbuddy-ai','WorkBuddy new','','claimed',100)];
  await page.route('**/*',async route=>{
   const u=new URL(route.request().url()),json=data=>route.fulfill({json:data});
   if(u.pathname==='/boot.js')return route.fulfill({contentType:'text/javascript',body:`window.bootPrefs=${JSON.stringify({lang,theme:'light',web:true})}`});
   if(u.pathname==='/wails/runtime.js')return route.fulfill({contentType:'text/javascript',body:'export const Window = {};'});
   if(u.pathname==='/api/state')return json({agents:[],profiles:[],settings});
   if(u.pathname==='/api/settings')return json(settings);
   if(u.pathname==='/api/providers')return json({providers:[],presets:[],plugins:[],excluded:[],gateway:{running:true,window:true}});
   if(u.pathname==='/api/usage/quotas')return json(cards);
   if(u.pathname==='/api/usage/qoder-checkin'){asked.push('qoder');return json([{outcome:'done'}]);}
   if(u.pathname==='/api/groups')return json({groups:[],models:[]});
   if(u.pathname==='/api/plugins')return json({plugins:[]});
   if(u.pathname.startsWith('/api/'))return json({});
   const file=path.join(assets,u.pathname==='/'?'index.html':u.pathname);
   try{return route.fulfill({body:await fs.readFile(file),contentType:{'.html':'text/html','.js':'text/javascript','.css':'text/css','.svg':'image/svg+xml'}[path.extname(file)]});}catch{return route.fulfill({status:404,body:''});}
  });
  await page.goto('http://magpie.test/?view=usage');
  const qoder=page.locator('.subscription-card',{hasText:'Qoder live'});
  await qoder.locator('.ci-now').waitFor();
  assert.equal(await page.locator('.subscription-card',{hasText:'Qoder CN plugin'}).locator('.ci-now').count(),1);
  const prior=page.locator('.subscription-card',{hasText:'WorkBuddy prior'});
  assert.doesNotMatch(await prior.locator('.ci-say').innerText(),/\+50/);
  assert.match(await page.locator('.subscription-card',{hasText:'WorkBuddy new'}).locator('.ci-say').innerText(),/\+100/);
  assert.equal(await prior.locator('.ci-now').count(),0);
  if(lang==='en'){
   assert.match(await prior.locator('.ci-say').innerText(),/Already checked in today/);
   await qoder.locator('.ci-now').click();
   await page.getByText('Check-in status refreshed',{exact:true}).waitFor();
   assert.deepEqual(asked,['qoder']);
  }
  for(const width of [440,900]){await page.setViewportSize({width,height:900});assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));}
  assert.deepEqual(errors,[]);
 });
}
