// Package storage 实现附件（头像）的磁盘存储。
//
// 目录布局逐字对应 application/libraries/storage/disk.php：
//
//	<file-storage>/<storageName>/<c1>/<c2>/<c3>/<c4>/<c5>/<fileName>
//
// 其中 c1..c5 是 fileName 的前 5 个字符 —— 这是 PHP 的 getPathByFileName() 规则，
// 决定了"文件名 → 磁盘路径"的映射。**不可变更**，否则既有头像全部失效。
package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound 表示文件不存在。
var ErrNotFound = errors.New("文件不存在")

// ErrPathEscape 表示路径越界。
var ErrPathEscape = errors.New("路径越界")

// Disk 是基于本地磁盘的存储。
type Disk struct {
	// root 是 file-storage 根目录的绝对路径。
	root string
}

// NewDisk 创建磁盘存储。
func NewDisk(root string) *Disk { return &Disk{root: root} }

// Root 返回 file-storage 根目录。
func (d *Disk) Root() string { return d.root }

// StoragePath 返回某类存储的根目录（例如 avatar）。
func (d *Disk) StoragePath(name string) string {
	return filepath.Join(d.root, name)
}

// PathByFileName 返回文件在存储中的相对路径。
//
// 逐字对应 PHP 的 Storage_Disk::getPathByFileName()：
// 取 fileName 的前 5 个字符作为 5 级目录，最后一级是完整文件名。
func PathByFileName(fileName string) string {
	if fileName == "" {
		return ""
	}
	parts := make([]string, 0, 6)
	for i := 0; i < 5 && i < len(fileName); i++ {
		parts = append(parts, fileName[i:i+1])
	}
	parts = append(parts, fileName)
	return filepath.Join(parts...)
}

// AbsolutePath 把相对路径解析为绝对路径，并校验没有越出存储根。
func (d *Disk) AbsolutePath(storageName, rel string) (string, error) {
	base := d.StoragePath(storageName)
	abs := filepath.Join(base, rel)
	cleanBase := filepath.Clean(base)
	cleanAbs := filepath.Clean(abs)
	if cleanAbs != cleanBase && !strings.HasPrefix(cleanAbs, cleanBase+string(os.PathSeparator)) {
		return "", ErrPathEscape
	}
	return cleanAbs, nil
}

// Save 按 fileName 写入内容。
func (d *Disk) Save(storageName, fileName string, content []byte) error {
	abs, err := d.AbsolutePath(storageName, PathByFileName(fileName))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

// SaveUploaded 把上传的临时文件保存为 fileName。
func (d *Disk) SaveUploaded(storageName, fileName, srcPath string) error {
	abs, err := d.AbsolutePath(storageName, PathByFileName(fileName))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	// 先写临时文件再改名，避免半截文件被读到。
	tmp := abs + ".tmp"
	if err := copyFile(srcPath, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, abs); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存文件失败: %w", err)
	}
	return nil
}

// Read 按 fileName 读取内容。
func (d *Disk) Read(storageName, fileName string) ([]byte, error) {
	abs, err := d.AbsolutePath(storageName, PathByFileName(fileName))
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return content, nil
}

// Remove 按 fileName 删除文件。
func (d *Disk) Remove(storageName, fileName string) error {
	abs, err := d.AbsolutePath(storageName, PathByFileName(fileName))
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// copyFile 复制文件。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}
	return out.Sync()
}
