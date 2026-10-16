(async () => {
  window.requestAnimationFrame = callback => setTimeout(() => callback(performance.now()), 0); const css=document.createElement("style");css.textContent="* { transition-duration: 0s !important; animation-duration: 0s !important; }";document.head.append(css); const t = window.avatarTest, results = []
  const assert = (condition, name, detail) => { if (!condition) throw new Error(name + ': ' + JSON.stringify(detail)); results.push({name, passed:true, detail}) }
  const wait = async predicate => { for(let i=0;i<100;i++){if(predicate())return;await new Promise(r=>setTimeout(r,40))}throw new Error('UI等待超时') }
  const edit = async () => { [...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='编辑资料').click();await wait(()=>document.querySelector('input[type=file]')) }
  const choose = () => { const input=document.querySelector('input[type=file]'),dt=new DataTransfer();dt.items.add(new File([new Uint8Array([137,80,78,71])],'retry.png',{type:'image/png'}));input.files=dt.files;input.dispatchEvent(new Event('change',{bubbles:true})) }
  await edit()
  let pickerOpened=false;document.querySelector('input[type=file]').click=()=>{pickerOpened=true};document.querySelector('.account-avatar-edit').click()
  assert(pickerOpened,'头像按钮打开文件选择器')
  choose();await wait(()=>t.state.uploads.length===1)
  assert(t.account.accountUser.value.headerImg==='/avatar-old.png','上传中保留原头像')
  assert(document.querySelector('button[type=submit]').disabled && document.querySelector('input[type=file]').disabled,'上传中禁止再次选择和保存')
  document.querySelector('form').dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}))
  let duplicateRejected=false;try{await t.account.uploadAccountAvatar(new File(['x'],'dup.png',{type:'image/png'}))}catch{duplicateRejected=true}
  assert(duplicateRejected&&t.state.uploads.length===1&&t.state.saves.length===0,'重复上传和保存均被拦截')
  document.querySelector('.account-close').click();await wait(()=>!t.open.value);t.state.pending();await wait(()=>!t.account.accountAvatarUploading.value)
  assert(t.account.accountUser.value.headerImg==='/avatar-new.png'&&t.state.saves.length===0,'关闭后上传成功直接更新共享用户且无需保存')
  t.open.value=true;await t.nextTick();await wait(()=>[...document.querySelectorAll('button')].some(b=>b.textContent.trim()==='编辑资料'))
  assert(t.account.accountUser.value.headerImg==='/avatar-new.png','重新打开仍保留成功头像')
  await edit()
  for(const [selector,value] of [['input[autocomplete=nickname]','新昵称'],['input[type=email]','new@example.com'],['input[type=tel]','456']]){const input=document.querySelector(selector);input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}))}
  document.querySelector('form').dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));await wait(()=>t.state.saves.length===1&&!document.querySelector('input[type=file]'))
  assert(!Object.hasOwn(t.state.saves[0],'headerImg')&&t.account.accountUser.value.headerImg==='/avatar-new.png','后续昵称电话邮箱保存不回滚头像',t.state.saves[0])
  await edit();t.state.failure=true;choose();await wait(()=>t.state.uploads.length===2);t.state.pending();await wait(()=>!t.account.accountAvatarUploading.value);await t.nextTick()
  assert(t.account.accountUser.value.headerImg==='/avatar-new.png'&&document.querySelector('[role=alert]')?.textContent.includes('重试'),'失败保留头像且显示重试提示')
  assert(document.querySelector('input[type=file]').value==='','文件选择已清空，可重选同一文件')
  t.state.failure=false;choose();await wait(()=>t.state.uploads.length===3);t.state.pending();await wait(()=>!t.account.accountAvatarUploading.value);await t.nextTick()
  assert(!document.querySelector('.account-avatar-editor-copy [role=alert]'),'失败后同文件重试成功清除错误')
  const oldUser={...t.account.accountUser.value,headerImg:'/stale.png'};let finishRead;window.go.system.AccountService.GetCurrentUser=()=>new Promise(r=>finishRead=r)
  const read=t.account.fetchCurrentUser();choose();await wait(()=>t.state.uploads.length===4);t.state.pending();await wait(()=>!t.account.accountAvatarUploading.value);finishRead(oldUser);await read
  assert(t.account.accountUser.value.headerImg==='/avatar-new.png','上传之前发起的读取响应不覆盖新头像')
  choose();await wait(()=>t.state.uploads.length===5);window.go.system.AccountService.GetSessionState=async()=>({authenticated:false,needChangePassword:false});t.account.resetAccountSession();t.state.pending();await wait(()=>!t.account.accountAvatarUploading.value)
  assert(t.account.accountUser.value===null,'退出或切服后迟到上传响应不恢复旧用户')
  window.avatarUITestResults=results;return results
})()

