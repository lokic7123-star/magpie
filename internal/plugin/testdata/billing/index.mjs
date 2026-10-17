export const BillingFixture = async () => ({
 config: async c => { c.provider ??= {}; c.provider["billing-fixture"] = {name:"Billing fixture",models:{}} },
 auth: {provider:"billing-fixture",methods:[{type:"api",label:"Fixture"}]},
 provider: {id:"billing-fixture",models: async (p,{auth}) => {
  const first=auth?.key === "unpriced"
  const ms={
   fast:{id:"fast",name:"Fast",free:false,rate:first?0:0.34},
   free:{id:"free",name:"Free candidate",free:!first,rate:0},
   priced:{id:"priced",name:"Priced",free:false,rate:first?0.7:0.9},
   unknown:{id:"unknown",name:"Unknown",free:false,rate:0},
  }
  if(!first)ms.extra={id:"extra",name:"Browser only",rate:0.5}
  return ms
 }},
})
