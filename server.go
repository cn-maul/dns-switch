package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
)

const dashHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>DNS-Switch</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; padding: 20px; max-width: 700px; margin: 0 auto; }
h1 { font-size: 22px; margin-bottom: 16px; display: flex; align-items: center; gap: 8px; }
.status-bar { background: #fff; border-radius: 8px; padding: 12px 16px; margin-bottom: 16px; box-shadow: 0 1px 3px rgba(0,0,0,.1); display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.status-bar .label { color: #666; font-size: 13px; }
.status-bar .value { font-weight: 600; }
.actions { display: flex; gap: 10px; margin-bottom: 16px; flex-wrap: wrap; }
.btn { padding: 10px 20px; border: none; border-radius: 6px; font-size: 14px; cursor: pointer; transition: opacity .2s; }
.btn:hover { opacity: .85; }
.btn:disabled { opacity: .5; cursor: not-allowed; }
.btn-primary { background: #4a90d9; color: #fff; }
.btn-success { background: #34c759; color: #fff; }
.btn-danger { background: #ff3b30; color: #fff; }
table { width: 100%; background: #fff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,.1); border-collapse: collapse; overflow: hidden; }
th { background: #fafafa; font-size: 12px; color: #666; text-transform: uppercase; letter-spacing: .5px; padding: 10px 12px; text-align: left; border-bottom: 1px solid #eee; }
td { padding: 10px 12px; border-bottom: 1px solid #f5f5f5; font-size: 14px; }
tr:last-child td { border-bottom: none; }
tr:hover td { background: #fafafa; }
.best { font-weight: 600; }
.best td:first-child::before { content: "✅ "; }
.error { color: #ff3b30; }
.rtt { font-family: "SF Mono", Monaco, monospace; font-size: 13px; }
.set-btn { padding: 4px 12px; border: 1px solid #4a90d9; background: #fff; color: #4a90d9; border-radius: 4px; font-size: 12px; cursor: pointer; }
.set-btn:hover { background: #4a90d9; color: #fff; }
.set-btn:disabled { opacity: .4; cursor: not-allowed; }
.msg { position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%); padding: 10px 20px; border-radius: 6px; font-size: 14px; color: #fff; display: none; z-index: 99; }
.msg.ok { background: #34c759; display: block; }
.msg.err { background: #ff3b30; display: block; }
.loading { text-align: center; padding: 20px; color: #999; }
</style>
</head>
<body>
<h1>🌐 DNS-Switch</h1>

<div class="status-bar">
	<div><span class="label">当前 DNS</span><br><span class="value" id="currentDns">—</span></div>
	<div><span class="label">最优服务器</span><br><span class="value" id="bestServer">—</span></div>
	<div><span class="label">延迟</span><br><span class="value" id="bestRtt">—</span></div>
</div>

<div class="actions">
	<button class="btn btn-primary" id="btnTest" onclick="runTest()">📡 全部测速</button>
	<button class="btn btn-success" id="btnRestore" onclick="restoreDns()">↺ 恢复 DHCP</button>
</div>

<table>
<thead><tr><th>名称</th><th>地址</th><th>延迟</th><th>操作</th></tr></thead>
<tbody id="serverList"></tbody>
</table>

<div id="toast" class="msg"></div>

<script>
let servers = {};

async function loadServers() {
	try {
		const r = await fetch('/api/status');
		const d = await r.json();
		servers = d.servers || {};
		renderTable(d);
	} catch(e) {
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

	// Build sorted rows
	const rows = names.map(name => {
		const ip = data.servers[name];
		let rttStr = '—', lossStr = '', isBest = false, isErr = false;
		if (data.results && data.results[name]) {
			const r = data.results[name];
			if (r.err) { rttStr = r.errMsg; isErr = true; }
			else { rttStr = r.avgRtt.toFixed(1) + 'ms'; }
			if (r.best) isBest = true;
		}
		return { name, ip, rttStr, isBest, isErr };
	});

	// Sort: best first, then by name
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
		const r = await fetch('/api/test', { method: 'POST' });
		const d = await r.json();
		renderTable(d);
		if (d.bestServer) {
			showToast('测速完成！最优: ' + d.bestServer + ' ' + d.bestRtt.toFixed(1) + 'ms');
		} else if (d.error) {
			showToast(d.error, true);
		}
	} catch(e) {
		showToast('测速请求失败', true);
	} finally {
		btn.disabled = false;
		btn.textContent = '📡 全部测速';
	}
}

async function setDns(name) {
	try {
		const r = await fetch('/api/set?name=' + encodeURIComponent(name), { method: 'POST' });
		const d = await r.json();
		if (d.ok) {
			document.getElementById('currentDns').textContent = name;
			showToast('已切换到 ' + name);
		} else {
			showToast(d.error || '切换失败', true);
		}
	} catch(e) {
		showToast('请求失败', true);
	}
}

async function restoreDns() {
	try {
		const r = await fetch('/api/restore', { method: 'POST' });
		const d = await r.json();
		if (d.ok) {
			document.getElementById('currentDns').textContent = 'DHCP';
			showToast('已恢复为 DHCP');
		} else {
			showToast(d.error || '恢复失败', true);
		}
	} catch(e) {
		showToast('请求失败', true);
	}
}

function showToast(msg, isErr) {
	const el = document.getElementById('toast');
	el.textContent = msg;
	el.className = 'msg ' + (isErr ? 'err' : 'ok');
	setTimeout(() => { el.className = 'msg'; }, 3000);
}

loadServers();
// Auto-refresh every 30s
setInterval(loadServers, 30000);
</script>
</body>
</html>`

// statusResponse is the JSON structure returned by API endpoints.
type statusResponse struct {
	Servers    map[string]string          `json:"servers"`
	Results    map[string]resultEntry     `json:"results,omitempty"`
	BestServer string                     `json:"bestServer,omitempty"`
	BestRtt    float64                    `json:"bestRtt,omitempty"`
	CurrentDns string                     `json:"currentDns,omitempty"`
	Ok         bool                       `json:"ok,omitempty"`
	Error      string                     `json:"error,omitempty"`
}

type resultEntry struct {
	AvgRtt float64 `json:"avgRtt,omitempty"`
	Err    bool    `json:"err"`
	ErrMsg string  `json:"errMsg,omitempty"`
	Best   bool    `json:"best"`
}

// runServer starts the HTTP server and opens the browser.
func runServer() error {
	// Initialize currentDNS from config backup state
	if cfg, err := ReadConfig(); err == nil && cfg.Backup != nil {
		state.setCurrentDNS("已设置（查看备份记录）")
	} else {
		state.setCurrentDNS("DHCP")
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/servers", handleServers)
	http.HandleFunc("/api/test", handleTest)
	http.HandleFunc("/api/set", handleSet)
	http.HandleFunc("/api/restore", handleRestore)

	addr := "127.0.0.1:9753"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("启动服务器失败: %w", err)
	}

	fmt.Printf("🌐 DNS-Switch 面板已启动: http://%s\n", addr)
	openBrowser("http://" + addr)

	return http.Serve(ln, nil)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashHTML))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	cfg, err := ReadConfig()
	if err != nil {
		json.NewEncoder(w).Encode(statusResponse{Servers: map[string]string{}, Error: err.Error()})
		return
	}
	results, bestIdx := state.getResults()
	resp := statusResponse{
		Servers:    cfg.Servers,
		CurrentDns: state.getCurrentDNS(),
		Results:    buildResultMap(results, bestIdx),
	}
	if bestIdx >= 0 && bestIdx < len(results) {
		b := results[bestIdx]
		resp.BestServer = b.Name
		resp.BestRtt = b.AvgRTT
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleServers(w http.ResponseWriter, r *http.Request) {
	cfg, err := ReadConfig()
	if err != nil {
		json.NewEncoder(w).Encode(statusResponse{Servers: map[string]string{}, Error: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusResponse{Servers: cfg.Servers})
}

func handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	cfg, err := ReadConfig()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusResponse{Error: err.Error()})
		return
	}
	if len(cfg.Servers) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusResponse{Error: "没有配置 DNS 服务器"})
		return
	}

	RunBenchmark(cfg.Servers, func(results []BenchResult, idx int) {
		state.setResults(results, idx)
	})

	results, bestIdx := state.getResults()
	resp := statusResponse{Servers: cfg.Servers, Results: buildResultMap(results, bestIdx)}
	if bestIdx >= 0 && bestIdx < len(results) {
		b := results[bestIdx]
		resp.BestServer = b.Name
		resp.BestRtt = b.AvgRTT
		if err := SaveLastTest(b.Name, b.AvgRTT); err != nil {
			fmt.Printf("ERR save last test: %v\n", err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusResponse{Error: "缺少 name 参数"})
		return
	}

	cfg, err := ReadConfig()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusResponse{Error: err.Error()})
		return
	}

	ip, found := LookupServer(cfg.Servers, name)
	if !found {
		// Try as raw IP
		if net.ParseIP(name) == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(statusResponse{Error: fmt.Sprintf("未知名称 %q", name)})
			return
		}
		ip = name
	}

	if err := execSet([]string{ip}); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusResponse{Error: err.Error()})
		return
	}

	state.setCurrentDNS(name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusResponse{Ok: true, CurrentDns: name})
}

func handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	if err := execRestore(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statusResponse{Error: err.Error()})
		return
	}
	state.setCurrentDNS("DHCP")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusResponse{Ok: true})
}

// buildResultMap converts bench results into a name-keyed map for JSON responses.
func buildResultMap(results []BenchResult, bestIdx int) map[string]resultEntry {
	if len(results) == 0 {
		return nil
	}
	m := make(map[string]resultEntry, len(results))
	for i, r := range results {
		e := resultEntry{Err: r.Err, ErrMsg: r.ErrMsg, Best: i == bestIdx}
		if !r.Err {
			e.AvgRtt = r.AvgRTT
		}
		m[r.Name] = e
	}
	return m
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux and others
		cmd = "xdg-open"
		args = []string{url}
	}
	exec.Command(cmd, args...).Start() // ignore errors — fallback to manual open
}

// serverState holds the mutable server state protected by a mutex.
// All HTTP handlers run in their own goroutine — without this,
// concurrent /api/test + /api/status requests cause a data race.
type serverState struct {
	mu          sync.RWMutex
	benchResults []BenchResult
	bestIdx      int
	currentDNS   string
}

func (s *serverState) getResults() ([]BenchResult, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benchResults, s.bestIdx
}

func (s *serverState) setResults(results []BenchResult, idx int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.benchResults = results
	s.bestIdx = idx
}

func (s *serverState) getCurrentDNS() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentDNS
}

func (s *serverState) setCurrentDNS(dns string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentDNS = dns
}

var state serverState
