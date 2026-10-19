import { createServer } from 'node:http';
import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
import {join} from 'node:path';
if(!process.env.DSH_APP_ASAR)throw Error('Set DSH_APP_ASAR and run this file with the installed Electron executable in ELECTRON_RUN_AS_NODE=1 mode.');
const modules=pathToFileURL(join(process.env.DSH_APP_ASAR,'dsh','node_modules')+'/').href;
const session=await import(modules+'@deepseek-ai/dsh-session/lib/index.js');
const pia=await import(modules+'@deepseek-ai/dsh-llm-pi-ai/lib/index.js');
const {BasicCompactionEngine}=await import(modules+'@deepseek-ai/dsh-compaction-basic/lib/index.js');
let messages,header={};
if(process.argv.includes('--events')){
  const chunks=[]; for await(const b of process.stdin)chunks.push(b);
  const events=JSON.parse(Buffer.concat(chunks).toString('utf8'));
  const surface=session.foldSurface(events);
  messages=surface.nodes.map(n=>session.deriveEventMessage(events[n],surface.projectedMessages)).filter(Boolean);
  header=events.findLast(e=>e.type==='request/header')?.data.header??{};
}else{
  messages=[{role:'user',content:[{type:'text',text:'public synthetic history '.repeat(85000)}],source:{kind:'user'},id:'synthetic'}];
}
let adapter;
let received=[];
const server=createServer(async(req,res)=>{
  let raw='';for await(const b of req)raw+=b;
  const body=JSON.parse(raw);
  const cap=body.max_tokens??body.max_completion_tokens;
  received.push({cap,model:body.model,messageCount:body.messages.length});
  const text=cap===1?'x':'## Goal\nResume the task.\n## Progress\nCheckpoint created.\n## Next steps\nRead the saved files.';
  res.writeHead(200,{'content-type':'text/event-stream'});
  res.end('data: '+JSON.stringify({id:'synthetic',model:body.model,choices:[{index:0,delta:{content:text},finish_reason:null}]})+'\n\ndata: '+JSON.stringify({id:'synthetic',model:body.model,choices:[{index:0,delta:{},finish_reason:cap===1?'length':'stop'}],usage:{prompt_tokens:577694,completion_tokens:cap===1?1:24,total_tokens:577718}})+'\n\ndata: [DONE]\n\n');
});
await new Promise(r=>server.listen(0,'127.0.0.1',r));
try{
  const profiles={magpie:{api:'openai-completions',baseURL:`http://127.0.0.1:${server.address().port}/v1`,models:[{id:'old',contextWindow:524288,maxTokens:65536},{id:'recovery',contextWindow:1048576,maxTokens:65536}]}};
  const ctx={fiber:{entry:{options:{id:'isolated'}}},inject(){},on(){},get(){return undefined},logger:{warn(){},error(){}},llm:{registerConfigurableProviders(){return{replace(){}}},registerModelDiscovery(){},registerAdapter(_routes,value){adapter=value;return{replace(){}}}}};
  pia.apply(ctx,{providers:{get:()=>profiles}});
  adapter.config.resolveApiKey=async()=> 'public';
  for(const model of ['old','recovery']){
    const agent={options:{provider:'magpie',model},session:{requestHeader:()=>({config:{provider:'magpie',model}}),toolHistory:()=>new Map()}};
    const config={modelPolicies:[],thresholdRatio:.8,headroomTokens:65536,retainRatio:.16,summarizationProvider:'',summarizationModel:'',maxTokens:65536};
    let verdict;
    try{
      const result=await BasicCompactionEngine.prototype.summarize.call({ctx:{llm:{stream:o=>adapter.stream(o)}},config},{messages,system:header.system,tools:header.tools},agent,AbortSignal.timeout(30000));
      verdict={success:true,summaryBlocks:result.summary.length};
    }catch(e){verdict={success:false,errorCode:e.code,error:e.message};}
    assert.equal(verdict.success,model==='recovery');
    assert.equal(received.at(-1).cap,model==='old'?1:65536);
    if(model==='old')assert.equal(verdict.errorCode,'MAX_TOKENS');
    console.log(JSON.stringify({model,...verdict,request:received.at(-1),modelRequests:0,fakeUpstream:true}));
  }
}finally{server.closeAllConnections();await new Promise(r=>server.close(r));}
