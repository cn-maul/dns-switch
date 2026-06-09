# DNS-Switch 设计书（Go 1.26+ / TOML / 并发）

---

## 1. 功能边界

二进制启动后进入交互式 REPL，用户输入 `/` 开头的命令，执行完不退出，等待下一条输入。

| 命令 | 作用 |
| --- | --- |
| `/test` | 并发探测配置中所有 DNS 延迟，输出排序表格 |
| `/set <name>` | 将指定 DNS 写入系统当前网卡 |
| `/restore` | 恢复为切换前状态（DHCP） |
| `/help` | 列出所有命令 |
| `/exit` | 退出程序 |

---

## 2. 配置文件（TOML）

### 2.1 路径

```
Linux:   ~/.config/dns-switch/config.toml
Windows: %APPDATA%/dns-switch/config.toml
```

### 2.2 结构

```toml
[servers]
AliDNS      = "223.5.5.5"
114DNS      = "114.114.114.114"
Google      = "8.8.8.8"
Cloudflare  = "1.1.1.1"

# ↓ 以下程序自动维护，用户不编辑

[last_test]
optimal = "Cloudflare"
rtt_ms  = 12.3
time    = "2026-01-15T10:30:00Z"

[backup]
dns     = ["192.168.1.1"]
backend = "NetworkManager"
```

### 2.3 设计要点

- `[servers]` 键即名字，`set Cloudflare` → `servers["Cloudflare"]` 查 IPv4，大小写不敏感
- 名字重复时 TOML 解析器直接报错
- `last_test` / `backup` 程序维护，用户不碰

### 2.4 示例配置文件

```toml
[servers]
# 国内
AliDNS      = "223.5.5.5"
114DNS      = "114.114.114.114"
TencentDNS  = "119.29.29.29"

# 国际
Google      = "8.8.8.8"
Cloudflare  = "1.1.1.1"
Quad9       = "9.9.9.9"
```

### 2.5 对应 Go 结构体

```go
type Config struct {
    Servers  map[string]string `toml:"servers"`
    LastTest LastTest           `toml:"last_test"`
    Backup   *Backup            `toml:"backup"`
}

type LastTest struct {
    Optimal string  `toml:"optimal"`
    RTTMs   float64 `toml:"rtt_ms"`
    Time    string  `toml:"time"`
}

type Backup struct {
    DNS     []string `toml:"dns"`
    Backend string   `toml:"backend"`
}
```

---

## 3. REPL 交互设计

### 3.1 入口

```
dns-switch          # 无参数启动，进入 REPL
```

无 flag，无子命令。启动后：

```
> _
```

提示符为 `> `，等待 `/` 开头的命令。

### 3.2 命令

| 输入 | 说明 |
| --- | --- |
| `/test` | 并发测速所有 DNS，输出表格 |
| `/set <name>` | 切换 DNS 到名字对应的地址 |
| `/restore` | 恢复 DHCP |
| `/help` | 列出命令表 |
| `/exit` | 退出 |

- 命令名大小写不敏感（`/Test` = `/test`）
- 非 `/` 开头的输入忽略并重新显示提示符
- 未知命令输出 `unknown command`，不回显用法

### 3.3 输出规范

```
# 成功 → stdout
格式: OK <msg>

# 错误 → stderr
格式: ERR <msg>
```

| 场景 | 输出 |
| --- | --- |
| test 完成 | 表格 + 最优行（见 §3.3） |
| set 成功 | `OK Cloudflare → eth0` |
| set 找不到名字 | `ERR unknown name "xxx"` |
| set 权限不足 | `ERR need root/Administrator` |
| restore 成功 | `OK eth0 restored to DHCP` |
| restore 无备份 | `ERR no backup found` |

**原则**：无 emoji、无 spinner、无进度条、无颜色。

### 3.4 `/test` 输出格式

```
NAME          RTT(ms)   LOSS
Cloudflare       12.3     0%
AliDNS           18.7     0%
Google           23.1     0%
114DNS           31.2     0%
Quad9          error     100%
---
best: Cloudflare 12.3ms
```

- 四列：NAME（左对齐）、RTT(ms)（右对齐）、LOSS（右对齐）
- 超时/不可达显示 `error`
- 分隔线后输出最优
- 用 `text/tabwriter` 对齐，不引入 table 库

---

## 4. DNS 并发测速模块

### 4.1 算法

```
输入: config.toml 中 [servers]
参数: 每台 3 次, 超时 2s, 目标 resolve1.opendns.com

每台 DNS:
  for i in 1..3:
    UDP 向 <ip>:53 发 A 查询（目标域名）
    超时 2s → 丢包
    成功 → 记录 RTT
  去极值后取平均 RTT 和丢包率

所有 DNS 并发执行
```

### 4.2 实现

```go
func benchmarkAll(servers map[string]string) []result {
    results := make([]result, 0, len(servers))
    var mu sync.Mutex
    var wg sync.WaitGroup

    for name, ip := range servers {
        wg.Go(func() {
            rtt, ok := measureOne(ip, 3, 2*time.Second)
            mu.Lock()
            if ok {
                results = append(results, result{Name: name, AvgRTT: rtt})
            } else {
                results = append(results, result{Name: name, Err: true})
            }
            mu.Unlock()
        })
    }
    wg.Wait()

    slices.SortFunc(results, func(a, b result) int {
        return cmp.Compare(a.AvgRTT, b.AvgRTT)
    })
    return results
}
```

**Go 1.26+ 特性使用**：

| 特性 | 位置 | 作用 |
| --- | --- | --- |
| `sync.WaitGroup.Go` | 并发探测 | `Add(1)` + `go func() { defer Done() }` 合为一步 |
| `slices.SortFunc` | 结果排序 | `sort.Slice` 的泛型替代，零闭包分配 |
| `cmp.Compare` | 排序比较 | 标准库三路比较 |
| `net.Resolver` 自定义 Dial | UDP 指定 DNS | 纯标准库，不引入第三方 DNS 包 |

### 4.3 单台测速

```go
func measureOne(dnsIP string, n int, timeout time.Duration) (time.Duration, bool) {
    resolver := &net.Resolver{
        PreferGo: true,
        Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
            return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "udp", dnsIP+":53")
        },
    }
    var total time.Duration
    ok := 0
    for range n {
        start := time.Now()
        _, err := resolver.LookupHost(context.Background(), "resolve1.opendns.com")
        if err == nil {
            total += time.Since(start)
            ok++
        }
    }
    if ok == 0 {
        return 0, false
    }
    return total / time.Duration(ok), true
}
```

### 4.4 结果持久化

测完写回 `config.toml` 的 `[last_test]`：

```toml
[last_test]
optimal = "Cloudflare"
rtt_ms  = 12.3
time    = "2026-01-15T10:30:00Z"
```

---

## 5. 平台适配模块

### 5.1 接口

```go
type Backend interface {
    SetDNS(iface string, dns string) error
    RestoreDNS(iface string) error
    DefaultIface() (string, error)
}
```

三个方法，每个平台一个实现文件，`_linux.go` / `_windows.go` 编译隔离。

### 5.2 Linux 检测链

```
systemd-resolved → NetworkManager → resolv.conf
```

| 后端 | 检测 | 设置 | 恢复 |
| --- | --- | --- | --- |
| systemd-resolved | `/run/systemd/resolve/stub-resolv.conf` 存在 | `resolvectl dns <iface> <ip>` | `resolvectl revert <iface>` |
| NetworkManager | `nmcli -t -f RUNNING general` = running | `nmcli con mod <con> ipv4.dns <ip>` → `nmcli con up <con>` | `nmcli con mod <con> ipv4.dns ""` → up |
| resolv.conf | `/etc/resolv.conf` 非 symlink | 备份 → 写 `nameserver <ip>` | 还原备份 |

### 5.3 Windows

**注册表直接写入**，不调 `netsh`。

```
路径: HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\{GUID}
键:   NameServer (REG_SZ)
```

流程：

```go
// 1. iphlpapi.GetBestInterface → 拿到 interface index
// 2. iphlpapi.GetAdaptersAddresses → 匹配拿到 GUID
// 3. registry.OpenKey + SetStringValue("NameServer", "1.1.1.1")
// 4. 恢复: DeleteValue("NameServer")
```

仅依赖 `golang.org/x/sys/windows`。

### 5.4 文件布局

```
platform/
├── backend.go       // Backend 接口
├── linux.go         // _linux 编译
└── windows.go       // _windows 编译
```

---

## 6. 备份恢复机制

### 6.1 `/set` 流程

```
1. 读 config.toml → servers 中匹配 name（大小写不敏感）
2. 未找到 → ERR + exit
3. 检测 Backend → 取 DefaultIface
4. 写 config.toml [backup]:
     dns     = 保留字段，无当前 DNS 时写空数组
     backend = Backend 名
5. Backend.SetDNS(iface, ip)
6. 输出: OK <name> → <iface>
```

### 6.2 `/restore` 流程

```
1. 读 config.toml → [backup] 不存在 → ERR
2. 检测 Backend
3. Backend.RestoreDNS(iface)
4. 删除 config.toml 中 [backup] 段
5. 输出: OK <iface> restored to DHCP
```

### 6.3 策略

- **写前备份、恢复后清空**，覆盖式，不保留历史
- **统一恢复到 DHCP**，不尝试还原旧静态 DNS — 简单可靠
- 权限不足在任何一步都直接报错退出，不留半完成状态

---

## 7. 项目结构

```
dns-switch/
├── main.go              # REPL 入口，读取输入 → 调度命令
├── shell.go             # 交互循环，提示符 + 行读取 + 命令解析
├── config.go            # TOML 读取/写入
├── bench.go             # 并发测速
├── output.go            # 表格输出
├── platform/
│   ├── backend.go       # Backend 接口
│   ├── linux.go         # Linux 三后端
│   └── windows.go       # Windows 注册表
├── go.mod
├── go.sum
└── config.example.toml
```

**10 个源文件。** 命令逻辑直接在 `main.go` 的 switch 中 dispatch，不拆多个文件——每个命令体不超过 30 行。

---

## 8. 依赖

| 包 | 用途 |
| --- | --- |
| `github.com/BurntSushi/toml` | TOML 解析 |
| `golang.org/x/sys` | Windows 注册表 + iphlpapi |
| 标准库 | `flag` `net` `os` `fmt` `time` `sync` `slices` `cmp` `text/tabwriter` |

**两个外部依赖。**

---

## 9. Go 1.26+ 特性应用汇总

| 特性 | 用途 | 旧写法 |
| --- | --- | --- |
| `wg.Go(func())` | 并发探测 | `wg.Add(1); go func() { defer wg.Done() }()` |
| `slices.SortFunc` | 结果排序 | `sort.Slice(results, func(i, j int) bool {...})` |
| `cmp.Compare` | SortFunc 比较函数 | 手动 if-else |
| `maps.Keys` / `range maps.Keys(m)` (迭代器) | 遍历 servers | `for k := range m` (等价，新写法更语义化) |
| `net.Resolver.Dial` | 指定目标 DNS:53 | 第三方 DNS 库或手动构造 UDP 报文 |

---

## 10. 关键决策

| 决策 | 理由 |
| --- | --- |
| DNS 列表手写 TOML | 省去增删改命令 |
| restore 统一 DHCP | 避免"智能还原"边界 bug |
| REPL 交互不跑完退出 | 用户连续操作避免反复 sudo，切完立刻测，无启动开销 |
| 并发测速 | 体验提升 10x，Go 原生 `wg.Go` 代价极低 |
| Windows 注册表不用 netsh | 无 shell 依赖，无中文网卡名问题 |
| Backup 存文件不读系统 | 跨平台"读当前 DNS"实现差异大，存文件可靠 |
