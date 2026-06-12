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

打开浏览器访问 [http://127.0.0.1:9753](http://127.0.0.1:9753)

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
dns-switch                        # 启动网页管理面板
dns-switch test                    # 测速全部 DNS
dns-switch set Cloudflare          # 切换到指定 DNS
dns-switch set Cloudflare 114DNS   # 设置主备 DNS
dns-switch restore                 # 恢复 DHCP
```

## 构建

```bash
# Linux / macOS
./scripts/build_linux.sh

# Windows (PowerShell)
.\scripts\build_windows.ps1
```

## 依赖

- Go 1.22+
- systemd-resolved / NetworkManager（Linux）
- Windows 10/11（Windows）

## 项目结构

```
dns-switch/
├── main.go                 入口
├── commands.go             CLI 命令
├── server.go               Web 面板入口
├── internal/
│   ├── bench/              DNS 测速核心
│   ├── config/             配置读写
│   ├── dns/                DNS 切换操作
│   ├── notify/             通知接口
│   └── server/             HTTP 管理面板
├── platform/               平台适配 (Linux/Windows)
├── scripts/                构建脚本
└── config.example.toml     配置示例
```
