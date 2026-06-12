let servers = {};

async function loadServers() {
    try {
        const d = await wails.Call.ByName('main.DNSService.Status');
        servers = d.servers || {};
        renderTable(d);
    } catch (e) {
        showToast('无法连接服务器', true);
    }
}

function renderTable(data) {
    const names = Object.keys(data.servers || {});
    const tbody = document.getElementById('serverList');
    if (names.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center;color:#999;">没有配置 DNS 服务器</td></tr>';
        return;
    }
    const rows = names.map(name => {
        const ip = data.servers[name];
        let rttStr = '—', isBest = false, isErr = false;
        if (data.results && data.results[name]) {
            const r = data.results[name];
            if (r.err) { rttStr = r.errMsg; isErr = true; }
            else { rttStr = r.avgRtt.toFixed(1) + 'ms'; }
            if (r.best) isBest = true;
        }
        return { name, ip, rttStr, isBest, isErr };
    });
    rows.sort((a, b) => {
        if (a.isBest && !b.isBest) return -1;
        if (!a.isBest && b.isBest) return 1;
        return a.name.localeCompare(b.name);
    });
    tbody.innerHTML = rows.map(r => {
        const cls = r.isBest ? ' class="best"' : '';
        const rttCls = r.isErr ? ' class="rtt error"' : ' class="rtt"';
        return '<tr' + cls + '><td>' + r.name + '</td><td>' + r.ip + '</td><td' + rttCls + '>' + r.rttStr + '</td><td><button class="set-btn" onclick="setDns(\'' + r.name + '\')">切换</button></td></tr>';
    }).join('');
    document.getElementById('currentDns').textContent = data.currentDns || '—';
    if (data.bestServer) {
        document.getElementById('bestServer').textContent = data.bestServer;
        document.getElementById('bestRtt').textContent = data.bestRtt ? data.bestRtt.toFixed(1) + 'ms' : '—';
    } else {
        document.getElementById('bestServer').textContent = '—';
        document.getElementById('bestRtt').textContent = '—';
    }
}

async function runTest() {
    const btn = document.getElementById('btnTest');
    btn.disabled = true;
    btn.textContent = '⏳ 测速中...';
    showToast('测速中，请稍候...');
    try {
        const d = await wails.Call.ByName('main.DNSService.Test');
        renderTable(d);
        if (d.bestServer) showToast('测速完成！最优: ' + d.bestServer + ' ' + d.bestRtt.toFixed(1) + 'ms');
        else if (d.error) showToast(d.error, true);
    } catch (e) {
        showToast('测速请求失败', true);
    } finally {
        btn.disabled = false;
        btn.textContent = '📡 全部测速';
    }
}

async function setDns(name) {
    try {
        const d = await wails.Call.ByName('main.DNSService.Set', name);
        if (d.ok) {
            document.getElementById('currentDns').textContent = name;
            showToast('已切换到 ' + name);
        } else {
            showToast(d.error || '切换失败', true);
        }
    } catch (e) {
        showToast('请求失败', true);
    }
}

async function restoreDns() {
    try {
        const d = await wails.Call.ByName('main.DNSService.Restore');
        if (d.ok) {
            document.getElementById('currentDns').textContent = 'DHCP';
            showToast('已恢复为 DHCP');
        } else {
            showToast(d.error || '恢复失败', true);
        }
    } catch (e) {
        showToast('请求失败', true);
    }
}

function showToast(msg, isErr) {
    const el = document.getElementById('toast');
    el.textContent = msg;
    el.className = 'msg ' + (isErr ? 'err' : 'ok');
    setTimeout(() => { el.className = 'msg'; }, 3000);
}

// Wait for the Wails runtime to be ready
window['_wails']?.ready?.then(() => {
    loadServers();
    setInterval(loadServers, 30000);
});
