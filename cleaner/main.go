package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
//
// 注意：
// - 只处理目录，不会动普通文件
// - 只处理名称形如 BACKUP_NAME-YYYY-MM-DD-HH 的目录，其他格式会被跳过

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

func main() {
	backupBase := os.Getenv("BACKUP_BASE")
	backupName := os.Getenv("BACKUP_NAME")

	if backupBase == "" || backupName == "" {
		log.Fatalf("环境变量 BACKUP_BASE 或 BACKUP_NAME 缺失：[BACKUP_BASE=%s] [BACKUP_NAME=%s]", backupBase, backupName)
		return
	}

	// 统一为绝对路径，避免误删
	absBase, err := filepath.Abs(backupBase)
	if err != nil {
		log.Fatalf("解析 BACKUP_BASE 绝对路径失败：[BACKUP_BASE=%s] [error=%v]", backupBase, err)
	}

	log.Printf("开始清理备份目录：[BACKUP_BASE=%s] [BACKUP_NAME=%s]", absBase, backupName)

	entries, err := os.ReadDir(absBase)
	if err != nil {
		log.Fatalf("读取备份根目录失败：[BACKUP_BASE=%s] [error=%v]", absBase, err)
	}

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
		return
	}

	log.Printf("准备删除旧备份目录数量：[count=%d]", len(toDelete))

	for _, name := range toDelete {
		fullPath := filepath.Join(absBase, name)
		log.Printf("删除旧备份目录：[path=%s]", fullPath)
		if err := os.RemoveAll(fullPath); err != nil {
			log.Printf("删除目录失败，请手动检查：[path=%s] [error=%v]", fullPath, err)
		}
	}

	log.Printf("旧备份清理完成。")
}
