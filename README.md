# 开机启动管理器（AutoStart Manager）

一个单文件 Windows 小工具：用图形界面维护启动清单，在**用户登录时**按“管理员 / 普通”两个权限层静默启动指定程序。登录阶段**不弹 UAC**，只有开启或关闭“开机自启”时可能需要确认一次。

主界面使用 WebView2 渲染内嵌的 HTML/CSS/JS，交互与原生程序一致；文件选择和关闭确认使用系统原生对话框。

## 功能特性

- **双权限层**：两个计划任务分别以最高权限（HighestAvailable）与普通权限（LeastPrivilege）运行，管理员项真正提权，普通项不会被连带提权。
- **登录触发**：登录后延迟 2 秒启动；任务里只保存 exe 路径，清单在运行时读取，因此增删改条目不需要重新应用任务。
- **完整条目设置**：名称、路径、启动参数、工作目录、延迟秒数、权限层、是否启用。
- **延迟语义**：从“用户登录”开始计算绝对时刻，不是在上一个程序之后累加。
- **失败可见**：启动失败弹出一条 Windows 通知并写日志；成功时完全静默。
- **状态可靠**：开关状态与真实计划任务一致；查询或解析失败时显示“状态未知”，不会误报为“未开启”。
- **无控制台闪窗**：打开管理器、登录启动、切换启停时都不会闪出 cmd 黑框。
- **无需常驻托盘**：管理器 GUI 本身不注册自启，只有 `--launch` 模式在登录时短暂运行。
- **草稿条目**：未填完的条目可以保存、编辑和删除，只是不会参与启动。
- **显式保存**：修改不会自动写入；点击“保存清单”或按 Ctrl+S 才落盘。有未保存修改时关闭窗口会先确认，确认退出则放弃这些修改。
- **高 DPI**：按 Per-Monitor V2 感知运行，窗口尺寸按显示缩放换算；在 125% / 150% / 200% 下文字和图标按原生分辨率渲染，跨不同缩放比例的显示器时自动调整。
- **等待反馈**：启动时显示半透明磨砂加载页；保存、测试运行、切换开机自启时显示旋转进度；切换数据目录时显示“迁移 / 重启”遮罩。系统开启“减少动态效果”时降级为静态提示。
- **可配置数据目录**：默认 `%LOCALAPPDATA%\AutoStartManager`，可迁移到任意文件夹；日志保留天数可调。

## 环境要求

- Windows 10/11（x64）
- Go 1.27.3 或更新版本（以 `go.mod` 为准）
- Microsoft Edge WebView2 运行时（Windows 11 自带；Windows 10 如缺失请安装 Evergreen Runtime）

## 构建

```powershell
go env -w GOPROXY=https://goproxy.cn,direct
go build -trimpath -ldflags "-s -w -H windowsgui" -o autostart-manager.exe .
```

`-H windowsgui` 让程序启动时不带控制台窗口，生成的是 GUI 子系统 exe。

图标由 `resources/make-icon.ps1` 生成 `resources/app.ico`，再通过 `rsrc.syso` 嵌入 exe（同时用于窗口和任务栏）。需要改图标时：

```powershell
powershell -ExecutionPolicy Bypass -File resources\make-icon.ps1
go install github.com/akavel/rsrc@latest
& "$(go env GOPATH)\bin\rsrc.exe" -ico resources\app.ico -o rsrc.syso
go build -trimpath -ldflags "-s -w -H windowsgui" -o autostart-manager.exe .
```

开发、测试与内部实现的详细说明见 [dev.md](dev.md)。

## 使用

1. 双击 `autostart-manager.exe`。
2. 点击“新建条目”，选择程序（例如 `AutoHotkey.exe`、`rclone.exe`），按需填写参数和延迟。
3. 需要以管理员身份运行的程序，把权限层切到“管理员”；其余保持“普通”。
4. 点击“保存清单”写入修改（快捷键 Ctrl+S）。编辑后不会自动保存；有未保存修改时关闭窗口会先确认。
5. 打开右上角的“开机自启”开关。在启用了 UAC 的系统上会提示一次；UAC 已关闭的系统不会提示。
6. 下次登录 Windows 时，清单中的程序会自动启动。

修改清单后不需要再次操作开关：计划任务只保存 exe 路径，清单在登录运行时读取。

### “开机自启”状态说明

| 状态 | 含义 | 操作 |
| --- | --- | --- |
| 已开启 · 登录时按清单启动 | 两个计划任务都已安装且指向当前 exe | 关闭即移除任务 |
| 未开启 · 登录时不会启动 | 还没有安装，或任务被删除 | 开启即安装 |
| 需更新 · 任务指向旧路径 | exe 被移动/重命名 | 开启即重新指向当前 exe |
| 状态未知 · 无法读取计划任务 | 任务查询失败，无法确认真实状态 | 开关保持原状；稍后重开程序或检查 `schtasks` |

“状态未知”不会把开关打回“未开启”，避免查询失败造成误判。

## 设置

点击右下角“设置…”：

- **数据目录**：默认 `%LOCALAPPDATA%\AutoStartManager`，可以改到其他文件夹。更改后程序会自动重启；勾选“迁移”时会把 `config.json`、`settings.json`、运行状态和日志复制到新目录。旧目录中的文件会保留，可手动删除。WebView2 缓存不迁移，会在新目录自动重建。
- **日志保留**：默认 14 天，可设为 1–365 天。程序启动时会删除超过该天数的日志。

数据目录的位置记录在 `HKCU\Software\AutoStartManager\DataDir`，这样登录时启动的进程也能找到同一份清单。恢复默认目录时会自动清除该记录。

## 数据与日志

默认位于 `%LOCALAPPDATA%\AutoStartManager\`，可在设置中更改：

- `config.json`：启动清单。
- `settings.json`：日志保留天数等设置。
- `state-admin.json` / `state-user.json`：最近一次各权限层的运行结果。
- `logs\launcher-<日期>-<层>.log`：按天、按层记录，单文件超过 2 MB 轮转为 `.1`，默认保留 14 天。
- `webview2\`：WebView2 缓存，可安全删除。

## 工作方式（简要）

- 计划任务名称：`AutoStartManager-Admin`、`AutoStartManager-User`；登录触发器延迟 2 秒，任务隐藏。
- 任务动作分别为 `autostart-manager.exe --launch --tier=admin` 与 `--launch --tier=user`。
- 启动器会交叉检查另一权限层的任务：任务缺失或指向旧路径时，本次运行会计为失败并通知。
- 每个权限层独立记录状态文件，两个任务并发运行也不会互相覆盖。
- 关闭“开机自启”会删除两个任务；任务本来就不存在时按成功处理（幂等）。
- 查询计划任务时自动识别 UTF-16LE、UTF-8 和系统 ANSI 代码页（中文系统为 GBK）输出；解析失败只报“状态未知”，不会当成任务不存在。
- 调用 `schtasks` 时使用 `CREATE_NO_WINDOW`，GUI 进程和登录启动器都不会弹出控制台窗口。
- 等待动画使用内嵌 CSS/JS，无外部依赖。启动加载页至少显示约 500ms；保存、测试运行和设置保存至少约 320–350ms；切换开机自启至少约 500ms。
- 保存清单时用修订号检测保存期间的新编辑：若保存请求发出后又有改动，不会错误清除“有未保存的更改”状态。
- 启动时在创建窗口前启用 Per-Monitor V2 DPI 感知；初始窗口为 1000×740 逻辑像素，限制在工作区 90% 以内；收到 `WM_DPICHANGED` 时先更新最小尺寸，再套用 Windows 建议的窗口矩形。

## 命令行（内部使用）

| 参数 | 说明 |
| --- | --- |
| 无参数 | 打开图形界面 |
| `--launch --tier=admin\|user` | 由计划任务调用，执行对应层的启动 |
| `--apply-tasks <结果文件>` | 由提权的自身进程调用，写入两个计划任务并把结果写入 JSON |
| `--apply-tasks <结果文件> --remove-tasks` | 由提权的自身进程调用，删除两个计划任务 |

## 故障排查

### 开关显示“未开启”，但计划任务确实存在

先确认任务和路径：

```powershell
schtasks /Query /TN AutoStartManager-Admin /XML
schtasks /Query /TN AutoStartManager-User /XML
```

当前版本已修复中文 Windows 上因 GBK 输出导致的误判。若仍出现，请确认运行的是最新 exe，并参考 [dev.md 的排查章节](dev.md#12-故障排查手册) 抓取 `schtasks` 的原始输出。

### 开关显示“状态未知”

说明任务查询或 XML 解析失败，程序无法确认真实状态，因此没有改动开关。常见原因：

- `schtasks` 被安全软件拦截或不可用；
- 任务存在但当前用户没有读取权限；
- 任务 XML 损坏。

可以手动运行上面的 `/Query` 命令确认，或重新打开程序重试。

### 打开程序或切换开关时闪出 cmd 黑框

当前构建已通过 `CREATE_NO_WINDOW` 消除该现象。若仍出现，请核对 exe 的 SHA256 是否为最新构建，或是否有其他程序在调用 `schtasks`。

### 首次运行被 SmartScreen 拦截

程序未做代码签名，选择“更多信息 → 仍要运行”。

### 提示无法创建窗口

安装 Microsoft Edge WebView2 Evergreen 运行时后重试。

## 卸载

先在界面中关闭“开机自启”，然后删除程序和数据：

```powershell
Remove-Item -Recurse "$env:LOCALAPPDATA\AutoStartManager"
Remove-ItemProperty "HKCU:\Software\AutoStartManager" -Name DataDir -ErrorAction SilentlyContinue
```

如果改过数据目录，请同时删除自定义目录。也可以手动删除计划任务：

```powershell
schtasks /Delete /TN AutoStartManager-Admin /F
schtasks /Delete /TN AutoStartManager-User /F
```

## 已知限制

- 未做代码签名，首次运行可能被 SmartScreen 拦截。
- 移动或重命名 exe 后，开关会显示“需更新”，重新开启一次即可。
- 登录触发发生在用户会话建立之后；登录前（如系统服务阶段）启动不在支持范围内。
- 自定义数据目录如果放在可移动磁盘上，登录时磁盘不可用会导致本次启动失败并通知。
- 标准用户无法创建最高权限任务，管理员层需要当前账户属于 Administrators 组。

## 更新记录

### 2026-10-21 · 状态回读与静默启动修复

- **修复中文 Windows 上开关回弹为“未开启”**：`schtasks /Query /XML` 在中文系统上按 GBK 输出，任务描述中的中文导致 XML 解析失败；旧逻辑把解析失败当成任务不存在。现在按系统 ANSI 代码页解码，并区分 `missing / stale / ok / unknown`。
- **修复打开管理器、登录启动、切换启停时闪 cmd 黑框**：所有 `schtasks` 调用改用 `CREATE_NO_WINDOW`，成功输出与 stderr 分离，避免警告污染 XML。
- 新增状态“状态未知”：查询失败时保持开关原状，不再误报关闭。
- 查询失败自动重试一次；任务缺失判定改为“XML 读取失败 + 任务列表确认不存在”。
- 回归测试覆盖 GBK、UTF-8（含/不含 BOM）、UTF-16LE（含/不含 BOM）、查询失败、解析失败、空命令、瞬时失败重试和 stderr 隔离。
