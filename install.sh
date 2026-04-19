#!/bin/bash

# do_incremental_rsync.sh 必须以 root 运行；cron 必须写入 root 的 crontab。
if [ "$(id -u)" -ne 0 ]; then
  echo "错误：必须以 root 运行本脚本（例如: sudo $0）。" >&2
  echo "do_incremental_rsync.sh 会检查 uid，非 root 时备份无法执行。" >&2
  exit 1
fi

INSTALL_DIR="$(cd "$(dirname "$0")" && pwd)"
# 供 bash -c 内 cd 使用：含空格或特殊字符时也能正确解析（printf %q 生成可复用的 shell 转义）
INSTALL_DIR_SH=$(printf '%q' "$INSTALL_DIR")

# 从 root crontab 中移除相关行；若本就没有 crontab 或未包含该任务，不向 crontab - 写入空输入，避免误清空。
existing=$(crontab -l 2>/dev/null) || existing=""
if echo "$existing" | grep -qF 'do_incremental_rsync.sh'; then
  echo "正在从 root 的 crontab 中移除 do_incremental_rsync.sh ..."
  echo "$existing" | grep -vF 'do_incremental_rsync.sh' | crontab -
fi

# 每隔 6 小时的第 15 分钟执行；路径为当前仓库目录
echo "正在将 do_incremental_rsync.sh 安装到 root 的 crontab..."
(
  crontab -l 2>/dev/null || true
  echo "15 */6 * * * bash -c \"cd ${INSTALL_DIR_SH} && ./do_incremental_rsync.sh\""
) | sort | uniq | crontab -

echo "安装完成。"
