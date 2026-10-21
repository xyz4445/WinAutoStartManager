# 开发者文档

本文面向维护者，说明项目结构、关键流程、内部约定和测试方式。

- 使用者请看：[USAGE.md](USAGE.md)
- 构建与发布：[BUILD.md](BUILD.md)
- 故障排查：[TROUBLESHOOTING.md](TROUBLESHOOTING.md)
- 项目概览：[../README.md](../README.md)

- 平台：Windows 10/11 x64
- 语言：Go 1.27.3（以 `go.mod` 为准）
- 界面：WebView2 + 内嵌 HTML/CSS/JS（无前端构建步骤）
- 产物：单 exe，GUI 子系统，无安装程序

## 1. 设计目标

1. 登录后自动启动清单中的启用项：管理员项带管理员令牌，普通项保持普通权限，登录阶段零 UAC。
2. 只有开启/关闭“开机自启”时可能需要一次 UAC。
3. 成功静默；失败弹一条 Windows 通知并写日志。
4. 单 exe、无托盘、无后台守护。
5. 任务只保存 exe 路径，清单在登录时读取，因此修改清单不需要重新应用任务。

非目标：登录前启动、多用户共享配置、绕过 UAC、自动更新、多语言、托盘常驻。

## 2. 目录与包职责

| 路径 | 职责 |
| --- | --- |
| `main.go` | 解析内部参数；分发 GUI、`--launch`、`--apply-tasks` 三种模式 |
| `internal/appdata` | 数据目录解析（默认目录 + HKCU 覆盖）、校验、迁移 |
| `internal/apply` | 提权 helper 的结果 JSON 读写 |
| `internal/config` | `config.json` 模型、读写、`%VAR%` 展开、保存校验 |
| `internal/elevate` | `ShellExecuteExW("runas")` 提权启动与等待 |
| `internal/gui` | WebView2 宿主、Go↔JS 绑定、Bootstrap 视图、原生对话框、关闭保护、DPI |
| `internal/gui/assets` | `index.html`、`app.css`、`app.js`，通过 `go:embed` 打包 |
| `internal/launcher` | 按权限层生成计划、计算延迟、启动进程、运行结果聚合 |
| `internal/logging` | 按天/按层写日志、2 MB 轮转、按保留天数清理 |
| `internal/notify` | `Shell_NotifyIconW` 气泡通知 |
| `internal/schedule` | 计划任务 XML、创建/删除、状态检查、输出解码、外部命令执行 |
| `internal/settings` | `settings.json`（日志保留天数） |
| `internal/state` | 每层运行结果 `state-<tier>.json`，原子写与并发重试 |

## 3. 运行模式

| 参数 | 入口 | 说明 |
| --- | --- | --- |
| 无参数 | `gui.Run()` | 打开管理器 |
| `--launch --tier=admin\|user` | `runLaunch()` | 由计划任务调用，执行一层启动后退出 |
| `--apply-tasks <result.json>` | `runApply()` | 提权 helper，创建两个任务并写结果 |
| `--apply-tasks <result.json> --remove-tasks` | `runApply()` | 提权 helper，删除两个任务 |

GUI 与 helper 是同一个 exe。helper 只做任务注册/删除，不打开窗口。

## 4. 权限模型与计划任务

### 4.1 两个任务

| 任务名 | RunLevel | 动作 |
| --- | --- | --- |
| `AutoStartManager-Admin` | `HighestAvailable` | `<exe> --launch --tier=admin` |
| `AutoStartManager-User` | `LeastPrivilege` | `<exe> --launch --tier=user` |

任务 XML 关键字段（`internal/schedule/schedule.go`）：

- `LogonTrigger`：`UserId = 当前用户`，`Delay = PT2S`，`Enabled = true`
- `Principal`：`LogonType = InteractiveToken`，`RunLevel = HighestAvailable / LeastPrivilege`
- `Settings`：`Hidden = true`、`MultipleInstancesPolicy = IgnoreNew`、`ExecutionTimeLimit = PT5M`、
  `DisallowStartIfOnBatteries = false`、`StopIfGoingOnBatteries = false`
- `Actions`：`Context = Author`，`Command = exe 路径`，`Arguments = --launch --tier=<tier>`，`WorkingDirectory = exe 所在目录`

XML 由 `xml.MarshalIndent` 生成，声明为 `encoding="UTF-16"`，写入临时文件时用 `UTF16LE()` 编码（带 BOM），再执行：

```
schtasks /Create /TN <name> /XML <temp> /F
```

临时文件写在数据目录（不可写时退回系统临时目录），创建后立即删除。

### 4.2 为什么要两个任务

进程只能继承父进程的完整性级别：提权进程无法在不弹 UAC 的情况下派生出普通权限子进程，反之亦然。因此按权限分层，每层一个登录任务。

### 4.3 开启/关闭流程

```
GUI setAutoStart(enabled)
  → ShellExecuteExW("runas", 自身 exe, "--apply-tasks <tmp.json> [--remove-tasks]")
  → helper: schedule.Apply / schedule.Remove
  → 结果写入 tmp.json { "ok": true } 或 { "ok": false, "error": "..." }
  → GUI 读取结果，重新 loadApp 刷新任务状态
```

- `runas` 在 UAC 开启时会弹一次确认；`EnableLUA=0` 时不会弹，这是系统行为，不是故障。
- helper 以 GUI 子系统运行，自身没有控制台窗口。
- 删除是幂等的：任务不存在时视为成功。

## 5. 任务状态检查（`internal/schedule/install.go`）

### 5.1 状态定义

```go
StatusMissing  = "missing"   // 确认不存在
StatusStale    = "stale"     // 存在，但 Command 指向别的 exe
StatusOK       = "ok"        // 存在且指向当前 exe
StatusUnknown  = "unknown"   // 查询或解析失败，无法确认
```

`StatusUnknown` 是关键：**查询失败绝不能被当成“未开启”**。

### 5.2 单层检查流程

```
Check(exePath, tier, runner):
  out, err = schtasks /Query /TN <name> /XML
  if err != nil:
      listed, listErr = schtasks /Query /FO CSV /NH
      if listErr != nil:        return unknown, error
      if !listed:               return missing, nil
      else:                     return unknown, error
  text = decodeOutput(out)
  parse text as XML; on error:  return unknown, error
  cmd = trim(<Actions><Exec><Command>)
  if cmd == "":                 return unknown, error
  if EqualFold(Clean(cmd), Clean(exePath)): return ok
  return stale
```

- 存在性回退用 `schtasks /Query /FO CSV /NH`，只匹配 ASCII 任务名（`"AutoStartManager-Admin"`），不受本地化编码影响。
- `Check` 对 `unknown` 额外重试一次，间隔 `checkRetryDelay`（默认 150ms，测试置 0）。
- 两层状态合并（`internal/gui/api.go`）：`missing > unknown > stale > ok`。任一层明确缺失就是 `missing`；否则任一层无法确认就是 `unknown`。

### 5.3 前端表现

`app.js` 的 `renderStatus()`：

- `ok` / `stale` / `missing` 才改写开关状态；
- `unknown` **不改写开关**，显示“状态未知 · 无法读取计划任务，开关保持原状”；
- `stale` 显示“需更新 · 任务指向旧路径”；
- `missing` 显示“未开启 · 登录时不会启动”。

## 6. 登录启动流程（`main.go` + `internal/launcher`）

1. 任务触发 `--launch --tier=<tier>`；进程已由 Task Scheduler 赋予对应令牌。
2. 打开本层日志（`logs/launcher-YYYYMMDD-<tier>.log`）。
3. 读取 `settings.json` 的日志保留天数。
4. 读取并校验 `config.json`；缺失按空清单处理，损坏则通知并退出。
5. 统计启用但无路径的草稿，记录跳过数量。
6. `schedule.Check` 另一层的任务状态，作为 `other` 传入 `RunTier`。
7. `BuildPlan` 过滤：`enabled && tier 匹配 && Runnable()`，保持数组顺序。
8. `Execute` 按**相对登录的绝对时刻**启动：第 i 项的 deadline = 进程启动时刻 + `delaySeconds`，已过期的立即启动，不累加。每项用 `CreateProcessW(..., CREATE_NO_WINDOW)` 启动，继承当前令牌。
9. 写 `state-<tier>.json`（临时文件 + 原子重命名 + 冲突重试）。
10. 有失败则发一条本层汇总通知并写日志；全部成功则完全静默。
11. 按保留天数清理旧日志。

`RunTier` 在另一层状态不是 `ok` 时会追加一条失败结果（`__other_tier__`），用于提示用户检查另一层的计划任务。这是保守策略：宁可提示，也不静默漏启动。

## 7. GUI 架构

- `gui.Run()` 在创建窗口前调用 `enablePerMonitorDPI()`，随后创建 WebView2 窗口。
- 数据目录中的 `webview2/` 作为 WebView2 缓存路径。
- `w.Bind(name, fn)` 注册 Go 函数，前端通过 `window.<name>(...)` 调用。
- `SetHtml(assetsHTML())` 注入内嵌页面；JS/CSS 通过占位符替换后拼成单页。

绑定列表：

| 名称 | 作用 |
| --- | --- |
| `loadApp` | 读取配置、设置、运行状态、任务状态，返回 Bootstrap |
| `saveItems` | 保存清单（只做结构校验，允许草稿） |
| `setAutoStart` | 提权 helper 创建/删除任务 |
| `saveSettings` | 保存数据目录与日志保留天数，必要时迁移并重启 |
| `restartApp` | 启动新实例并退出旧实例 |
| `runItem` | 测试运行单个条目（管理员项走 `runas`，会弹 UAC） |
| `setDirty` | 同步“有未保存修改”给 Go，供关闭保护使用 |
| `pickExe` / `pickDir` | 原生文件/目录对话框 |
| `openLogs` / `openDataDir` | 打开日志/数据目录 |

### 7.1 保存与关闭

- 编辑只改内存，`state.rev` 递增；点击“保存清单”或 Ctrl+S 才写盘。
- 保存期间继续编辑时，用保存开始时的 `rev` 判断是否仍有未保存修改，避免错误清除 dirty。
- `installCloseGuard` 子类化顶层窗口，拦截 `WM_CLOSE`：dirty 时弹原生“是/否”，默认“否”；选“是”直接放行，不写盘，下次启动恢复上次保存的清单。
- `atomic.Bool` 防止消息框的模态循环导致重复弹窗。

### 7.2 高 DPI

- 创建窗口前调用 `SetProcessDpiAwarenessContext(PER_MONITOR_AWARE_V2)`；不可用时退回 `SetProcessDpiAwareness(2)`。
- 初始窗口 1000×740 逻辑像素，按 `dpi/96` 换算，限制在工作区 90% 内；最小 860×560 同步换算。
- 子类化窗口过程处理 `WM_DPICHANGED`：先更新最小尺寸，再 `SetWindowPos` 套用 Windows 建议矩形，避免从高缩放屏幕移回低缩放屏幕时无法缩小。

### 7.3 等待动画

- 启动：半透明磨砂加载页（62% 遮罩 + 10px backdrop blur + 中心玻璃卡片），至少 500ms；成功/失败都淡出。
- 保存/测试运行/设置保存：按钮内嵌 spinner，至少 320–350ms。
- 切换开机自启：开关旁 spinner 与高亮，覆盖 helper 执行与状态刷新，至少 500ms。
- `prefers-reduced-motion` 下禁用大部分动画。

## 8. 数据格式

### 8.1 数据目录

- 默认：`%LOCALAPPDATA%\AutoStartManager`
- 覆盖：`HKCU\Software\AutoStartManager\DataDir`（`REG_SZ`）
- 恢复默认时删除该值。
- 迁移复制 `config.json`、`settings.json`、`state-admin.json`、`state-user.json`、`logs/*`；不复制 WebView2 缓存。

### 8.2 `config.json`

```json
{
  "version": 1,
  "items": [
    {
      "id": "a1b2c3d4e5f60718",
      "name": "AutoHotkey",
      "path": "C:\\Program Files\\AutoHotkey\\v2\\AutoHotkey64.exe",
      "args": "",
      "workDir": "",
      "elevated": true,
      "delaySeconds": 0,
      "enabled": true
    }
  ]
}
```

- 数组顺序即启动顺序。
- `path` 支持 `%VAR%`，保存保留原文，启动时展开；未知变量原样保留。
- `id` 为随机十六进制字符串，仅用于界面关联。
- `ValidateForSave` 只校验延迟范围、ID 非空、ID 不重复；名称/路径为空是合法草稿。
- `Runnable()` 以 `path` 非空为准。

### 8.3 `settings.json`

```json
{
  "logRetentionDays": 14
}
```

范围 1–365，缺失或为 0 时取默认 14，越界自动钳制。

### 8.4 `state-<tier>.json`

```json
{
  "tier": "admin",
  "startedAt": "2026-10-21T08:31:00+08:00",
  "finishedAt": "2026-10-21T08:31:04+08:00",
  "results": [
    {
      "id": "a1b2c3d4e5f60718",
      "name": "AutoHotkey",
      "ok": true,
      "pid": 1234,
      "startedAt": "2026-10-21T08:31:02+08:00"
    }
  ]
}
```

- 每层独立文件，原子替换；Windows 上重命名冲突时最多重试 20 次（每次 10ms）。
- 界面按条目 ID 取 `StartedAt` 最新的一条结果。

### 8.5 helper 结果文件

```json
{ "ok": true }
```

失败时：

```json
{ "ok": false, "error": "注册任务 AutoStartManager-Admin 失败: ..." }
```

文件由 `setAutoStart` 创建临时路径，helper 写入，父进程读取后删除。

### 8.6 日志

- 路径：`<dataDir>\logs\launcher-YYYYMMDD-<tier>.log`
- 追加写，单文件 ≥ 2 MB 时轮转为 `.1`（旧的 `.1` 会被覆盖）。
- 行格式：`2026-10-21 08:31:02 [admin] START ok pid=1234 name=AutoHotkey`
- 保留天数来自 `settings.json`，启动时清理超期文件。

## 9. 输出编码与外部命令

### 9.1 观察到的 `schtasks` 输出形态

同一份代码在不同系统上看到过三种形态：

| 系统 | `/Query /XML` 输出 |
| --- | --- |
| 部分系统重定向到文件 | UTF-16LE + BOM |
| 英文系统管道 | UTF-8 无 BOM（XML 声明仍写 `UTF-16`） |
| 中文系统管道 | 系统 ANSI 代码页（CP936/GBK） |

任务 XML 的 `<Description>` 包含中文（“开机启动管理器：admin 层”）。按 UTF-8 读取 GBK 字节会得到 `invalid UTF-8`，这是 2026-10-21 修复的“开关回弹为未开启”的根因。

### 9.2 `decodeOutput` 的识别顺序

1. `FF FE` BOM → UTF-16LE 解码。
2. 无 BOM，但奇数位大量为 0 → 按 UTF-16LE 解码（`looksUTF16LE`）。
3. 去掉 UTF-8 BOM（`EF BB BF`）。
4. `utf8.Valid` → 原样作为 UTF-8。
5. 否则调用 Windows `MultiByteToWideChar(CP_ACP, 0, ...)`，按系统 ANSI 代码页解码。
6. 仍失败则退回原始字节字符串。

`CP_ACP = 0`。实现位于 `encoding_windows.go`；非 Windows 构建使用 Latin-1 一对一映射，只是为了保持包可编译。

### 9.3 XML 声明

`parseQueryXML` 给 `xml.Decoder` 设置 `CharsetReader`，对任何声明都返回原始 reader。这样即使字符串已经转成 UTF-8、声明仍写 `encoding="UTF-16"`，也能正常解析。

### 9.4 stdout / stderr 分离

`ExecRunner` 不再使用 `CombinedOutput`：

- 成功时只返回 stdout，避免 stderr 上的警告污染 XML；
- 失败时返回 stdout+stderr 的拼接，并把解码后的文本放进错误消息。

### 9.5 状态检查的失败语义

- 查询失败：先查任务列表。列表确认不存在 → `missing`；列表里存在但 XML 读不出来 → `unknown`。
- 解析失败、空命令 → `unknown`。
- `unknown` 重试一次；仍失败则保留错误，交给上层合并。
- 这条规则保证“一次查询失败不会把开关打回未开启”。

## 10. 控制台窗口抑制

GUI 子系统进程自身没有控制台。它启动 `schtasks` 这类控制台程序时，如果没有 `CREATE_NO_WINDOW`，Windows 会为子进程新建一个控制台窗口，表现为 cmd 黑框一闪。

`internal/schedule/exec_windows.go`：

```go
cmd.SysProcAttr = &syscall.SysProcAttr{
    HideWindow:    true,
    CreationFlags: windows.CREATE_NO_WINDOW,
}
```

`ExecRunner` 统一通过 `newCommand` 创建命令，因此：

- 打开管理器时的两次状态检查；
- 登录启动器的跨层检查；
- 切换启停时 helper 里的创建/删除；

都不会再闪控制台。非 Windows 构建使用普通 `exec.Command`。

启动清单中的程序由 `launcher.StartProcess` 用 `CreateProcessW(..., CREATE_NO_WINDOW)` 启动，同样不产生控制台窗口。

## 11. 测试

### 11.1 命令

```powershell
go test ./... -count=1
go vet ./...
gofmt -l .\internal .\main.go

# 交叉编译代码检查（非 Windows 主机）
GOOS=windows GOARCH=amd64 go build ./...
GOOS=windows GOARCH=amd64 go vet ./...

# 完整构建见 BUILD.md
./build.sh
```

> 在 Linux 上直接 `go build ./...` 会报 `build constraints exclude all Go files`，这是因为 Windows 专属文件被构建约束排除，属正常现象，需带 `GOOS=windows`。

### 11.2 覆盖范围

| 包 | 主要测试 |
| --- | --- |
| `config` | 读写往返、坏 JSON、缺省值、`%VAR%` 展开、草稿校验 |
| `logging` | 文件名、2 MB 轮转、按天清理 |
| `state` | 原子写、并发不覆盖、缺失文件、损坏 JSON |
| `schedule` | XML 生成、UTF-16、中文路径、状态判定、删除幂等 |
| `launcher` | 分层过滤、绝对延迟、失败聚合、跨层检查 |
| `apply` | helper 结果读写 |
| `appdata` | 默认目录、注册表覆盖、迁移规则 |
| `settings` | 默认值、往返、钳制、损坏 |
| `elevate` | `SHELLEXECUTEINFOW` 布局 |
| `gui` | Bootstrap 合并、草稿保存、状态合并、对话框布局、DPI |

### 11.3 状态检查回归用例

`internal/schedule/install_test.go`、`exec_windows_test.go`：

- `TestCheckHandlesGBKOutput`：GBK 中文描述必须解析成功（本次根因）。
- `TestCheckHandlesUTF8WithoutBOM` / `TestCheckHandlesUTF8WithBOM`。
- `TestCheckHandlesUTF16WithoutBOM`。
- `TestCheckQueryFailureIsUnknownWhenTaskExists`：任务存在但读不出来 → `unknown`。
- `TestCheckParseFailureIsUnknown`、`TestCheckEmptyCommandIsUnknown`。
- `TestCheckRetriesTransientFailure`：第一次失败、第二次成功 → `ok`。
- `TestExecRunnerKeepsStderrOutOfSuccessfulStdout`。
- `TestNewCommandHidesConsoleWindow`：`HideWindow` + `CREATE_NO_WINDOW`。
- `TestCheckMissing`：XML 失败且任务列表无此名称 → `missing`。
- `TestRunTier...`：跨层状态聚合。
- `TestCombineTaskStatus`：`missing > unknown > stale > ok`。

### 11.4 手工验证（中文 Windows）

1. 安装任务，确认 `Get-ScheduledTask` 中 Admin=Highest、User=Limited。
2. 重启管理器，开关应显示“已开启”，且启动过程不闪 cmd。
3. 关闭再开启，过程中不闪 cmd；UAC 关闭的机器不应出现提示。
4. 删除一个任务，重开管理器应显示“未开启”或“需更新”，而不是“状态未知”。
5. 模拟查询失败（见 12.1）时，开关不应被打回关闭。

## 12. 故障排查手册

### 12.1 抓取 `schtasks` 的原始输出（不经过 PowerShell 解码）

在目标机器上运行，观察 `utf8_has_desc` / `gbk_has_desc` 与 `<Description>` 后的字节：

```powershell
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = 'schtasks'
$psi.Arguments = '/Query /TN AutoStartManager-Admin /XML'
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$p = [System.Diagnostics.Process]::Start($psi)
$ms = New-Object System.IO.MemoryStream
$p.StandardOutput.BaseStream.CopyTo($ms)
$p.WaitForExit()
$b = $ms.ToArray()
"ACP=" + (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Nls\CodePage').ACP
"locale=" + (Get-WinSystemLocale).Name
"len=$($b.Length) exit=$($p.ExitCode)"
$u = [Text.Encoding]::UTF8.GetString($b)
$g = [Text.Encoding]::GetEncoding(936).GetString($b)
"utf8_has_desc=" + $u.Contains('开机启动管理器')
"gbk_has_desc=" + $g.Contains('开机启动管理器')
$i = [Text.Encoding]::ASCII.GetString($b).IndexOf('<Description>')
"descIndex=$i"
if ($i -ge 0) { $end=[Math]::Min($i+70, $b.Length-1); (($b[$i..$end] | % { $_.ToString('X2') }) -join ' ') }
```

判读：

- `ACP=936`、`utf8_has_desc=False`、`gbk_has_desc=True`，且 `<Description>` 后出现 `BF AA BB FA` 一类字节 → 输出是 GBK。
- 当前 `decodeOutput` 会按 `CP_ACP` 解码，不应再失败。
- 若仍失败，检查 `decodeANSI` 是否被正确编译进 Windows 构建。

### 12.2 确认任务本身

```powershell
Get-ScheduledTask -TaskName AutoStartManager-Admin, AutoStartManager-User |
  Select-Object TaskName, State, @{n='RunLevel';e={$_.Principal.RunLevel}}, @{n='User';e={$_.Principal.UserId}}

(Get-ScheduledTask -TaskName AutoStartManager-Admin).Actions |
  Select-Object Execute, Arguments, WorkingDirectory

whoami /groups | Select-String 'Mandatory Label'
```

- `EnableLUA=0` 时进程为 High，`runas` 不再弹 UAC，这是预期行为。
- 任务存在但 XML 查询失败时，当前版本显示“状态未知”，不会显示“未开启”。

### 12.3 确认 exe 版本

```powershell
Get-FileHash .\WinAutoStartManager.exe -Algorithm SHA256
```

与构建记录对照。确定性构建（`-buildvcs=false`）的当前值为：

```
c39ee3217f7ae580bdff8d1658d42c393d46f086a302b024c8c8960ba8ecda14
```

不加 `-buildvcs=false` 时 hash 会随 commit 变化，详见 [BUILD.md](BUILD.md#4-为什么必须-buildvcsfalse)。

### 12.4 确认子系统

```powershell
$b = [IO.File]::ReadAllBytes('.\WinAutoStartManager.exe')
$pe = [BitConverter]::ToInt32($b, 0x3C)
'subsystem=' + [BitConverter]::ToUInt16($b, $pe + 0x5C)
```

- `2` = GUI（无控制台）
- `3` = 控制台

## 13. 构建与发布

完整的构建参数说明、可复现构建原理、图标生成和发布检查清单见 [BUILD.md](BUILD.md)。要点：

- 一键构建：`./build.sh`（可在 Linux / macOS / WSL 上交叉编译）
- `-buildvcs=false` 是确定性构建的必要条件
- `rsrc.syso` 已入库，clone 后无需重新生成图标资源
- 依赖：`github.com/jchv/go-webview2`、`github.com/jchv/go-winloader`、`golang.org/x/sys`；不需要 C 编译器

## 14. 不变量与设计决策

1. 管理器 GUI 不注册自启；登录时只运行 `--launch`。
2. 计划任务只保存 exe 路径；清单运行时读取。
3. 两层任务各自独立，互不连带提权。
4. 每层状态文件独立，原子写，互不覆盖。
5. 失败必须留下痕迹：日志 + 通知。
6. 任务状态：`missing` 只在“确认不存在”时使用；查询/解析失败是 `unknown`。
7. 用户可见的开关只在 `ok/stale/missing` 时改写；`unknown` 保持原状。
8. 所有内部 `schtasks` 调用使用 `CREATE_NO_WINDOW`，成功输出与 stderr 分离。
9. 清单只在用户明确保存时写盘；关闭时确认，确认退出即放弃修改。
10. 延迟是相对登录的绝对时刻，不累加。
11. 草稿可以保存，但不参与启动。
12. 数据目录位置写入 HKCU，保证登录进程与 GUI 读取同一份数据。

## 15. 已知限制与后续可选项

- 未做代码签名。
- 登录前启动不支持。
- 标准用户无法创建最高权限任务。
- 自定义数据目录位于可移动磁盘时，登录时不可用会导致启动失败。
- 登录时另一层状态不是 `ok`（含 `unknown`）会按失败聚合，发出一条提示；管理器中的状态更精确。
- 未做任务状态定时轮询；状态在打开管理器、切换开关、保存设置后刷新。

## 16. 变更记录

- **第 1–5 轮**：基础功能、草稿、显式保存与关闭确认、图标、等待动画、半透明加载页、Per-Monitor V2 DPI。
- **第 6 轮（2026-10-21）**：修复中文 Windows 上状态误判。
  - 根因：`schtasks` 管道输出为 GBK，中文 `<Description>` 导致 `invalid UTF-8`，旧代码把解析失败当成 `missing`。
  - 修复：`decodeOutput` 支持 UTF-16LE（含/不含 BOM）、UTF-8（含 BOM）、系统 ANSI 代码页；新增 `StatusUnknown`、任务列表回退、失败重试、stderr 分离；前端不再因 `unknown` 改写开关。
- **第 7 轮（2026-10-21）**：消除 cmd 闪窗。
  - `ExecRunner` 改用 `newCommand`，设置 `HideWindow + CREATE_NO_WINDOW`。
  - 覆盖管理器启动、登录启动器、切换启停三条路径。
- 本轮验证：`go test ./... -count=1` 全绿；`go vet ./...` 无输出；`gofmt` 干净；GUI 冒烟启动正常。
