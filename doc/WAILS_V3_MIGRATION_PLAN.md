# DNS-Switch → Wails v3 改造计划书

> 版本: 1.0  
> 日期: 2026-07-08  
> 目标: 将当前 Go + 内嵌前端 的项目改造为 Wails v3 桌面应用，同时保留 CLI

---

## 一、现状分析

### 1.1 当前架构

```
main.go                 入口 → 分发 server / CLI 命令
server.go               HTTP 面板启动器
commands.go             CLI 子命令 (test / set / restore)
internal/
  server/server.go      HTTP server + 内嵌 HTML/JS/CSS 前端 (dashHTML 常量, ~160行)
  bench/bench.go        DNS 并发测速 (纯计算, 无副作用)
  config/               TOML 配置文件管理 (Store 接口 + FileStore 实现)
  dns/dns.go            DNS 切换编排 (Manager)
  notify/notify.go      桌面通知接口 (Notifier + Noop)
platform/
  backend.go            Backend 接口定义
  windows.go            Windows 注册表 DNS 操作
  linux.go              Linux systemd-resolved / NM / resolv.conf DNS 操作
```

### 1.2 依赖

```
go 1.26
github.com/pelletier/go-toml/v2
golang.org/x/sys
```

纯 Go, 当前零 CGO，编译产物 ~8MB。

### 1.3 前端特点

- 单文件内嵌在 Go 常量 `dashHTML` 中 (~160 行)
- vanilla HTML + CSS + JS，无框架
- 4 个 API 端点：`/api/status`, `/api/test`, `/api/set`, `/api/restore`

---

## 二、改造目标

| 目标 | 说明 |
|------|------|
| 桌面 GUI | Wails v3 原生窗口，替代浏览器访问 http://127.0.0.1:9753 |
| 保留 CLI | `dns-switch test/set/restore` 命令行模式不变 |
| 零 CGO | 编译时 CGO_ENABLED=0，利用 Wails v3 的 purego |
| Linux 静态交付 | 通过 AppImage 打包 GTK4 + WebKitGTK 运行时依赖 |
| 代码复用 | platform/、bench/、config/、dns/ 全部零改动 |
| 前端独立 | 从 Go 常量提取为 `frontend/` 目录，支持热重载开发 |

---

## 三、CGO 与静态编译策略

### 3.1 Wails v3 的 CGO 情况

Wails v3 使用 `ebitengine/purego` 在运行时动态加载系统原生库：

| 平台 | 运行时库 | 来源 | 是否需要安装 |
|------|---------|------|-------------|
| Windows | WebView2 | 系统预装 (Win10+) | 否 |
| macOS | WebKit | 系统预装 | 否 |
| Linux | GTK4 + WebKitGTK 6.0 | 需额外安装 | **是** |

编译时：全部平台都是纯 Go 编译，`CGO_ENABLED=0` 可行。

### 3.2 Linux 策略：AppImage

不把 GTK4/WebKitGTK 静态编译进 Go 二进制（技术上几乎不可行），而是：

```
AppImage 包结构:
  dns-switch.AppImage
  ├── dns-switch (Go 二进制, CGO_ENABLED=0)
  ├── libgtk-4.so.1      ← 从系统或 CI 环境提取
  ├── libwebkitgtk-6.0.so.9
  ├── 及其传递依赖...
  └── AppRun (设置 LD_LIBRARY_PATH)
```

构建方式：使用 `linuxdeploy` + `linuxdeploy-plugin-gtk` 自动收集所有 .so。

### 3.3 各平台最终交付

| 平台 | 交付格式 | 大小 (估) | 运行时依赖 |
|------|---------|-----------|-----------|
| Windows | `.exe` + NSIS 安装器 | ~15MB | WebView2 (系统自带) |
| macOS | `.app` bundle | ~15MB | WebKit (系统自带) |
| Linux | `.AppImage` | ~80MB | 无 (自包含) |

---

## 四、实施步骤

### Phase 1: 项目初始化 (预估 0.5h)

**Step 1.1** — 创建 Wails v3 项目骨架

```bash
# 在当前仓库根目录初始化 Wails v3
wails3 init -n dns-switch -t vanilla --reinit
```

- 生成 `wails.json`、`Taskfile.yml`、`frontend/`、`build/` 等
- 选择 `vanilla` 模板（无框架，与现有前端风格一致）

**Step 1.2** — 更新 `go.mod`

```
module dns-switch  →  保持不变

新增依赖:
  github.com/wailsapp/wails/v3  (latest alpha)
```

- 运行 `go mod tidy`
- 验证 `go build ./...` 通过

---

### Phase 2: 提取前端 (预估 0.5h)

**Step 2.1** — 从 `dashHTML` 常量拆出独立文件

将 `internal/server/server.go` 第 25-166 行的内嵌 HTML 拆分为：

```
frontend/
├── index.html          ← <head> + <body> 结构
├── style.css           ← <style> 块
├── main.js             ← <script> 块
└── package.json        ← npm 配置 (可选，仅 dev server 用)
```

**Step 2.2** — 更新 JS 调用方式

原有 4 个 `fetch('/api/xxx')` 调用改为 Wails v3 的 Service Binding 调用：

```
旧: fetch('/api/status')  → 新: wails.Call.ByName('main.DNSService.Status')
旧: fetch('/api/test')    → 新: wails.Call.ByName('main.DNSService.Test')
旧: fetch('/api/set')     → 新: wails.Call.ByName('main.DNSService.Set')
旧: fetch('/api/restore') → 新: wails.Call.ByName('main.DNSService.Restore')
```

- 不再需要 HTTP 端口 (9753)
- 调用变为进程内 IPC，延迟更低
- 保留手动 `fetch` 的 fallback (用于 server build 模式)

---

### Phase 3: 创建 Wails Service (预估 1h)

**Step 3.1** — 新建 `internal/service/dnsservice.go`

```go
// Package service provides the Wails v3 Service binding for DNS operations.
package service

type DNSService struct {
    cfgStore config.Store
    dnsMgr   *dns.Manager
    // benchmark results cache (替代 serverState)
    mu       sync.RWMutex
    results  []bench.Result
    bestIdx  int
}
```

绑定方法（每个对应原有 HTTP handler）：

| HTTP Handler | Service Method |
|-------------|----------------|
| `handleStatus()` | `Status() *StatusResponse` |
| `handleTest()` | `Test() *StatusResponse` |
| `handleSet(name)` | `Set(name string) *SetResponse` |
| `handleRestore()` | `Restore() *SimpleResponse` |

**Step 3.2** — 数据模型复用

保留 `statusResponse`、`resultEntry` 等类型，作为 Service 方法的返回值 —— Wails v3 自动生成 TypeScript 类型。

---

### Phase 4: 重写 main.go (预估 1h)

**Step 4.1** — 应用入口

```go
// main.go
package main

func main() {
    // 如果带 CLI 参数，走原有的命令行模式（完全不变）
    if len(os.Args) > 1 {
        runCLI()   // 搬移原 main() 的 switch 逻辑
        return
    }

    // 否则启动 Wails 桌面应用
    runApp()
}
```

**Step 4.2** — Wails Application 设置 (`app.go`)

```go
func runApp() {
    app := application.New(application.Options{
        Name:     "DNS-Switch",
        Services: []application.Service{
            application.NewService(service.NewDNSService(
                config.FileStore{},
            )),
        },
        Assets: application.AssetOptions{
            FS: assets,  // go:embed frontend (或开发模式用 vite)
        },
    })

    // 主窗口
    app.Window.NewWithOptions(application.WebviewWindowOptions{
        Title:  "DNS-Switch",
        Width:  700,
        Height: 600,
    })

    // 系统托盘（最小化到托盘）
    setupSystemTray(app)

    app.Run()
}
```

**Step 4.3** — CLI 模式保留

原有 `commands.go` 的 `setCmd`、`restoreCmd`、`testCmd` 完全不动。

---

### Phase 5: 清理旧代码 (预估 0.5h)

**Step 5.1** — 删除或重构

| 文件 | 操作 |
|------|------|
| `internal/server/server.go` | **删除** — 功能由 DNSService + Wails 替代 |
| `server.go` | **删除** — `runServer()` 不再需要 |
| `internal/server/` 目录 | **删除** |

**Step 5.2** — 保留不变的文件

```
✅ internal/bench/         — 零改动
✅ internal/config/        — 零改动
✅ internal/dns/           — 零改动 (Manager 被 Service 复用)
✅ internal/notify/        — 零改动 (后续对接 Wails 原生通知)
✅ platform/               — 零改动
✅ commands.go             — 零改动
```

---

### Phase 6: 构建配置 (预估 1h)

**Step 6.1** — `wails.json`

```json
{
  "name": "dns-switch",
  "frontend": {
    "dir": "./frontend",
    "build": "",
    "dev": ""
  }
}
```

因为前端是 vanilla HTML (无需 npm build)，`frontend.build` 和 `frontend.dev` 留空。

**Step 6.2** — AppImage 构建脚本

新建 `build/linux/build-appimage.sh`:

```bash
#!/bin/bash
# 1. 编译 Go 二进制 (CGO_ENABLED=0)
CGO_ENABLED=0 go build -ldflags="-s -w" -o build/appdir/dns-switch .

# 2. 收集 GTK4/WebKitGTK 依赖
linuxdeploy --appdir build/appdir \
  --plugin gtk \
  --output appimage

# 输出: dns-switch-x86_64.AppImage
```

**Step 6.3** — CI (可选)

GitHub Actions matrix build:
- `ubuntu-24.04` → AppImage
- `windows-latest` → NSIS installer
- `macos-latest` → .app bundle

---

### Phase 7: 测试验证 (预估 1h)

**Step 7.1** — 功能回归

- [ ] `wails3 dev` — 开发模式热重载
- [ ] DNS 测速 → 结果正确渲染
- [ ] DNS 切换 → 系统 DNS 实际改变
- [ ] DHCP 恢复 → 系统 DNS 恢复
- [ ] CLI 模式 — `dns-switch test/set/restore` 正常
- [ ] Server Build — `go build -tags server` 可启动纯 HTTP 服务
- [ ] 权限提示 — 非管理员/root 运行时提示正确

**Step 7.2** — 平台验证

- [ ] Windows 10/11 — WebView2 正常
- [ ] Ubuntu 24.04 — GTK4 + WebKitGTK 6.0 + AppImage
- [ ] macOS 14+ — .app bundle

---

## 五、文件变更总览

```
📁 dns-switch/
├── 🆕 app.go                          # Wails Application 初始化
├── ✏️ main.go                          # 改造：GUI + CLI 双模式入口
├── ❌ server.go                        # 删除
├── ✏️ commands.go                      # 不变 (或微调 main 合并)
├── ✏️ go.mod                           # 新增 wails v3 依赖
├── 🆕 wails.json                       # Wails v3 项目配置
├── 🆕 Taskfile.yml                     # 构建任务
│
├── 🆕 frontend/                        # 从 dashHTML 提取
│   ├── 🆕 index.html
│   ├── 🆕 style.css
│   ├── 🆕 main.js
│   └── 🆕 package.json
│
├── internal/
│   ├── ✅ bench/bench.go               # 不变
│   ├── ✅ config/config.go             # 不变
│   ├── ✅ config/store.go              # 不变
│   ├── ✅ config/filestore.go          # 不变
│   ├── ✅ dns/dns.go                   # 不变
│   ├── ❌ server/server.go             # 删除
│   ├── 🆕 service/dnsservice.go        # 新建：Wails Service
│   └── ✅ notify/notify.go             # 不变
│
├── ✅ platform/                        # 完全不变
│   ├── backend.go
│   ├── windows.go
│   └── linux.go
│
├── 🆕 build/
│   ├── linux/
│   │   └── 🆕 build-appimage.sh        # AppImage 打包脚本
│   ├── windows/                        # (wails3 init 自动生成)
│   └── darwin/                         # (wails3 init 自动生成)
│
├── ✅ config.example.toml              # 不变
├── ✅ scripts/                         # 保留，可能微调路径
└── ✅ README.md                        # 更新：新架构说明
```

**统计：**
- 新增文件：~8 个
- 修改文件：~3 个
- 删除文件：2 个 (`server.go`, `internal/server/server.go`)
- 不变文件：~12 个 (核心业务逻辑全部保留)

---

## 六、时间估算

| Phase | 内容 | 预估 |
|-------|------|------|
| 1 | 项目初始化 | 0.5h |
| 2 | 前端提取 | 0.5h |
| 3 | DNSService 实现 | 1h |
| 4 | main.go 改造 | 1h |
| 5 | 清理旧代码 | 0.5h |
| 6 | 构建/AppImage 配置 | 1h |
| 7 | 测试验证 | 1h |
| **总计** | | **5.5h** |

---

## 七、风险与注意事项

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Wails v3 API 不稳定 | Alpha 版本可能有 breaking change | 锁定具体 commit hash |
| Linux GTK4/WebKitGTK 兼容性 | 老发行版可能缺包 | 提供 Flatpak + AppImage 双方案 |
| Windows 注册表权限 | 与现在一致，无变化 | 无新增风险 |
| 管理员权限获取 | GUI 下无法 sudo | Wails v3 支持 `runas` / PolicyKit |
| Server Build 实验性 | `-tags server` 功能可能变更 | 保留 CLI 模式作为无头替代 |
| 前端 Wails Runtime API 变化 | JS 端调用方式可能调整 | 前后端都在同一仓库，同步更新 |

---

## 八、后续增强 (可选)

- [ ] 系统托盘图标 + 快速切换菜单
- [ ] 开机自启动 (Wails v3 Autostart)
- [ ] 原生桌面通知 (替换 notify 包)
- [ ] 多语言支持
- [ ] 自动更新 (Wails v3 In-App Updater)
