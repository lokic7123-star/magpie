const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium,webkit}=require('playwright');
const assets=path.resolve(__dirname,'../assets');
const source=fs.readFileSync(path.join(assets,'app.js'),'utf8');
const helpers=source.includes('function checkinCurrent(')?source.slice(source.indexOf('function checkinCurrent('),source.indexOf('function checkinRow(')):'';
const row=source.slice(source.indexOf('function checkinRow('),source.indexOf('// checkinVendorName'));
for(const [name,engine] of Object.entries({chromium,webkit}))test(`${name}: inactive recheck, old rewards and narrow layout`,async()=>{
 const browser=await engine.launch({headless:true});
 try{const page=await browser.newPage({viewport:{width:440,height:650}});
 await page.setContent('<body></body>');await page.addStyleTag({content:fs.readFileSync(path.join(assets,'app.css'),'utf8')});
 await page.evaluate(({helpers,row})=>{
  window.state={settings:{qoderCheckin:true}};
  window.el=(tag,cls,txt)=>{const x=document.createElement(tag);x.className=cls||'';if(txt)x.textContent=txt;return x};
  window.t=(s,v={})=>s.replace(/\{(\w+)\}/g,(_,k)=>v[k]??k);
  window.wbToday=()=>new Date(Date.now()+8*3600e3).toISOString().slice(0,10);
  window.CHECKINS={qoder:{pref:'qoderCheckin',api:'qoder-checkin',say:'',on:'',off:''}};
  window.status=()=>{};window.loadQuotas=()=>{};
  window.api=async()=>[];
  window.make=(outcome)=>{const q={checkins:true,checkinBy:'qoder',checkin:{day:wbToday(),outcome,credit:100,at:new Date().toISOString()}};document.body.replaceChildren(checkinRow(q,true,[q]));return q};
  (0,eval)(helpers+row);
 },{helpers,row});
 await page.evaluate(()=>make('inactive'));
 assert.equal(await page.locator('.ci-now').count(),1,'inactive Qoder must remain manually recheckable');
 await page.evaluate(()=>make('done'));
 assert.match(await page.locator('.ci-say').textContent(),/Already checked in today/);
 assert.doesNotMatch(await page.locator('.ci-say').textContent(),/\+100/,'prior reward must not look newly granted');
 assert.equal(await page.locator('.ci-now').count(),0,'current completed campaign must not be claimed again');
 assert.match(await page.locator('.ci-time').first().textContent(),/UTC/);
 for(const theme of ['light','dark']){await page.evaluate(theme=>{document.documentElement.dataset.theme=theme;document.body.style.cssText='margin:12px;max-width:416px';},theme);assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'no horizontal overflow');const dir=process.env.ARTIFACT_DIR;if(dir){fs.mkdirSync(dir,{recursive:true});await page.screenshot({path:path.join(dir,`checkin-${name}-${theme}.png`)})}}
 await page.evaluate(()=>make('claimed'));assert.match(await page.locator('.ci-say').textContent(),/Checked in today \+100/);
 }finally{await browser.close()}
});
