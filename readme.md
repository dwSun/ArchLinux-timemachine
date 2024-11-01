
需要提前设置好ssh密钥，以便无密码登录。注意密钥也需要放在客户机的root用户下。


这个是放在root用户下的crontab文件，每天12点15分执行一次。

```
15 12 * * * bash -c "cd /home/david/timemachine/ && ./do_incremental_rsync.sh"
```
