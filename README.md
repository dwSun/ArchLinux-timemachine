Linux 时间机器
------------------


## 致谢

这是从 [Linux TimeMachine](https://github.com/ekenberg/linux-timemachine) fork 过来的，感谢原作者慷慨无私的分享，本脚本在原作者的基础上做了一些修改，使其更适合我的使用场景。

## 使用说明

使用硬链接的 Rsync 增量备份。节省时间和空间，以及保护您的数据。

Mac 电脑通过内置的 [Time Machine](http://en.wikipedia.org/wiki/Time_Machine_%28Mac_OS%29) 实现了自动增量备份。

[Apple TimeMachine](http://ekenberg.github.io/linux-timemachine/images/mac-timemachine.png)

Linux 拥有 rsync、bash 和 cron。Rsync 可以利用 [硬链接](http://en.wikipedia.org/wiki/Hard_link) 对未更改的文件进行处理：只有自上次备份以来发生变化的文件才会被复制。这可以节省大量时间和存储空间。

### 预备条件
* 备份到支持硬链接和软链接的文件系统。除了 FAT 或 NTFS（微软的产品）之外没有问题。推荐使用 btrfs 并开启压缩功能，这可以进一步节省空间。
* 本脚本是在**备份目标机器**上运行的，因此目标机与源机都需要安装 `rsync`。
* 需要提前配置 SSH 密钥以便无密码登录。若备份的是 root 权限下的内容，密钥也需放在源机对应用户的 `~/.ssh` 中（通常为 root）。
* 若直接备份到本机挂载的存储设备，可不必配置 SSH，但需自行修改 `do_incremental_rsync.sh` 中的 `rsync` 参数（去掉 `ssh` 相关选项）。
* **首次运行增量备份前**，请在仓库内编译清理工具：`cd cleaner && go build -o cleaner`。`do_incremental_rsync.sh` 会在每个配置的备份任务开始前执行 `./cleaner/cleaner`（见脚本中的调用）；若二进制不存在，对应备份会失败。

### 如何操作
* 在 config文件夹中设置备份配置，可以设置多个备份目录，以及备份对象，每个备份对象可以设置排除的目录。
* 在 config/exclude 中设置排除路径
* 建议先用**体量较小的源路径**、单独的配置文件做一次试跑，观察终端与日志输出：`sudo ./do_incremental_rsync.sh`（脚本未解析 `-v`；需要更详细输出时可直接编辑脚本里 `rsync` 行或临时增加 `set -x`）。
* 首次全备份需要很长时间，因为所有文件都需要复制。
* 最后，通过 cron 安排固定时间以 root 身份运行。`do_incremental_rsync.sh` 会检查当前用户是否为 root（`id -u` 为 0），因此 **cron 必须写在 root 用户的 crontab 里**，否则定时任务每次都会失败。仓库中的 `install.sh` / `uninstall.sh` 必须以 root 执行（例如 `sudo ./install.sh`），会向 **root 的 crontab** 添加或移除任务，并使用**脚本所在目录**作为工作路径（无需再手改 `/home/某用户/timemachine`）。

下面等价于 `install.sh` 写入的 crontab 行：每隔 6 小时、在每小时的第 15 分钟执行。

```
15 */6 * * * bash -c "cd /path/to/timemachine/ && ./do_incremental_rsync.sh"
```

将 `/path/to/timemachine/` 换为你克隆本仓库的实际路径。若路径中含空格或特殊字符，手写 cron 时须对 `cd` 的目标路径做正确转义或引号；**推荐**使用 `sudo ./install.sh`，脚本会用 `printf %q` 生成安全的 `cd` 参数。


### 检查硬链接
为了验证硬链接确实起作用，可以使用 `stat` 命令检查最近备份中某个已知一段时间未改变的文件。`stat` 显示一个字段 `Links: #`，该字段显示文件有多少个硬链接。我的 /etc/fstab 已经很长时间没有改变了：

[Stat 输出](http://ekenberg.github.io/linux-timemachine/images/stat-verify-hard-links.jpg)

### 注意事项
* _重要提示：_ 为了让硬链接工作，第一个备份必须是全系统的备份。为什么？因为脚本在运行时会更新当前链接。如果一天中的第一个备份是针对 /home/user/some/directory 的，并且当前链接被更新，那么当执行全备份时，它将通过当前链接查找最后一次备份，但只能找到 /home/user/some/directory 中的文件，因此必须重新复制所有内容。这将浪费大量的空间！
* 备份目录名由脚本中的 `$TODAY` 决定，当前格式为 **`YYYY-MM-DD-HH`（含小时）**。同一小时内若多次运行，会写入**同名目录**（后一次相当于在同目录上继续/覆盖同步）；不同小时会得到不同子目录。若需更高频率的独立快照，可修改 `$TODAY` 的 `date` 格式（例如加入分钟）。
* rsync 是与 --one-file-system 选项一起运行的。如果您有几个文件系统需要备份，请单独设置备份配置文件。
* rsync 的 --delete 选项不会删除硬链接。如果您删除了备份中的文件，硬链接将保留，直到所有硬链接都被删除。这是 rsync 的默认行为，但是请注意，如果您使用了其他选项，可能会删除硬链接。
* 请注意，针对每个备份配置，本脚本都会启动一个单独的rsync 进程。如果您有多个配置，可能会同时运行多个rsync进程。这可能会导致系统负载增加，因此请注意。

## 清理旧备份工具（cleaner）

仓库下的 `cleaner` 目录提供了一个简单的 Go 程序，用于**定期清理旧的增量备份目录**，以免长期占用过多磁盘空间。

### 命名约定

假设某一类备份的前缀为 `BACKUP_NAME=eXile-vms`，则备份目录在 `BACKUP_BASE` 下的命名约定如下：

例如：

* eXile-vms-2025-12-26-00/
* eXile-vms-2025-12-26-06/
* eXile-vms-2025-12-29-00/
* eXile-vms-2025-12-29-06/
* eXile-vms-2026-01-02-00/
* eXile-vms-2026-01-02-06/
* eXile-vms-2026-01-02-12/
* eXile-vms-2026-01-02-18/
* eXile-vms-2026-01-03-00/
* eXile-vms-2026-01-03-06/
* eXile-vms-2026-01-03-12/
* eXile-vms-2026-01-03-18/
* eXile-vms-2026-01-04-00/
* eXile-vms-2026-01-04-06/
* eXile-vms-2026-01-05-00/
* eXile-vms-2026-01-06-00/
* eXile-vms-2026-01-06-06/
* eXile-vms-2026-01-07-00/
* eXile-vms-2026-01-07-06/

即：`BACKUP_NAME-YYYY-MM-DD-HH/` 的形式，其中：

* `YYYY`：年份
* `MM`：月份
* `DD`：日期
* `HH`：小时（一天可以多次备份）

### 清理规则

* 程序会读取当前日期（以本机时间为准）
* 对于**备份目录名中日期不早于「今天零点往前数第 7 个日历日」**的备份（代码中与 `threshold = today - 7 天` 比较）：**全部保留，不做删除**（该范围内同一天可有多个 `HH` 目录，均保留）。
* 对于**日期早于上述阈值**的备份目录：
  * 按日期（YYYY-MM-DD）分组
  * 每个日期只保留**当天最早时间点的一个目录**
  * 该日期下的其他同名前缀目录**全部删除**

换句话说：**备份目录名中的日期若落在「今天 −7 天」及之后，则该日期的所有时间点目录都保留；若日期更早，则每个日历日只保留目录名排序最靠前（通常即当日最早一次备份）的那一份，其余同日前缀目录会被删掉。**

### 空间保障（例行清理之后）

在**上述例行清理执行完毕后**，程序会对 **`BACKUP_BASE` 解析后的真实路径**（含符号链接解析）所在**文件系统**调用 `statfs`，读取**可用空间**（与 `df` 中 avail 含义一致）。日志里会以 IEC 二进制单位（KiB、MiB、GiB、TiB 等）输出当前可用空间与 200 GiB 阈值，便于阅读。

* 若可用空间**不大于** **200 GiB**（`200 * 1024^3` 字节），则按目录名字典序（即时间从旧到新）依次删除**名称符合** `BACKUP_NAME-YYYY-MM-DD-HH` 的备份目录；每删除一个目录后**重新统计**该文件系统可用空间，直到可用空间**大于** 200 GiB，或没有可删目录为止。
* 此阶段**不**保留「最近 7 天」例外：只要空间不足且仍有合法命名的备份目录，就会从最旧开始删。
* 若 `BACKUP_BASE` 与其他数据共用同一文件系统（例如同一块盘的 btrfs），`statfs` 反映的是**整块文件系统**的空闲，而非仅备份子目录独占的空间；名称不规范的目录不会被自动删除（仅打日志跳过）。

### 使用方法

进入 `cleaner` 目录后，可以按如下方式使用：

1. 设置环境变量：

   * `BACKUP_BASE`：备份存放的根目录，例如 `/mnt/backup/timemachine`
   * `BACKUP_NAME`：要清理的某一类备份前缀，例如 `eXile-vms`

2. 编译：

   ```bash
   cd cleaner
   go build -o cleaner
   ```

3. 运行（示例）：

   ```bash
   BACKUP_BASE=/mnt/backup/timemachine BACKUP_NAME=eXile-vms ./cleaner
   ```

4. 可以结合 `cron` 定期执行该程序，用于自动清理旧的备份点。

程序内部会输出详细日志（包括哪些目录被保留、哪些目录被删除），方便你在早期阶段观察清理行为是否符合预期。

