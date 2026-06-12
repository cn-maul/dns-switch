# DNS-Switch

DNS 优选 / 一键切换工具。并发测速多个 DNS 服务器，通过 CLI 或 Web 面板一键切换系统 DNS。

## 快速开始

```bash
# 直接运行（自动编译）
./run.sh

# 或手动编译
go build -o dns-switch .
./dns-switch
```

无参数运行将启动桌面 GUI 窗口（基于 Wails v3）。

## 配置

配置文件路径（首次运行自动创建）：

| 平台 | 路径 |
|------|------|
| Linux | `~/.config/dns-switch/config.toml` |
| Windows | `%APPDATA%\dns-switch\config.toml` |
| macOS | `~/Library/Application Support/dns-switch/config.toml` |

参考 [`config.example.toml`](./config.example.toml) 编写配置。

## CLI 命令

```bash
dns-switch                        # 启动桌面管理面板
dns-switch test                    # 测速全部 DNS
dns-switch set Cloudflare          # 切换到指定 DNS
dns-switch set Cloudflare 114DNS   # 设置主备 DNS
dns-switch restore                 # 恢复 DHCP
```

## 构建

```bash
# 普通编译（CGO_ENABLED=0）
CGO_ENABLED=0 go build -ldflags="-s -w" -o dns-switch .

# Linux AppImage（自包含 GTK4/WebKitGTK）
./build/linux/build-appimage.sh
```

## 开发模式

```bash
# 需要安装 wails3 CLI
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
wails3 dev
```

## 依赖

- Go 1.22+
- systemd-resolved / NetworkManager（Linux）
- Windows 10/11（Windows）

## 项目结构

```
dns-switch/
├── main.go                 入口（CLI / GUI 双模式）
├── app.go                  Wails 桌面应用初始化
├── commands.go             CLI 命令（test / set / restore）
├── wails.json              Wails v3 项目配置
├── frontend/               桌面 GUI 前端 (vanilla HTML+CSS+JS)
│   ├── index.html
│   ├── style.css
│   └── main.js
├── internal/
│   ├── bench/              DNS 测速核心
│   ├── config/             配置读写
│   ├── dns/                DNS 切换操作
│   ├── service/            Wails Service 绑定
│   └── notify/             通知接口
├── platform/               平台适配 (Linux/Windows)
├── build/                  构建脚本
│   └── linux/
│       └── build-appimage.sh  AppImage 打包
├── scripts/                旧版构建脚本
└── config.example.toml     配置示例
```
