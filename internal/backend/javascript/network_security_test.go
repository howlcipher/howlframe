package javascript

import (
	"encoding/json"
	"github.com/howlcipher/howlframe/internal/capability"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/nettest"
)

func securityJS(t *testing.T, script string, env ...string) {
	t.Helper()
	out, err, code := runJS(t, fetchJSHelper()+execJSHelper()+`
const assert = require("assert");
(async () => {
`+script+`
})().then(()=>console.log("passed"),err=>{console.error(err);process.exit(1)});
`, env...)
	if code != 0 || strings.TrimSpace(out) != "passed" {
		t.Fatalf("Node conformance: code=%d out=%q err=%s", code, out, err)
	}
}

const jsServers = `
const http = require("http");
const listen = async handler => {
 const s = http.createServer(handler);
 await new Promise(resolve=>s.listen(0,"127.0.0.1",resolve));
 return s;
};
const url = s => "http://127.0.0.1:"+s.address().port;
const grants = (...urls) => {process.env.HOWLFRAME_ALLOW_CAPS=urls.map(u=>"network:fetch="+new URL(u).host).join(",");};
const deny = async fn => {await assert.rejects(fn,/CAPABILITY_DENIED/);};
`

func TestJSRedirectCap11(t *testing.T) {
	securityJS(t, jsServers+`
 let hits=0;
 const s=await listen((req,res)=>{
  hits++; const n=Number(req.url.slice(1));
  if(n===11) return res.end("escaped");
  res.writeHead(302,{location:"/"+(n+1)});res.end();
 });
 try {
  grants(url(s));await deny(()=>howlFrameFetch(url(s)+"/0","GET"));
  assert.equal(hits,11);
 } finally {s.close();}
`)
}
func TestJSNamePrivateDial(t *testing.T) {
	securityJS(t, jsServers+`
 let hits=0;
 const s=await listen((req,res)=>{hits++;res.end("ok");});
 const dns=require("dns");
 const original=dns.promises.lookup;
 dns.promises.lookup=async()=>[{address:"127.0.0.1",family:4}];
 try {
  const named="http://private.test:"+s.address().port;
  grants(named);await deny(()=>howlFrameFetch(named,"GET"));assert.equal(hits,0);
  grants("http://localhost:"+s.address().port);
  assert.equal(await howlFrameFetch("http://localhost:"+s.address().port,"GET"),"ok");
  grants(url(s));assert.equal(await howlFrameFetch(url(s),"GET"),"ok");
  assert.equal(hits,2);
 } finally {dns.promises.lookup=original;s.close();}
`)
}
func TestJSPrivateRanges(t *testing.T) {
	list := func(xs []string) string { return `["` + strings.Join(xs, `","`) + `"]` }
	securityJS(t, `
 for (const ip of `+list(append(nettest.BlockedAddresses, "0:0:0:0:0:0:0:1", "::ffff:7f00:1"))+`) {
  assert.equal(howlFrameForbiddenAddress(ip,false),true,ip);
 }
 for (const ip of `+list(nettest.PublicAddresses)+`) assert.equal(howlFrameForbiddenAddress(ip,false),false,ip);
`)
}
func TestJSRedirectToOptOut(t *testing.T) {
	securityJS(t, jsServers+`
 const dns=require("dns"); const original=dns.promises.lookup;
 // localhost now has to resolve to loopback (#177); every other name is public.
 dns.promises.lookup=async h=>[{address:h==="localhost"?"127.0.0.1":"93.184.216.34",family:4}];
 let seen=[];
 howlFrameFetchRequest=async (u)=>{seen.push(u.href); const to=u.searchParams.get("to");
  return to ? {status:302,headers:{get:()=>to}} : {status:200,headers:{get:()=>null},text:async()=>"ok"};};
 process.env.HOWLFRAME_ALLOW_CAPS="network:fetch=api.example.com,network:fetch=10.0.0.5,network:fetch=localhost,network:fetch=127.0.0.1,network:fetch=[fd00::5]";
 try {
  for (const [from,to] of [["http://api.example.com/","http://10.0.0.5/"],["http://api.example.com/","http://localhost/"],["http://api.example.com/","http://[fd00::5]/"],["http://localhost/","http://127.0.0.1/"],["http://127.0.0.1/","http://localhost/"],["http://10.0.0.5/","http://127.0.0.1/"]]) {
   seen=[]; await deny(()=>howlFrameFetch(from+"?to="+encodeURIComponent(to),"GET")); assert.equal(seen.length,1,from+" -> "+to);
  }
  for (const [from,to] of [["http://api.example.com/","http://api.example.com/next"],["http://10.0.0.5/","http://10.0.0.5/next"],["http://localhost/","http://localhost/next"]]) {
   assert.equal(await howlFrameFetch(from+"?to="+encodeURIComponent(to),"GET"),"ok",from+" -> "+to);
  }
 } finally {dns.promises.lookup=original;}
`)
}
func TestJSMulticastDial(t *testing.T) {
	securityJS(t, `
 for(const ip of ["224.0.0.1","239.255.255.255","[ff00::1]","[ff02::1]"]) {
  process.env.HOWLFRAME_ALLOW_CAPS="network:fetch="+ip;
  await assert.rejects(()=>howlFrameFetch("http://"+ip,"GET"),/CAPABILITY_DENIED/);
 }
`)
}
func TestJSCredentialsPortChange(t *testing.T) {
	securityJS(t, jsServers+`
 let credentials, hits=0;
 const dst=await listen((req,res)=>{hits++;credentials=req.headers.authorization||req.headers.cookie||"";res.end("ok");});
 const src=await listen((req,res)=>{res.writeHead(307,{location:url(dst)});res.end();});
 try {
  grants(url(src),url(dst));
  assert.equal(await howlFrameFetch(url(src),"GET",undefined,{Authorization:"Bearer secret",Cookie:"session=secret"}),"ok");
  assert.equal(hits,1);assert.equal(credentials,"");
 } finally {src.close();dst.close();}
`)
}
func TestJSBrowserHTTPSDowngrade(t *testing.T) {
	// Browser-style manual responses avoid trusting a fake DNS or TLS certificate.
	securityJS(t, `
 const vm=require("vm");
 const source=howlFrameFetch.toString();
 let calls=0;
 const browser={
  URL, howlFrameFetchURL,
  howlFrameFetchAllowed:()=>true,
  howlFrameFetchResolvedOk:async()=>true,
  howlFrameFetchDeny,
  howlFrameFetchRequest:async()=>{calls++;return {status:302,headers:{get:()=>"http://example.test/"},text:async()=>"escaped"};}
 };
 vm.runInNewContext(source,browser);
 await assert.rejects(()=>browser.howlFrameFetch("https://example.test/","GET"),/CAPABILITY_DENIED/);
 assert.equal(calls,1);
`)
}
func TestJSAmbiguousNumericGrant(t *testing.T) {
	securityJS(t, `
 for(const spec of ["127.1","2130706433","0177.0.0.1","0x7f.0.0.1"]) {
  process.env.HOWLFRAME_ALLOW_CAPS="network:fetch="+spec;
  assert.equal(howlFrameFetchAllowed(new URL("http://127.0.0.1/")),false,spec);
 }
 process.env.HOWLFRAME_ALLOW_CAPS="network:fetch=127.0.0.1";
 for(const host of ["127.1","2130706433","0177.0.0.1","0x7f.0.0.1"]) {
  assert.throws(()=>howlFrameFetchURL("http://"+host),/CAPABILITY_DENIED/);
 }
 assert.equal(howlFrameFetchAllowed(new URL("http://127.0.0.1/")),true);
 for(const raw of ["http:127.1","http:///127.1","http:\\127.1"]) assert.throws(()=>howlFrameFetchURL(raw),/CAPABILITY_DENIED/);
`)
}
func TestJSCodeLoadingEnv(t *testing.T) {
	refused, _ := json.Marshal(nettest.EnvRefused)
	allowed, _ := json.Marshal(nettest.EnvAllowed)
	securityJS(t, `
 for(const raw of `+string(refused)+`) {
  process.env[raw]="injected";process.env.HOWLFRAME_ALLOW_CAPS="process:env="+raw;
  assert.deepEqual(howlFrameExecEnv(),{},raw);delete process.env[raw];
 }
 for(const raw of `+string(allowed)+`) {
  process.env[raw]="ok";process.env.HOWLFRAME_ALLOW_CAPS="process:env="+raw;
  assert.deepEqual(howlFrameExecEnv(),{[raw]:"ok"},raw);delete process.env[raw];
 }
`)
}

// The emitted tables are exactly capability's single-source tables.
func TestJSSharedTables(t *testing.T) {
	names, _ := json.Marshal(capability.CodeLoadingEnvNames)
	prefixes, _ := json.Marshal(capability.CodeLoadingEnvPrefixes)
	var rows [][2]any
	for _, raw := range capability.SpecialPrefixes {
		p := netip.MustParsePrefix(raw)
		bits := p.Bits()
		if p.Addr().Is4() {
			bits += 96
		}
		b := p.Addr().As16()
		ints := make([]int, 16)
		for i := range b {
			ints[i] = int(b[i])
		}
		rows = append(rows, [2]any{ints, bits})
	}
	special, _ := json.Marshal(rows)
	var lower []string
	for _, h := range capability.RedirectSafeHeaders {
		lower = append(lower, strings.ToLower(h))
	}
	headers, _ := json.Marshal(lower)
	securityJS(t, `
 assert.deepEqual(howlFrameCodeLoadingEnvNames,`+string(names)+`);
 assert.deepEqual(howlFrameCodeLoadingEnvPrefixes,`+string(prefixes)+`);
 assert.deepEqual(howlFrameSpecialPrefixes,`+string(special)+`);
 assert.deepEqual(howlFrameRedirectSafeHeaders,`+string(headers)+`);
`)
}
func TestJSProxyEnvIgnored(t *testing.T) {
	securityJS(t, jsServers+`
 let connections=0;
 const trap=await listen((req,res)=>res.end("proxy"));
 trap.on("connection",()=>connections++);
 const dst=await listen((req,res)=>res.end("direct"));
 for(const key of ["HTTP_PROXY","HTTPS_PROXY","http_proxy","https_proxy","ALL_PROXY"])process.env[key]=url(trap);
 process.env.NODE_USE_ENV_PROXY="1";process.env.NO_PROXY="";process.env.no_proxy="";
 globalThis.fetch=async()=>{throw new Error("environment-proxy fetch selected");};
 try {
  grants(url(dst));assert.equal(await howlFrameFetch(url(dst),"GET"),"direct");
  assert.equal(connections,0);
 } finally {trap.close();dst.close();}
`)
}

// TestJSProxyEnvAtStartup also runs under Node 24 when node is selected on PATH.
func TestJSProxyEnvAtStartup(t *testing.T) {
	var connections atomic.Int64
	trap := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "proxy") }))
	trap.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
			conn.Close()
		}
	}
	trap.Start()
	defer trap.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "direct") }))
	defer target.Close()
	tlsTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "tls-direct") }))
	defer tlsTarget.Close()
	script := `
 const targets=[` + strconv.Quote(target.URL) + `,` + strconv.Quote(tlsTarget.URL) + `];
 for(const u of targets) {
  process.env.HOWLFRAME_ALLOW_CAPS="network:fetch="+new URL(u).host;
  assert.equal(await howlFrameFetch(u,"GET"),u.startsWith("https")?"tls-direct":"direct");
 }
`
	securityJS(t, script, "HTTP_PROXY="+trap.URL, "HTTPS_PROXY="+trap.URL, "http_proxy="+trap.URL, "https_proxy="+trap.URL, "NO_PROXY=", "no_proxy=", "NODE_USE_ENV_PROXY=1", "NODE_TLS_REJECT_UNAUTHORIZED=0")
	if connections.Load() != 0 {
		t.Fatalf("proxy got %d connections", connections.Load())
	}
}

func TestJSPinsVettedLookup(t *testing.T) {
	securityJS(t, jsServers+`
 const s=await listen((req,res)=>res.end("pinned"));
 const originalRequest=http.request;
 const dns=require("dns");const originalLookup=dns.promises.lookup;
 let resolutions=0,connectedAddress;
 dns.promises.lookup=async()=>{resolutions++;return [{address:resolutions===1?"93.184.216.34":"127.0.0.1",family:4}];};
 http.request=(u,options,handler)=>{
  assert.equal(typeof options.lookup,"function","checked lookup must be used at connect time");
  options.lookup(u.hostname,{all:true},(err,addresses)=>{
   assert.ifError(err);connectedAddress=addresses[0].address;
  });
  return originalRequest(url(s),{method:options.method,agent:false},handler);
 };
 try {
  grants("http://public.test");
  assert.equal(await howlFrameFetch("http://public.test","GET"),"pinned");
  assert.equal(resolutions,1);assert.equal(connectedAddress,"93.184.216.34");
 } finally {http.request=originalRequest;dns.promises.lookup=originalLookup;s.close();}
`)
}

func TestJSHTTPSDowngrade(t *testing.T) {
	var hits atomic.Int64
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); io.WriteString(w, "escaped") }))
	defer dst.Close()
	src := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dst.URL, 302) }))
	defer src.Close()
	securityJS(t, `
 process.env.HOWLFRAME_ALLOW_CAPS=`+strconv.Quote("network:fetch="+strings.TrimPrefix(src.URL, "https://")+",network:fetch="+strings.TrimPrefix(dst.URL, "http://"))+`;
 await assert.rejects(()=>howlFrameFetch(`+strconv.Quote(src.URL)+`,"GET"),/CAPABILITY_DENIED/);
`, "NODE_TLS_REJECT_UNAUTHORIZED=0")
	if hits.Load() != 0 {
		t.Fatalf("downgrade reached HTTP target %d times", hits.Load())
	}
}

func TestJSFetchCompressedResponse(t *testing.T) {
	securityJS(t, jsServers+`
 const zlib=require("zlib");
 for(const encoding of ["gzip","deflate","br"]) {
  const data=Buffer.from("\ufeffcompressed response");
  const body=encoding==="gzip"?zlib.gzipSync(data):encoding==="deflate"?zlib.deflateSync(data):zlib.brotliCompressSync(data);
  const s=await listen((req,res)=>{res.writeHead(200,{"content-encoding":encoding});res.end(body);});
  try {grants(url(s));assert.equal(await howlFrameFetch(url(s),"GET"),"compressed response");} finally {s.close();}
 }
`)
}

// #177: localhost reaches loopback only, and Node pins every vetted address.
func TestJSLocalhostLoopbackAndPinAll(t *testing.T) {
	securityJS(t, `
 const dns=require("dns"); const original=dns.promises.lookup;
 try {
  for (const set of [["10.0.0.5"],["8.8.8.8"],["::ffff:10.0.0.5"],["fd00::1"],["127.0.0.1","10.0.0.5"],["::1","93.184.216.34"]]) {
   dns.promises.lookup=async()=>set.map(a=>({address:a,family:a.includes(":")?6:4}));
   assert.equal(await howlFrameFetchResolvedOk(new URL("http://localhost/")),false,set.join(","));
  }
  for (const set of [["127.0.0.1"],["127.0.0.1","::1"],["127.9.9.9"]]) {
   dns.promises.lookup=async()=>set.map(a=>({address:a,family:a.includes(":")?6:4}));
   assert.equal(await howlFrameFetchResolvedOk(new URL("http://localhost/")),true,set.join(","));
  }
  dns.promises.lookup=async()=>[{address:"93.184.216.34",family:4},{address:"10.0.0.5",family:4}];
  assert.equal(await howlFrameFetchResolvedOk(new URL("http://api.example.com/")),false);
  dns.promises.lookup=async()=>[{address:"93.184.216.34",family:4},{address:"2606:4700::1",family:6}];
  const u=new URL("http://api.example.com/");
  assert.equal(await howlFrameFetchResolvedOk(u),true);
  const http=require("http"); const realRequest=http.request; let lookupFn;
  http.request=(url,options)=>{lookupFn=options.lookup; throw new Error("stop");};
  try { await howlFrameFetchRequest(u,{method:"GET",headers:{}}); } catch(e) {}
  http.request=realRequest;
  const call=opts=>new Promise((res,rej)=>lookupFn("api.example.com",opts,(err,a,f)=>err?rej(err):res([a,f])));
  assert.deepEqual((await call({all:true}))[0],[{address:"93.184.216.34",family:4},{address:"2606:4700::1",family:6}]);
  assert.deepEqual(await call({family:6}),["2606:4700::1",6]);
  assert.deepEqual(await call({}),["93.184.216.34",4]);
 } finally {dns.promises.lookup=original;}
`)
}

// #177: only the shared safe headers cross an origin change; userinfo redirect targets are refused.
func TestJSCrossOriginHeadersAndUserinfo(t *testing.T) {
	sensitive, _ := json.Marshal(nettest.SensitiveHeaders)
	securityJS(t, jsServers+`
 let seen, hits=0;
 const dst=await listen((req,res)=>{hits++;seen=req.headers;res.end("ok");});
 const src=await listen((req,res)=>{res.writeHead(307,{location:url(dst)});res.end();});
 const same=await listen((req,res)=>{ if(req.url==="/") {res.writeHead(307,{location:"/next"});return res.end();} seen=req.headers;res.end("ok");});
 const headers={Accept:"text/plain"}; for (const h of `+string(sensitive)+`) headers[h]="secret";
 try {
  grants(url(src),url(dst));
  assert.equal(await howlFrameFetch(url(src),"GET",undefined,Object.assign({},headers)),"ok");
  assert.equal(hits,1);
  for (const [k,v] of Object.entries(seen)) assert.ok(!String(v).includes("secret"),k+" crossed origin");
  assert.equal(seen.accept,"text/plain");
  grants(url(same));
  assert.equal(await howlFrameFetch(url(same),"GET",undefined,Object.assign({},headers)),"ok");
  assert.equal(seen["x-api-key"],"secret","same-origin custom header dropped");
  for (const target of ["http://user:pw@127.0.0.1:"+dst.address().port+"/","http://user@127.0.0.1:"+dst.address().port+"/"]) {
   const r=await listen((req,res)=>{res.writeHead(302,{location:target});res.end();});
   hits=0; grants(url(r),url(dst));
   await deny(()=>howlFrameFetch(url(r),"GET")); assert.equal(hits,0,target); r.close();
  }
 } finally {src.close();dst.close();same.close();}
`)
}
