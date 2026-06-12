# Windows 构建与运行

## 前置要求

- [Go 1.26+](https://go.dev/dl/)
- PowerShell 5.0+（Windows 10/11 自带）

---

## 一键构建

用 PowerShell 执行构建脚本：

```powershell
.\build_windows.ps1
```

脚本会自动：

1. 检查 Go 版本
2. 安装 `rsrc` 工具（将 `.manifest` → `.syso`）
3. 生成 `rsrc.syso`（Go 构建时自动链接）
4. 执行 `go build`
5. 清理临时文件

构建产物为 `dns-switch.exe`。

---

## 运行

**必须以管理员身份运行**，否则 `set` / `restore` 会因权限不足报错。

### 方法 1：提权后启动（推荐）

打开「命令提示符」或「PowerShell」→ 右键 → **"以管理员身份运行"**，然后：

```powershell
.\dns-switch.exe                 # 启动网页管理面板 (http://127.0.0.1:9753)
.\dns-switch.exe test             # 测速所有 DNS
.\dns-switch.exe set Cloudflare   # 切换 DNS
.\dns-switch.exe restore          # 恢复 DHCP
```

### 方法 2：直接双击

因为嵌入了 UAC 清单（`dns-switch.exe.manifest`），Windows 会自动弹出 **用户账户控制 (UAC)** 提权对话框：

- 点击 **"是"** → 以管理员权限启动 → 打开网页管理面板
- 点击 **"否"** → 程序拒绝启动

> **注意**：UAC 提权后程序的工作目录可能是 `C:\Windows\System32`，但配置文件现在使用标准路径 `%APPDATA%/dns-switch/config.toml`，所以不受工作目录影响。

---

## 手动构建（不用脚本）

如果不想用 `rsrc` 工具，也可以手动完成：

```powershell
# 1. 生成 manifest 资源
rsrc -manifest dns-switch.exe.manifest -o rsrc.syso

# 2. 编译
go build -ldflags "-s -w" -o dns-switch.exe

# 3. 清理临时文件
del rsrc.syso
```

或者将 `rsrc.syso` 提交到版本控制，这样 `go build` 可以直接使用，无需 `rsrc`。

> **注意**：本项目当前将 `rsrc.syso` 加入 `.gitignore`（不追踪），因为它是编译产物。每次构建前需运行 `rsrc` 重新生成。如果希望简化流程，可移除 `.gitignore` 中的 `rsrc.syso` 并提交一次。

---

## 原理说明

### UAC 清单

`dns-switch.exe.manifest` 文件的内容：

```xml
<trustInfo xmlns="urn:schemas-microsoft-com:asm.v2">
  <security>
    <requestedPrivileges>
      <requestedExecutionLevel level="requireAdministrator" uiAccess="false" />
    </requestedPrivileges>
  </security>
</trustInfo>
```

- `level="requireAdministrator"` — 要求管理员权限，启动时弹 UAC 提权
- `uiAccess="false"` — 不需要 UI 辅助权限（安全）

### rsrc 工具

`rsrc`（`github.com/akavel/rsrc`）将 XML 清单编译为 Go 可链接的 `.syso` 文件。Go 编译时会自动搜索当前目录下的 `.syso` 文件并链接进 PE 二进制。

---

## 常见问题

**Q: 双击后闪退 / 没有窗口出现**  
A: 因为 `requireAdministrator` + 双击 → UAC 弹窗 → 通过后打开新窗口。如果闪退，检查 `%APPDATA%\dns-switch\config.toml` 是否存在或是否已配置 DNS 服务器。

**Q: 报 "rsrc 不是可运行的程序"**  
A: 确认 `$env:USERPROFILE\go\bin` 或 `$env:GOPATH\bin` 在 `PATH` 中。可以手动执行：`go install github.com/akavel/rsrc@latest`

**Q: 不需要管理员权限的其他命令（如 `/test`）也要提权？**  
A: 是的，清单是 PE 级别的，要么整个程序提权，要么不。`/test` 只做 DNS 查询其实不需要管理员权限，但考虑大部分操作（set/restore）都需要，统一提权更简单。如果介意，可以改为 `asInvoker` 并在运行时用 `ShellExecute` 提权（但复杂度高很多）。
