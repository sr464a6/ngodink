async function post(u,b){try{const r=await fetch(u,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)});return await r.json()}catch(e){return{ok:false,msg:'Koneksi bermasalah'}}}
function showOut(r,label){const o=document.getElementById('out');o.hidden=false;o.className='out '+(r.ok?'ok':'bad');o.textContent=r.ok?(r.out!==undefined?'✓ Berhasil\n'+(label?label+':\n':'')+r.out:'✓ '+r.msg):'✗ '+(r.msg||'Gagal')}
document.querySelectorAll('textarea.editor').forEach(t=>t.addEventListener('keydown',e=>{if(e.key==='Tab'){e.preventDefault();const s=t.selectionStart;t.value=t.value.slice(0,s)+'    '+t.value.slice(t.selectionEnd);t.selectionStart=t.selectionEnd=s+4}}));
(function(){const b=document.getElementById('aiBtn');if(!b)return;const box=document.getElementById('aiBox'),msgs=document.getElementById('aiMsgs'),inp=document.getElementById('aiIn');
const add=(t,c)=>{const d=document.createElement('div');d.className='m '+c;d.textContent=t;msgs.appendChild(d);msgs.scrollTop=msgs.scrollHeight;return d};
b.onclick=()=>{box.hidden=false;b.hidden=true;inp.focus()};document.getElementById('aiX').onclick=()=>{box.hidden=true;b.hidden=false};
async function go(){const q=inp.value.trim();if(!q)return;inp.value='';add(q,'me');const w=add('…','bot');const c=window.CTX||{};const r=await post('/api/ask',{lang:c.lang||'',topic:c.topic||'',q});w.textContent=r.a||r.msg||'Gagal'}
document.getElementById('aiGo').onclick=go;inp.onkeydown=e=>{if(e.key==='Enter')go()}})();
