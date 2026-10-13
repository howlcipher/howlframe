package javascript

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
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
	securityJS(t, `
 for (const ip of ["127.1.2.3","0:0:0:0:0:0:0:1","::1","10.1.2.3","172.16.0.1","172.31.255.255","192.168.1.1","fc00::1","fd12::1","64:ff9b::7f00:1","169.254.169.254","fe80::1","::ffff:127.0.0.1","::ffff:7f00:1"]) {
  assert.equal(howlFrameForbiddenAddress(ip,false),true,ip);
 }
 for (const ip of ["172.15.0.1","172.32.0.1","192.169.1.1","2001:4860::1","64:ff9b:1::1","64:ff9b::1:0:0"]) assert.equal(howlFrameForbiddenAddress(ip,false),false,ip);
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
	securityJS(t, `
 for(const name of ["PYTHONPATH","NODE_OPTIONS","BASH_ENV","ENV","PERL5LIB","PERL5OPT","RUBYOPT","RUBYLIB","PYTHONSTARTUP","NODE_PATH"]) {
  for(const raw of [name,name.toLowerCase()]) {
   process.env[raw]="injected";process.env.HOWLFRAME_ALLOW_CAPS="process:env="+raw;
   assert.deepEqual(howlFrameExecEnv(),{},raw);delete process.env[raw];
  }
 }
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
