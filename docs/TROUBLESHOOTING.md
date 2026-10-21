# 故障排查

排查前先确认两点：**运行的是最新构建**，以及**计划任务的真实状态**。

```powershell
# 确认两个任务是否存在、指向何处
Get-ScheduledTask -TaskName AutoStartManager-Admin, AutoStartManager-User |
  Select-Object TaskName, State,
    @{n='RunLevel';e={$_.Principal.RunLevel}},
    @{n='User';e={$_.Principal.UserId}}

(Get-ScheduledTask -TaskName AutoStartManager-Admin).Actions |
  Select-Object Execute, Arguments, WorkingDirectory

# 确认 exe 版本
Get-FileHash .\WinAutoStartManager.exe -Algorithm SHA256
```

---

## 开关显示「未开启」，但计划任务确实存在

先查任务 XML：

```powershell
schtasks /Query /TN AutoStartManager-Admin /XML
schtasks /Query /TN AutoStartManager-User /XML
```

当前版本已修复中文 Windows 上因 GBK 输出导致的误判（详见 [DEVELOPMENT.md](DEVELOPMENT.md) 的输出编码章节）。若仍出现，请确认：

1. 运行的是最新 exe（对比 SHA256）；
2. 安全软件没有拦截 `schtasks`；
3. 抓取原始输出，见下节。

## 开关显示「状态未知」

含义：任务查询或 XML 解析失败，程序无法确认真实状态，因此**没有改动开关**（这是刻意的，避免查询失败被误当成「未安装」而重复创建任务）。

常见原因：

- `schtasks` 被安全软件拦截或不可用；
- 任务存在但当前用户没有读取权限；
- 任务 XML 损坏。

处理：手动跑上面的 `/Query` 命令确认；或重新打开程序重试（程序内部会自动重试一次）。

## 打开程序或切换开关时闪出 cmd 黑框

当前构建已通过 `CREATE_NO_WINDOW` 消除该现象。若仍出现：

1. 核对 exe 的 SHA256 是否与最新构建一致 —— 旧版才会闪窗；
2. 确认没有其他程序（如自定义脚本）在调用 `schtasks`。

## 抓取 `schtasks` 原始输出（不经过 PowerShell 解码）

用于确认输出编码问题。在目标机器上运行：

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

- `ACP=936` + `utf8_has_desc=False` + `gbk_has_desc=True`，且 `<Description>` 后出现 `BF AA BB FA` 一类字节 → 输出确实是 GBK，当前 `decodeOutput` 会按 `CP_ACP` 解码，不应失败。
- 若仍失败，检查 `decodeANSI` 是否被正确编译进 Windows 构建。

## 首次运行被 SmartScreen 拦截

程序未做代码签名。选择「更多信息 → 仍要运行」。

若希望彻底消除，需购买代码签名证书并对 exe 签名 —— 目前不做。

## 提示无法创建窗口 / 界面空白

缺少 Microsoft Edge WebView2 运行时。Windows 11 自带；Windows 10 请安装 [Evergreen Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)。

安装后重试，无需重启。

## 登录后程序没有启动

按顺序排查：

1. **开关是否为「已开启」**。若是「需更新」，说明 exe 被移动过，关掉再打开开关。
2. **条目是否启用**（左侧绿点）。
3. **看日志**：界面左下「打开日志」→ `logs\launcher-<日期>-<层>.log`。
4. **看失败通知**：失败会弹一条 Windows 通知，成功则完全静默。
5. **检查任务运行结果**：

```powershell
schtasks /Query /TN AutoStartManager-Admin /V /FO LIST | Select-String 'Last Result','上次结果'
```

6. **数据目录在可移动磁盘上**：登录时磁盘未就绪会失败，日志中会有明确记录。

## 管理员条目启动失败，日志提示需要管理员权限

标准用户账户无法创建最高权限任务。管理员层需要当前账户属于 **Administrators** 组。

另外：普通层若遇到自身 manifest 要求管理员的程序，会失败（登录时无人点 UAC），日志会记录「该项需要管理员权限，请在 GUI 中勾选管理员」，并计入失败。把该条目的权限层改成「管理员」即可。

## 提权没生效 / 不弹 UAC

检查 `EnableLUA`：

```powershell
whoami /groups | Select-String 'Mandatory Label'
Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System' -Name EnableLUA
```

`EnableLUA=0` 时进程直接为 High，`runas` 不再弹 UAC —— 这是预期行为，不是故障。

## 确认 PE 子系统

若怀疑构建参数不对（例如闪黑框）：

```powershell
$b = [IO.File]::ReadAllBytes('.\WinAutoStartManager.exe')
$pe = [BitConverter]::ToInt32($b, 0x3C)
'subsystem=' + [BitConverter]::ToUInt16($b, $pe + 0x5C)
```

- `2` = GUI（无控制台）✅
- `3` = 控制台（构建时漏了 `-H windowsgui`）
