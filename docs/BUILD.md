# 构建与发布

## 1. 环境

- Go 1.27+（以 `go.mod` 为准）
- **不需要 C 编译器**：依赖 `github.com/jchv/go-webview2` 是纯 syscall 实现，`CGO_ENABLED=0` 即可
- 交叉编译可在 Linux / macOS / WSL 上进行，无需 Windows

依赖（`go.mod`）：

| 模块 | 用途 |
| --- | --- |
| `github.com/jchv/go-webview2` | WebView2 宿主，纯 Go syscall 封装 |
| `github.com/jchv/go-winloader` | 间接依赖 |
| `golang.org/x/sys` | Windows API |

## 2. 一键构建

```bash
./build.sh              # 生成 WinAutoStartManager.exe 并打印 SHA256
./build.sh --clean      # 先删除旧产物再构建
```

脚本内固化的命令：

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -buildvcs=false \
  -ldflags "-s -w -H windowsgui" \
  -o WinAutoStartManager.exe .
```

Windows PowerShell 等价写法：

```powershell
$env:CGO_ENABLED="0"
go build -trimpath -buildvcs=false -ldflags "-s -w -H windowsgui" -o WinAutoStartManager.exe .
```

## 3. 各参数的作用

| 参数 | 作用 | 去掉会怎样 |
| --- | --- | --- |
| `CGO_ENABLED=0` | 纯 Go 静态构建 | Linux 上需要 Windows 交叉 C 工具链 |
| `-trimpath` | 去掉源码绝对路径 | 二进制里泄露构建机路径，且不同机器不一致 |
| `-buildvcs=false` | 不嵌入 git commit / 时间戳 | **每次 commit 后构建的 hash 都不同**，见下节 |
| `-s -w` | 去符号表与 DWARF | 体积约 +1 MB，无实际收益 |
| `-H windowsgui` | GUI 子系统 | 双击时闪出控制台黑窗 |

## 4. 为什么必须 `-buildvcs=false`

`-trimpath` 无法消除 Go 嵌入的模块版本戳。同一个 commit、同一个 tag 下，两种环境的产物不同：

| 构建环境 | 嵌入的构建信息 |
| --- | --- |
| git 仓库内（干净） | `mod WinAutoStartManager v0.0.0-20261021182511-49f907c6a0b7`<br>`vcs=git` / `vcs.revision` / `vcs.time` / `vcs.modified=false` |
| git 仓库内（有改动） | 同上，但 `vcs.modified=true` |
| 非 git 目录（如解压的 zip） | `mod WinAutoStartManager (devel)`，无 `vcs.*` 字段 |

因为模块本身处于版本控制之下，Go 会把 commit hash 与时间写进 `runtime/debug.BuildInfo`。结果是：**同一份源码，在不同 clone、不同时间构建，字节不同。**

加上 `-buildvcs=false` 后，三者的产物逐字节一致。验证方式：

```bash
git clone <repo> /tmp/verify && cd /tmp/verify && ./build.sh
cmp /tmp/verify/WinAutoStartManager.exe <原构建>/WinAutoStartManager.exe && echo IDENTICAL
```

当前确定性构建的 SHA256：

```
c39ee3217f7ae580bdff8d1658d42c393d46f086a302b024c8c8960ba8ecda14
```

> 该 hash 对应 `-buildvcs=false` 的构建。历史上带 VCS 戳记的构建是 `7f391078...`，两者功能完全相同，仅构建元数据不同。

## 5. 图标资源

图标链条：

```
resources/make-icon.ps1  →  resources/app.ico  →  rsrc.syso  →  链接进 exe
```

`rsrc.syso` **已纳入版本控制**，因此 clone 后直接构建就能得到带图标的 exe，无需在 Windows 上重新生成。它被 Go 构建器自动拾取（`.syso` 文件会作为资源对象链接）。

验证资源是否嵌入：

```bash
python3 - <<'EOF'
import struct
d = open('WinAutoStartManager.exe','rb').read()
pe = struct.unpack_from('<I', d, 0x3c)[0]
nsec   = struct.unpack_from('<H', d, pe+6)[0]
optsz  = struct.unpack_from('<H', d, pe+20)[0]
sec, opt = pe+24+optsz, pe+24
for i in range(nsec):
    s = sec + i*40
    if d[s:s+8].rstrip(b'\0') == b'.rsrc':
        print("ICON resources embedded, .rsrc size =", struct.unpack_from('<I', d, s+16)[0])
EOF
```

### 重新生成图标

需要 Windows PowerShell（依赖 `System.Drawing`）：

```powershell
powershell -ExecutionPolicy Bypass -File resources\make-icon.ps1

go install github.com/akavel/rsrc@latest
& "$(go env GOPATH)\bin\rsrc.exe" -ico resources\app.ico -o rsrc.syso

go build -trimpath -buildvcs=false -ldflags "-s -w -H windowsgui" -o WinAutoStartManager.exe .
```

`resources/make-icon.ps1` 会把中间产物写进 `resources/build/`。

> 关于清单：本程序**有意不嵌 `requireAdministrator` 清单**。主程序平时以普通权限运行（编辑配置不需要管理员），提权在「开启自启」时由代码通过计划任务发起。因此 PE 中没有 MANIFEST 资源是预期行为，Windows 按 `asInvoker` 处理。

## 6. 发布检查清单

```powershell
go test ./... -count=1
go vet ./...
gofmt -l .\internal .\main.go     # 应无输出
```

交叉编译代码时（非 Windows 主机）需指定目标，否则 Windows 专属文件会被构建约束排除：

```bash
GOOS=windows GOARCH=amd64 go build ./...
GOOS=windows GOARCH=amd64 go vet ./...
```

直接 `go build ./...` 在 Linux 上报 `build constraints exclude all Go files` 属于正常现象，不是代码错误。

发布前确认：

- [ ] `go test` / `go vet` 全部通过
- [ ] 产物为 PE32+、x86-64、Subsystem `2`（GUI）
- [ ] `.rsrc` 段存在且含 ICON
- [ ] 记录本次构建的 SHA256 与大小
- [ ] 在真实 Windows 上跑一遍：启动 → 新建条目 → 测试运行 → 开启自启 → 注销重登验证静默启动

确认子系统（PowerShell）：

```powershell
$b = [IO.File]::ReadAllBytes('.\WinAutoStartManager.exe')
$pe = [BitConverter]::ToInt32($b, 0x3C)
'subsystem=' + [BitConverter]::ToUInt16($b, $pe + 0x5C)   # 2 = GUI, 3 = 控制台
```
