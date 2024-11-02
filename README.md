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
* 备份到支持硬链接和软链接的文件系统。除了 FAT 或 NTFS（微软的产品）之外没有问题。推荐使用btrfs并开启压缩功能，这可以进一步节省空间。
* 本脚本是在备份的目标机器上运行的，所以需要在备份的目标机器和源机器都安装好rsync。
* 需要提前设置好ssh密钥，以便无密码登录。请注意如果备份的内容是root用户的，那么密钥也需要放在客户机的root用户下。
* 当然如果有方便的存储设备，也可以直接备份到存储设备上，这样就不需要设置ssh密钥了。但请注意修改一下脚本中的rsync参数，去掉ssh参数。

### 如何操作
* 在 config文件夹中设置备份配置，可以设置多个备份目录，以及备份对象，每个备份对象可以设置排除的目录。
* 在 config/exclude 中设置排除路径
* 使用一些小目录和 -v 参数测试：`sudo do_incremental_rsync.sh -v`
* 首次全备份需要很长时间，因为所有文件都需要复制。
* 最后，通过 cron 安排固定时间以 root 身份运行。因为我这里是办公室使用的机器，所以我设置了每天12点15分执行一次。正好是午餐的时间，不会影响到我工作。

下面是我使用的crontab文件，每天12点15分执行一次：


```
15 12 * * * bash -c "cd /home/david/timemachine/ && ./do_incremental_rsync.sh"
```

注意，我这里因为需要备份根目录，所以这个配置文件是放在root用户下的，如果只是备份普通用户的目录，那么可以放在普通用户下。


### 检查硬链接
为了验证硬链接确实起作用，可以使用 `stat` 命令检查最近备份中某个已知一段时间未改变的文件。`stat` 显示一个字段 `Links: #`，该字段显示文件有多少个硬链接。我的 /etc/fstab 已经很长时间没有改变了：

[Stat 输出](http://ekenberg.github.io/linux-timemachine/images/stat-verify-hard-links.jpg)

<a name='notes'/>

### 注意事项
* _重要提示：_ 为了让硬链接工作，第一个备份必须是全系统的备份。为什么？因为脚本在运行时会更新当前链接。如果一天中的第一个备份是针对 /home/user/some/directory 的，并且当前链接被更新，那么当执行全备份时，它将通过当前链接查找最后一次备份，但只能找到 /home/user/some/directory 中的文件，因此必须重新复制所有内容。这将浪费大量的空间！
* 我每天做一次备份，脚本会用当前日期作为目录名称来存储备份。因此，当天的任何额外备份都会覆盖当前日期的备份。对我来说这是可以接受的，但如果您希望保留更频繁的副本，应该查看脚本中的 `$TODAY` 变量。也许可以在格式中添加小时或小时-分钟。
* rsync 是与 --one-file-system 选项一起运行的。如果您有几个文件系统需要备份，请单独设置备份配置文件。
* rsync 的 --delete 选项不会删除硬链接。如果您删除了备份中的文件，硬链接将保留，直到所有硬链接都被删除。这是 rsync 的默认行为，但是请注意，如果您使用了其他选项，可能会删除硬链接。
* 请注意，针对每个备份配置，本脚本都会启动一个单独的rsync 进程。如果您有多个配置，可能会同时运行多个rsync进程。这可能会导致系统负载增加，因此请注意。
