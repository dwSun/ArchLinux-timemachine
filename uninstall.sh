#!/bin/bash

if [ "$(id -u)" -ne 0 ]; then
  echo "错误：必须以 root 运行本脚本（例如: sudo $0），以移除 root crontab 中的任务。" >&2
  exit 1
fi

existing=$(crontab -l 2>/dev/null) || existing=""
if echo "$existing" | grep -qF 'do_incremental_rsync.sh'; then
  echo "正在从 root 的 crontab 中移除 do_incremental_rsync.sh ..."
  echo "$existing" | grep -vF 'do_incremental_rsync.sh' | crontab -
else
  echo "root crontab 中未找到 do_incremental_rsync.sh，无需更改。"
fi

echo "卸载步骤完成。"
