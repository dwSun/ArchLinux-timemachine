package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// 本程序用于清理基于 linux-timemachine 生成的备份目录。
// 逻辑说明（以 BACKUP_NAME=eXile-vms 为例）：
// 1. 从环境变量读取：
//    - BACKUP_BASE：备份根目录，例如 /mnt/backup/timemachine/
//    - BACKUP_NAME：备份前缀，例如 eXile-vms
// 2. 在 BACKUP_BASE 目录下，列出所有以 BACKUP_NAME 开头的子目录：
//    例如 eXile-vms-2026-01-02-00、eXile-vms-2026-01-02-06 等。
// 3. 按名称做字符排序（即字典序），保证同一天中最早的时间点排在最前面。
// 4. 计算当前日期，对 “超过 7 天” 的日期（严格大于 7 天前的日期）：
//    - 每个日期只保留当天最早时间点对应的目录
//    - 该日期下其他同名前缀目录全部删除（rm -rf 效果，使用 os.RemoveAll）
// 5. 例行清理结束后，对 BACKUP_BASE 所在文件系统（statfs 路径与备份一致）检查可用空间；
//    若可用空间不大于 200GB，则按时间从旧到新删除合法命名的备份目录，每删一个重新统计，
//    直到可用空间大于 200GB 或无目录可删（此阶段不保留「最近 7 天」例外）。
//
// 注意：
// - 只处理目录，不会动普通文件
// - 只处理名称形如 BACKUP_NAME-YYYY-MM-DD-HH 的目录，其他格式会被跳过

// minFreeBytes 空间保障阈值：可用空间需严格大于该值（200 GiB）。
const minFreeBytes uint64 = 200 * 1024 * 1024 * 1024

// parseBackupDate 从目录名里提取日期部分并解析。
// 目录名示例：eXile-vms-2026-01-02-00
// BACKUP_NAME: eXile-vms
// 提取出的日期字符串为：2026-01-02
func parseBackupDate(dirName, backupName string) (time.Time, string, error) {
	prefix := backupName + "-"
	if !strings.HasPrefix(dirName, prefix) {
		return time.Time{}, "", fmt.Errorf("目录名[%s]不以前缀[%s]开头", dirName, prefix)
	}

	// 去掉前缀后应为：YYYY-MM-DD-HH
	rest := strings.TrimPrefix(dirName, prefix)
	parts := strings.Split(rest, "-")
	if len(parts) != 4 {
		return time.Time{}, "", fmt.Errorf("目录名[%s]格式不正确，期望类似[%s-YYYY-MM-DD-HH]", dirName, backupName)
	}

	dateStr := strings.Join(parts[0:3], "-") // YYYY-MM-DD
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("解析日期[%s]失败: %w", dateStr, err)
	}

	return parsedDate, dateStr, nil
}

// resolveStatPath 得到用于 statfs 的路径：解析符号链接，确保统计的是备份数据实际所在挂载。
func resolveStatPath(absBase string) string {
	real, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		log.Printf("解析 BACKUP_BASE 符号链接失败，将使用原路径做磁盘统计：[absBase=%s] [error=%v]", absBase, err)
		return absBase
	}
	return real
}

// availableBytes 返回 path 所在文件系统的可用字节数（与 df 中 avail 含义一致，使用 Bavail）。
func availableBytes(statPath string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(statPath, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}

// listSortedValidBackupNames 列出 BACKUP_BASE 下符合前缀且名称可解析为 BACKUP_NAME-YYYY-MM-DD-HH 的目录名（已排序）。
func listSortedValidBackupNames(absBase, backupName string) ([]string, error) {
	entries, err := os.ReadDir(absBase)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, backupName+"-") {
			continue
		}
		if _, _, err := parseBackupDate(name, backupName); err != nil {
			log.Printf("跳过目录（名称不符合备份格式）：[name=%s] [error=%v]", name, err)
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// deleteBackupWithRsync 使用与例行清理相同的方式删除单个备份目录。
func deleteBackupWithRsync(tmpEmptyDir, absBase, name string) error {
	fullPath := filepath.Join(absBase, name)
	targetPath := fullPath + "/"
	log.Printf("正在删除目录：[path=%s]", fullPath)
	cmd := exec.Command("rsync", "-av", "--delete", tmpEmptyDir+"/", targetPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync 删除目录失败：[path=%s] [error=%w]", fullPath, err)
	}
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("删除空目录失败：[path=%s] [error=%w]", fullPath, err)
	}
	log.Printf("成功删除目录：[path=%s]", fullPath)
	return nil
}

// ensureFreeSpaceAboveMin 在 statPath 上检查可用空间；不足则按最旧备份依次删除，每删一次重新统计。
// 不保留「最近 7 天」：只要空间不足且还有可删目录就会删。
func ensureFreeSpaceAboveMin(statPath, absBase, backupName, tmpEmptyDir string) {
	for {
		avail, err := availableBytes(statPath)
		if err != nil {
			log.Printf("读取备份所在文件系统可用空间失败，跳过空间保障：[statPath=%s] [error=%v]", statPath, err)
			return
		}
		log.Printf("备份所在文件系统可用空间：[statPath=%s] [availBytes=%d] [minFreeBytes=%d]", statPath, avail, minFreeBytes)
		if avail > minFreeBytes {
			log.Printf("可用空间已大于阈值，空间保障结束。")
			return
		}

		names, err := listSortedValidBackupNames(absBase, backupName)
		if err != nil {
			log.Printf("列出备份目录失败，无法继续释放空间：[BACKUP_BASE=%s] [error=%v]", absBase, err)
			return
		}
		if len(names) == 0 {
			log.Printf("可用空间仍不足且无符合命名规则的备份目录可删，请人工处理。[availBytes=%d] [minFreeBytes=%d]", avail, minFreeBytes)
			return
		}

		oldest := names[0]
		log.Printf("可用空间不足，将删除最旧备份：[name=%s]", oldest)
		if err := deleteBackupWithRsync(tmpEmptyDir, absBase, oldest); err != nil {
			log.Printf("删除备份失败，停止空间保障以免反复出错：[error=%v]", err)
			return
		}
	}
}

func main() {
	backupBase := os.Getenv("BACKUP_BASE")
	backupName := os.Getenv("BACKUP_NAME")

	if backupBase == "" || backupName == "" {
		log.Fatalf("环境变量 BACKUP_BASE 或 BACKUP_NAME 缺失：[BACKUP_BASE=%s] [BACKUP_NAME=%s]", backupBase, backupName)
	}

	// 统一为绝对路径，避免误删
	absBase, err := filepath.Abs(backupBase)
	if err != nil {
		log.Fatalf("解析 BACKUP_BASE 绝对路径失败：[BACKUP_BASE=%s] [error=%v]", backupBase, err)
	}

	statPath := resolveStatPath(absBase)
	log.Printf("开始清理备份目录：[BACKUP_BASE=%s] [BACKUP_NAME=%s] [statPath=%s]", absBase, backupName, statPath)

	entries, err := os.ReadDir(absBase)
	if err != nil {
		log.Fatalf("读取备份根目录失败：[BACKUP_BASE=%s] [error=%v]", absBase, err)
	}

	tmpEmptyDir := "/tmp/tm-cleaner-empty"
	if err := os.MkdirAll(tmpEmptyDir, 0755); err != nil {
		log.Fatalf("创建临时空目录失败：[path=%s] [error=%v]", tmpEmptyDir, err)
	}
	defer func() {
		if err := os.RemoveAll(tmpEmptyDir); err != nil {
			log.Printf("清理临时空目录失败：[path=%s] [error=%v]", tmpEmptyDir, err)
		} else {
			log.Printf("已清理临时空目录：[path=%s]", tmpEmptyDir)
		}
	}()

	// 收集所有符合前缀且为目录的名称
	var dirNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, backupName+"-") {
			dirNames = append(dirNames, name)
		}
	}

	if len(dirNames) == 0 {
		log.Printf("未找到任何以指定前缀开头的备份目录：[BACKUP_BASE=%s] [BACKUP_NAME=%s]", absBase, backupName)
		ensureFreeSpaceAboveMin(statPath, absBase, backupName, tmpEmptyDir)
		return
	}

	// 字典序排序
	sort.Strings(dirNames)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	threshold := today.AddDate(0, 0, -7) // 7 天前的日期

	log.Printf("当前日期：[today=%s] [threshold(7天前)=%s]", today.Format("2006-01-02"), threshold.Format("2006-01-02"))

	// 记录每个日期已保留的最早目录名
	keptForDate := make(map[string]string)

	var toDelete []string

	for _, name := range dirNames {
		backupDate, dateKey, err := parseBackupDate(name, backupName)
		if err != nil {
			// 非预期格式，跳过但记录日志
			log.Printf("跳过目录，名称不符合备份格式：[name=%s] [error=%v]", name, err)
			continue
		}

		// 判断是否“超过 7 天”
		if !backupDate.Before(threshold) {
			// 在 7 天内或正好为阈值日期及之后，不做删除操作
			log.Printf("保留最近 7 天内的备份目录：[name=%s] [date=%s]", name, dateKey)
			continue
		}

		// 超过 7 天的目录：同一日期只保留最早时间点（因为已按名称排序）
		if _, exists := keptForDate[dateKey]; !exists {
			keptForDate[dateKey] = name
			log.Printf("保留超过 7 天中每日期的最早备份目录：[date=%s] [keep=%s]", dateKey, name)
		} else {
			toDelete = append(toDelete, name)
		}
	}

	if len(toDelete) == 0 {
		log.Printf("没有需要删除的旧备份目录。")
	} else {
		log.Printf("准备删除旧备份目录数量：[count=%d]", len(toDelete))
		for _, name := range toDelete {
			log.Printf("待删除目录：[name=%s]", name)
		}

		log.Printf("使用 rsync 方式批量删除，临时空目录：[tmpEmptyDir=%s]", tmpEmptyDir)

		for _, name := range toDelete {
			if err := deleteBackupWithRsync(tmpEmptyDir, absBase, name); err != nil {
				log.Printf("例行清理删除失败，请手动检查：[error=%v]", err)
			}
		}
	}

	log.Printf("旧备份清理完成。")
	ensureFreeSpaceAboveMin(statPath, absBase, backupName, tmpEmptyDir)
}
