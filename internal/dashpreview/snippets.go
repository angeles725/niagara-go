// The snippets below were frozen from tools/dashboard-preview.py and the Python 3.15 http.server
// at niagara-research tag pre-go-retire; no generator exists, so they are edited by hand.

package dashpreview

// editorSnippet is EDITOR_SNIPPET, verbatim.
const editorSnippet = "\n" +
	"<style id=\"__ed_css\">\n" +
	"  #__ed{position:fixed;top:10px;left:10px;z-index:2147483647;width:300px;max-height:92vh;\n" +
	"        overflow:auto;background:rgba(20,22,26,.94);color:#e6e9ee;font:12px system-ui,sans-serif;\n" +
	"        border:1px solid #3a3f47;border-radius:10px;box-shadow:0 12px 40px -12px #000;padding:12px}\n" +
	"  #__ed h4{margin:0 0 6px;font-size:12px;letter-spacing:.06em}\n" +
	"  #__ed .hint{color:#9aa2ad;font-size:11px;line-height:1.5;margin-bottom:8px}\n" +
	"  #__ed table{width:100%;border-collapse:collapse;font-size:11px}\n" +
	"  #__ed th,#__ed td{padding:2px 3px;text-align:left;border-bottom:1px solid #2a2f37;white-space:nowrap}\n" +
	"  #__ed th{color:#9aa2ad;font-weight:600}\n" +
	"  #__ed td b{color:#7fd1a8}\n" +
	"  #__ed .btns{display:flex;flex-wrap:wrap;gap:6px;margin-top:10px}\n" +
	"  #__ed button{background:#2b303a;color:#e6e9ee;border:1px solid #3a3f47;border-radius:6px;\n" +
	"               padding:6px 8px;font-size:11px;cursor:pointer}\n" +
	"  #__ed button:hover{background:#353b46}\n" +
	"  #__ed .warn{background:#3a2b2b;border-color:#5a3a3a}\n" +
	"  #__ed textarea{width:100%;height:70px;margin-top:8px;background:#11141a;color:#cfe;border:1px solid #2a2f37;\n" +
	"                 border-radius:6px;font:11px monospace;padding:6px;display:none}\n" +
	"  #__ed .ok{color:#7fd1a8;font-size:11px;min-height:14px;margin-top:4px}\n" +
	"</style>\n" +
	"<script id=\"__ed_js\">\n" +
	"(function(){\n" +
	"  if (window.__labelEditor) return; window.__labelEditor = true;\n" +
	"  function ready(){\n" +
	"    if (typeof ui==='undefined' || typeof SENSORS==='undefined'){ return setTimeout(ready,150); }\n" +
	"    ui.editPuntos = true;\n" +
	"    var box=document.createElement('div'); box.id='__ed';\n" +
	"    box.innerHTML =\n" +
	"      '<h4>EDITOR DE LABELS (solo preview)</h4>'+\n" +
	"      '<div class=\"hint\">Abr\\u00ed un cuarto (clic en la lista de la derecha). '+\n" +
	"      'Arrastr\\u00e1 el <b>punto</b> (c\\u00edrculo) y la <b>etiqueta</b> para moverlos. '+\n" +
	"      'Los valores se actualizan abajo. Copi\\u00e1 y pas\\u00e1melos, o los aplico yo.</div>'+\n" +
	"      '<div id=\"__ed_room\"></div>'+\n" +
	"      '<table><thead><tr><th>tag</th><th>rx,ry</th><th>rlx,rly</th><th>side</th></tr></thead>'+\n" +
	"      '<tbody id=\"__ed_body\"></tbody></table>'+\n" +
	"      '<div class=\"btns\">'+\n" +
	"      '<button id=\"__ed_copy\">Copiar valores</button>'+\n" +
	"      '<button id=\"__ed_all\">Copiar TODO</button>'+\n" +
	"      '<button id=\"__ed_reset\" class=\"warn\">Reset respaldo local</button>'+\n" +
	"      '</div><div class=\"ok\" id=\"__ed_ok\"></div><textarea id=\"__ed_ta\" readonly></textarea>';\n" +
	"    document.body.appendChild(box);\n" +
	"    var fc=document.getElementById('frameC'); if(fc) fc.classList.add('edit');\n" +
	"\n" +
	"    function rows(all){\n" +
	"      return SENSORS.filter(function(s){ return all || s.cuarto===ui.cuarto; })\n" +
	"        .map(function(s){\n" +
	"          return s.tag+': rx:'+(+s.rx).toFixed(2)+', ry:'+(+s.ry).toFixed(2)+\n" +
	"                 ', rlx:'+(+s.rlx).toFixed(2)+', rly:'+(+s.rly).toFixed(2)+', rside:\"'+s.rside+'\"';\n" +
	"        }).join('\\n');\n" +
	"    }\n" +
	"    function render(){\n" +
	"      var fc2=document.getElementById('frameC'); if(fc2) fc2.classList.add('edit');\n" +
	"      if(ui) ui.editPuntos=true;\n" +
	"      var r=document.getElementById('__ed_room');\n" +
	"      var b=document.getElementById('__ed_body');\n" +
	"      if(ui.cuarto==null){ r.textContent='Ning\\u00fan cuarto abierto.'; b.innerHTML=''; return; }\n" +
	"      var nm=(typeof CUARTOS!=='undefined'&&CUARTOS.find)?(CUARTOS.find(function(c){return c.id===ui.cuarto;})||{}).nombre:'';\n" +
	"      r.innerHTML='<b>'+(nm||('Cuarto '+ui.cuarto))+'</b>';\n" +
	"      var html='';\n" +
	"      SENSORS.filter(function(s){return s.cuarto===ui.cuarto;}).forEach(function(s){\n" +
	"        html+='<tr><td>'+s.tag+'</td><td><b>'+(+s.rx).toFixed(2)+','+(+s.ry).toFixed(2)+\n" +
	"              '</b></td><td>'+(+s.rlx).toFixed(2)+','+(+s.rly).toFixed(2)+'</td><td>'+s.rside+'</td></tr>';\n" +
	"      });\n" +
	"      b.innerHTML=html;\n" +
	"    }\n" +
	"    function copy(txt,msg){\n" +
	"      var ok=document.getElementById('__ed_ok'); var ta=document.getElementById('__ed_ta');\n" +
	"      ta.style.display='block'; ta.value=txt; ta.select();\n" +
	"      try{ (navigator.clipboard&&navigator.clipboard.writeText)?navigator.clipboard.writeText(txt):document.execCommand('copy'); ok.textContent=msg+' (copiado)'; }\n" +
	"      catch(e){ ok.textContent=msg+' (selecciona y Ctrl+C)'; }\n" +
	"    }\n" +
	"    document.getElementById('__ed_copy').onclick=function(){\n" +
	"      if(ui.cuarto==null){ document.getElementById('__ed_ok').textContent='Abr\\u00ed un cuarto primero.'; return; }\n" +
	"      copy(rows(false),'Cuarto '+ui.cuarto);\n" +
	"    };\n" +
	"    document.getElementById('__ed_all').onclick=function(){ copy(rows(true),'Todos los cuartos'); };\n" +
	"    document.getElementById('__ed_reset').onclick=function(){\n" +
	"      try{ localStorage.removeItem('panccadia.puntos.v11'); }catch(e){}\n" +
	"      document.getElementById('__ed_ok').textContent='Respaldo local borrado. Recargando...';\n" +
	"      setTimeout(function(){ location.reload(); },500);\n" +
	"    };\n" +
	"    setInterval(render,250); render();\n" +
	"  }\n" +
	"  ready();\n" +
	"})();\n" +
	"</script>\n"

// configLoginSnippet is CONFIG_LOGIN_SNIPPET, verbatim (__PREFIX__ is substituted).
const configLoginSnippet = "\n" +
	"<style id=\"__cl_css\">\n" +
	"  #__cl{position:fixed;inset:0;z-index:2147483000;display:none;align-items:center;justify-content:center;\n" +
	"        background:rgba(27,28,23,.55)}\n" +
	"  #__cl.open{display:flex}\n" +
	"  #__cl .card{width:min(440px,92vw);background:var(--surface,#fff);border:1px solid var(--line,#e2ded3);\n" +
	"        box-shadow:0 30px 90px rgba(27,28,23,.35);font-family:inherit;color:var(--ink,#1b1c17)}\n" +
	"  #__cl .ch{padding:13px 20px;border-bottom:1px solid var(--line-2,#eeebe3);font-family:var(--display,inherit);font-size:16px}\n" +
	"  #__cl .body{padding:16px 20px 18px}\n" +
	"  #__cl p{margin:0 0 11px;font-size:11px;line-height:1.8;color:var(--muted,#8b8a7c)}\n" +
	"  #__cl label{display:block;font-size:10px;letter-spacing:.12em;text-transform:uppercase;color:var(--muted,#8b8a7c);margin:8px 0 4px}\n" +
	"  #__cl input{width:100%;box-sizing:border-box;min-height:44px;font-size:15px;padding:8px 12px;background:var(--surface-2,#faf9f5);\n" +
	"        border:1px solid var(--line,#e2ded3);color:var(--ink,#1b1c17)}\n" +
	"  #__cl .acts{display:flex;gap:8px;margin-top:14px;align-items:center;justify-content:flex-end}\n" +
	"  #__cl button{min-height:44px;min-width:110px;padding:8px 16px;font-size:13px;letter-spacing:.06em;cursor:pointer;\n" +
	"        border:1px solid var(--line,#e2ded3);background:var(--surface-2,#faf9f5);color:var(--ink,#1b1c17)}\n" +
	"  #__cl button.pri{background:var(--sage,#6c715d);border-color:var(--sage-d,#545948);color:#fff}\n" +
	"  #__cl .err{display:none;color:var(--alarm,#b23f2f);font-size:11px;margin-top:8px}\n" +
	"  #__cl .err.on{display:block}\n" +
	"  #__chip{position:fixed;top:64px;right:12px;z-index:2147482999;display:none;align-items:center;gap:10px;\n" +
	"        background:var(--surface,#fff);border:1px solid var(--line,#e2ded3);padding:6px 10px 6px 12px;font-size:11px;\n" +
	"        letter-spacing:.06em;color:var(--ink,#1b1c17);box-shadow:0 8px 24px rgba(27,28,23,.18)}\n" +
	"  #__chip.on{display:flex}\n" +
	"  #__chip b{color:var(--sage-d,#545948)}\n" +
	"  #__chip button{min-height:34px;padding:4px 10px;font-size:11px;cursor:pointer;border:1px solid var(--line,#e2ded3);\n" +
	"        background:var(--surface-2,#faf9f5);color:var(--ink,#1b1c17)}\n" +
	"  #__log{display:none!important;position:fixed;right:12px;bottom:12px;z-index:2147482998;width:min(520px,94vw);background:var(--surface,#fff);\n" +
	"        border:1px solid var(--line,#e2ded3);box-shadow:0 8px 24px rgba(27,28,23,.18);font-size:11px;color:var(--ink,#1b1c17)}\n" +
	"  #__log .h{padding:6px 10px;border-bottom:1px solid var(--line-2,#eeebe3);letter-spacing:.12em;text-transform:uppercase;\n" +
	"        color:var(--muted,#8b8a7c);font-size:10px;cursor:pointer;display:flex;justify-content:space-between}\n" +
	"  #__log table{width:100%;border-collapse:collapse;font-family:var(--mono,monospace);font-size:10px}\n" +
	"  #__log td{padding:3px 8px;border-bottom:1px solid var(--line-2,#eeebe3);white-space:nowrap}\n" +
	"  #__log.min table{display:none}\n" +
	"</style>\n" +
	"<div id=\"__cl\" role=\"dialog\" aria-modal=\"true\"><div class=\"card\">\n" +
	"  <div class=\"ch\">Confirmar identidad para escribir</div>\n" +
	"  <div class=\"body\">\n" +
	"    <p>Esta pantalla usa una sesi&oacute;n compartida del panel. Para <b>cambiar un valor</b> se pide tu usuario de la\n" +
	"       estaci&oacute;n: el cambio queda registrado a tu nombre. La sesi&oacute;n de configuraci&oacute;n dura\n" +
	"       <b><span id=\"__cl_ttl\">--</span></b> y se cierra al pulsar <b>Salir</b>.</p>\n" +
	"    <label for=\"__cl_u\">Usuario (estaci&oacute;n)</label><input id=\"__cl_u\" autocomplete=\"username\" autocapitalize=\"off\">\n" +
	"    <label for=\"__cl_p\">Contrase&ntilde;a</label><input id=\"__cl_p\" type=\"password\" autocomplete=\"current-password\">\n" +
	"    <div class=\"err\" id=\"__cl_err\">Credenciales inv&aacute;lidas. Int&eacute;ntalo de nuevo.</div>\n" +
	"    <div class=\"acts\"><button type=\"button\" id=\"__cl_cancel\">Cancelar</button><button type=\"button\" class=\"pri\" id=\"__cl_ok\">Entrar</button></div>\n" +
	"  </div></div></div>\n" +
	"<div id=\"__chip\"><span>&#128274; <b id=\"__chip_u\"></b> &middot; <span id=\"__chip_t\">--:--</span></span><button type=\"button\" id=\"__chip_out\">Salir</button></div>\n" +
	"<div id=\"__log\" class=\"min\"><div class=\"h\" id=\"__log_h\"><span>Registro de cambios (mock change_log, superficie B)</span><span id=\"__log_n\">0</span></div><table><tbody id=\"__log_b\"></tbody></table></div>\n" +
	"<script id=\"__cl_js\">\n" +
	"(function(){\n" +
	"  var P=\"__PREFIX__\", sess={active:false,user:null,remaining:0}, pending=null, tick=null;\n" +
	"  var $=function(i){return document.getElementById(i)};\n" +
	"  function xhr(){return {\"X-Requested-With\":\"XMLHttpRequest\",\"Content-Type\":\"application/json\"}}\n" +
	"  var rawFetch=window.fetch.bind(window);\n" +
	"  function fmt(s){s=Math.max(0,s|0);return (s/60|0)+\":\"+(\"0\"+(s%60)).slice(-2)}\n" +
	"  function paint(){ $(\"__chip\").classList.toggle(\"on\",!!sess.active); if(sess.active){$(\"__chip_u\").textContent=sess.user;$(\"__chip_t\").textContent=fmt(sess.remaining);} }\n" +
	"  function refresh(){ return rawFetch(P+\"/api/config/session\",{headers:xhr()}).then(function(r){return r.json()}).then(function(j){sess=j;paint();$(\"__cl_ttl\").textContent=fmt(j.ttl||0);return j}).catch(function(){}) }\n" +
	"  function loop(){ clearInterval(tick); tick=setInterval(function(){ if(sess.active){sess.remaining--; if(sess.remaining<=0){sess.active=false;refresh();} paint();} },1000) }\n" +
	"  function open(){ $(\"__cl_err\").classList.remove(\"on\"); $(\"__cl_p\").value=\"\"; $(\"__cl\").classList.add(\"open\"); setTimeout(function(){($(\"__cl_u\").value?$(\"__cl_p\"):$(\"__cl_u\")).focus()},50) }\n" +
	"  function close(){ $(\"__cl\").classList.remove(\"open\") }\n" +
	"  function isWrite(url,init){ try{ var u=new URL(url,location.href); return (init&&String(init.method||\"GET\").toUpperCase()===\"POST\") && u.pathname.indexOf(P+\"/api/\")===0; }catch(e){return false} }\n" +
	"  window.fetch=function(url,init){\n" +
	"    if(!isWrite(url,init)) return rawFetch(url,init);\n" +
	"    if(sess.active) return rawFetch(url,init).then(function(r){ if(r.status===403){ return hold(url,init); } logRefresh(); return r; });\n" +
	"    return hold(url,init);\n" +
	"  };\n" +
	"  function hold(url,init){ return new Promise(function(res,rej){ pending={url:url,init:init,res:res,rej:rej}; open(); }) }\n" +
	"  $(\"__cl_cancel\").onclick=function(){ close(); if(pending){ var p=pending; pending=null; p.res(new Response(JSON.stringify({ok:false,error:\"config_login_cancelled\"}),{status:403,headers:{\"Content-Type\":\"application/json\"}})); } };\n" +
	"  $(\"__cl_ok\").onclick=function(){\n" +
	"    var u=$(\"__cl_u\").value.trim(), pw=$(\"__cl_p\").value;\n" +
	"    rawFetch(P+\"/api/config/login\",{method:\"POST\",headers:xhr(),body:JSON.stringify({user:u,pass:pw})}).then(function(r){\n" +
	"      if(r.status!==200){ $(\"__cl_err\").classList.add(\"on\"); $(\"__cl_p\").value=\"\"; $(\"__cl_p\").focus(); return; }\n" +
	"      return r.json().then(function(j){ sess={active:true,user:j.user,remaining:j.ttl,ttl:j.ttl}; paint(); loop(); close();\n" +
	"        if(pending){ var p=pending; pending=null; rawFetch(p.url,p.init).then(function(rr){ logRefresh(); refresh(); p.res(rr); },p.rej); } });\n" +
	"    });\n" +
	"  };\n" +
	"  $(\"__cl_p\").addEventListener(\"keydown\",function(e){ if(e.key===\"Enter\") $(\"__cl_ok\").click(); });\n" +
	"  $(\"__chip_out\").onclick=function(){ rawFetch(P+\"/api/config/logout\",{method:\"POST\",headers:xhr()}).then(function(){ sess={active:false,user:null,remaining:0}; paint(); }) };\n" +
	"  $(\"__log_h\").onclick=function(){ $(\"__log\").classList.toggle(\"min\") };\n" +
	"  function logRefresh(){ rawFetch(P+\"/__mock/change_log\",{headers:xhr()}).then(function(r){return r.json()}).then(function(rows){\n" +
	"    $(\"__log_n\").textContent=rows.length; var b=$(\"__log_b\"); b.innerHTML=\"\";\n" +
	"    rows.slice(-6).reverse().forEach(function(x){ var tr=document.createElement(\"tr\");\n" +
	"      tr.innerHTML=\"<td>\"+x.ts+\"</td><td>\"+x.user_email+\"</td><td>\"+(x.room||\"\")+\"/\"+x.slot+\"</td><td>\"+x.old_value+\" &rarr; \"+x.new_value+\"</td><td>\"+x.surface+\"</td>\"; b.appendChild(tr); });\n" +
	"    if(rows.length) $(\"__log\").classList.remove(\"min\");\n" +
	"  }).catch(function(){}) }\n" +
	"  refresh().then(loop); logRefresh();\n" +
	"})();\n" +
	"</script>\n"

// hmiHead and hmiTail surround the prefix in the /hmi frame.
const hmiHead = "<!doctype html><html><head><meta charset=\"utf-8\">\n" +
	"<title>HMI 1280x800 preview</title><style>\n" +
	" html,body{margin:0;height:100%;background:#0b0e12;color:#c7ced8;\n" +
	"   font:13px system-ui,sans-serif;display:flex;flex-direction:column;\n" +
	"   align-items:center;justify-content:center;overflow:hidden}\n" +
	" #cap{margin:10px 0 8px;letter-spacing:.12em;color:#8b93a1;font-size:11px}\n" +
	" #device{background:#171a1f;border:1px solid #2a2f37;border-radius:22px;padding:22px;\n" +
	"   box-shadow:0 20px 60px -20px #000;transform-origin:top center}\n" +
	" #dot{width:7px;height:7px;border-radius:50%;background:#2a2f37;margin:0 auto 12px}\n" +
	" iframe{width:1280px;height:800px;border:0;display:block;background:#fff;border-radius:4px}\n" +
	"</style></head><body>\n" +
	"<div id=\"cap\">HMI 10.1&quot; capacitivo &middot; 1280&times;800</div>\n" +
	"<div id=\"device\"><div id=\"dot\"></div><iframe src=\""

const hmiTail = "/\" title=\"dashboard\"></iframe></div>\n" +
	"<script>var d=document.getElementById('device');\n" +
	" function fit(){d.style.transform='scale('+Math.min(1,(innerWidth-60)/1324,(innerHeight-96)/892)+')';}\n" +
	" addEventListener('resize',fit);fit();</script></body></html>"

// errorPage is http.server.DEFAULT_ERROR_MESSAGE split at code, message, code, explain.
var errorPage = [5]string{
	"<!DOCTYPE HTML>\n" +
		"<html lang=\"en\">\n" +
		"    <head>\n" +
		"        <meta charset=\"utf-8\">\n" +
		"        <style type=\"text/css\">\n" +
		"            :root {\n" +
		"                color-scheme: light dark;\n" +
		"            }\n" +
		"        </style>\n" +
		"        <title>Error response</title>\n" +
		"    </head>\n" +
		"    <body>\n" +
		"        <h1>Error response</h1>\n" +
		"        <p>Error code: ",
	"</p>\n" +
		"        <p>Message: ",
	".</p>\n" +
		"        <p>Error code explanation: ",
	" - ",
	".</p>\n" +
		"    </body>\n" +
		"</html>\n",
}

// mimeTypes is MIME.
var mimeTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "application/javascript; charset=utf-8",
	".json":  "application/json",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".svg":   "image/svg+xml",
	".ico":   "image/x-icon",
	".map":   "application/json",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".eot":   "application/vnd.ms-fontobject",
}
