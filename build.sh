#!/usr/bin/env bash
# 交叉编译 Windows GUI 可执行文件（可在 Linux / macOS / WSL 上运行）。
#
# 为保证「任何新环境 clone 后都能得到逐字节相同的包」，本脚本固定了：
#   - CGO_ENABLED=0        纯 Go 构建，无需 C 工具链
#   - -trimpath            去掉本机绝对路径
#   - -buildvcs=false      不嵌入 git commit / 时间戳
#                          （否则每个 clone 的产物 hash 都不同）
#   - -s -w -H windowsgui  去符号与调试信息，且运行时不弹控制台窗口
#
# rsrc.syso 已纳入版本控制，图标资源无需重新生成即可链接进 exe。
#
# 用法：
#   ./build.sh              # 生成 WinAutoStartManager.exe
#   ./build.sh --clean      # 先删除旧产物再构建
set -euo pipefail

cd "$(dirname "$0")"

OUT="WinAutoStartManager.exe"

if [[ "${1:-}" == "--clean" ]]; then
	rm -f "$OUT"
fi

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	go build -trimpath -buildvcs=false \
	-ldflags "-s -w -H windowsgui" \
	-o "$OUT" .

echo "built: $OUT"
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum "$OUT"
elif command -v shasum >/dev/null 2>&1; then
	shasum -a 256 "$OUT"
fi
