// Package config 负责加载 env.yaml 与 config.yaml。
//
// 配置文件结构与现有 PHP 后端保持完全一致 —— 同一个 env.yaml 要同时被
// PHP 后端、http-gateway、ssh-gateway 与 misc/hooks/main 读取，
// 因此键名与层级不能有任何改动。
//
// PHP 侧通过 www/index.php 的 make_constants('YAML_', ...) 把嵌套 YAML 展平成
// 大写下划线常量（先 env.yaml 后 config.yaml，后者覆盖同名键）；
// Go 侧直接按结构体解析，等价且更安全。
//
// 少数**只属于 Go 后端**的开关不写进 env.yaml（那是共享文件），而是走环境变量，
// 例如 CODEFEVER_PASSWORD_HASH —— 见各自字段的说明。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"codefever-next/backend/internal/auth"
)

// Env 是 env.yaml 的结构。
type Env struct {
	Env           string  `yaml:"env"`
	ReleaseNumber string  `yaml:"releaseNumber"`
	MySQL         MySQL   `yaml:"mysql"`
	Session       Session `yaml:"session"`
	TOTP          TOTP    `yaml:"totp"`
	Path          Path    `yaml:"path"`
	CLI           CLI     `yaml:"cli"`
	Users         Users   `yaml:"users"`
	Gateway       Gateway `yaml:"gateway"`
	Auth          Auth    `yaml:"auth"`
}

// Auth 是与认证相关的部署配置。
//
// 注意：这一段是**只有 Go 后端会读**的键，不写进共享的 env.yaml
// （env.yaml 同时被 PHP 后端与两个 Go 网关读取，键结构不能变）。
// 实际取值走环境变量 CODEFEVER_PASSWORD_HASH；env.yaml 里的
// auth.hash_format 只是可选的同义入口，便于运维就地修改。
type Auth struct {
	// HashFormat 决定新口令的哈希写入格式：legacy（md5(md5(x))）或 argon2id。
	//
	// 默认 legacy —— PHP 后端只会用 md5(md5($password)) 校验口令，
	// 所以只有 legacy 格式能被 PHP 验证。双后端共存期间必须保持 legacy，
	// 否则被 Go 改写过的账号将无法在 PHP 侧登录，"秒级回滚"随之失效。
	HashFormat string `yaml:"hash_format"`
}

// MySQL 是数据库连接配置。
type MySQL struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       string `yaml:"db"`
}

// Session 是会话配置。
type Session struct {
	Domain string `yaml:"domain"`
	Name   string `yaml:"name"`
	Key    string `yaml:"key"`
}

// TOTP 是验证码盐值配置。
type TOTP struct {
	Salt string `yaml:"salt"`
}

// Path 是存储路径配置。
//
// 注意：现有 PHP 代码硬编码了 '/git-storage' 而没有读取 path.git，
// 重写后新后端以本字段为准，从而消除配置漂移。
type Path struct {
	File string `yaml:"file"`
	Git  string `yaml:"git"`
}

// CLI 是外部命令路径配置。
type CLI struct {
	// Git 是 git 可执行体的完整路径，容器内是 /usr/bin/git。
	//
	// 注意：**目录部分**（/usr/bin）会被写进 authorized_keys 的强制命令里
	// （PATH=$PATH:<目录>），供 SSH 登录后的 git 命令使用；
	// 其余地方（git 服务的 argv 调用）用的是完整路径。
	Git   string `yaml:"git"`
	Shell string `yaml:"shell"`
	HTTP  string `yaml:"http"`
}

// Users 是容器内服务账号配置。
type Users struct {
	// WWW 是应用进程（php-fpm / Go 后端）的运行账号。
	WWW string `yaml:"www"`
	// Git 是 git 账号，SSH 登录用的就是它。
	//
	// 它的家目录 /home/<users.git> 下放着 .ssh/authorized_keys ——
	// 那是 SSH 公钥的落地文件，由 internal/authkeys 维护
	// （PHP 时代由 misc/modify_authorized_keys.php 守护进程维护）。
	// 因此 users.git 决定了公钥文件的路径与归属。
	Git string `yaml:"git"`
}

// Gateway 是与 Go 网关、git 钩子之间的内部契约配置。
type Gateway struct {
	Token    string `yaml:"token"`
	Hooks    string `yaml:"hooks"`
	AuthHTTP string `yaml:"authHTTP"`
	AuthSSH  string `yaml:"authSSH"`
}

// Site 是 config.yaml 的结构。
type Site struct {
	Host          string `yaml:"host"`
	SSH           string `yaml:"ssh"`
	AllowRegister bool   `yaml:"allowRegister"`
	Email         Email  `yaml:"email"`
}

// Email 是发件人配置。
type Email struct {
	Name string `yaml:"name"`
	From string `yaml:"from"`
}

// Config 是运行时配置聚合。
type Config struct {
	Env  Env
	Site Site

	// BaseDir 是配置文件所在目录，所有相对路径都以此为基准解析。
	BaseDir string
}

// Load 加载 env.yaml 与 config.yaml。
//
// baseDir 为配置文件所在目录；PHP 侧以 dirname(dirname(www/index.php)) 为基准，
// 新后端以传入的 baseDir 为准（容器内为 /data/www/codefever-community）。
func Load(baseDir string) (*Config, error) {
	cfg := &Config{BaseDir: baseDir}

	env, err := LoadEnv(baseDir)
	if err != nil {
		return nil, err
	}
	cfg.Env = *env

	sitePath := filepath.Join(baseDir, "config.yaml")
	if err := readYAML(sitePath, &cfg.Site); err != nil {
		return nil, fmt.Errorf("加载 %s 失败: %w", sitePath, err)
	}

	cfg.applyDefaults()
	return cfg, nil
}

// LoadEnv 只加载 env.yaml。
//
// 与 Load 的区别：**不要求 config.yaml 存在** —— git 钩子（backend/cmd/codefever-hook）
// 只需要 env.yaml 里的 gateway 段，PHP 侧的 misc/hooks/main 也只读 env.yaml。
// 单独提供这个入口是为了让钩子复用同一套 YAML 解析器与同一份 Env 结构，
// 而不是另写一套 YAML 语义。
//
// 返回的 Env 不含 applyDefaults 的缺省值填充（钩子只用 gateway 段，无关）。
func LoadEnv(baseDir string) (*Env, error) {
	var env Env
	envPath := filepath.Join(baseDir, "env.yaml")
	if err := readYAML(envPath, &env); err != nil {
		return nil, fmt.Errorf("加载 %s 失败: %w", envPath, err)
	}
	return &env, nil
}

// applyDefaults 补齐模板中的占位默认值，避免因配置缺项导致运行时崩溃。
func (c *Config) applyDefaults() {
	if c.Env.MySQL.Port == 0 {
		c.Env.MySQL.Port = 3306
	}
	if c.Env.MySQL.DB == "" {
		c.Env.MySQL.DB = "codefever_community"
	}
	if c.Env.Session.Name == "" {
		c.Env.Session.Name = "codefever_community"
	}
	if c.Env.Path.File == "" {
		c.Env.Path.File = "file-storage"
	}
	if c.Env.Path.Git == "" {
		c.Env.Path.Git = "git-storage"
	}
	if c.Env.Auth.HashFormat == "" {
		c.Env.Auth.HashFormat = string(auth.HashFormatLegacy)
	}
	if c.Env.CLI.Git == "" {
		c.Env.CLI.Git = "git"
	}
	if c.Site.Host == "" {
		c.Site.Host = "http://localhost"
	}

	// 环境变量覆盖：仅用于本地开发与测试，生产仍以 env.yaml 为准。
	// 键名刻意与 env.yaml 的层级对应，便于理解。
	overrideString(&c.Env.MySQL.Host, "CODEFEVER_MYSQL_HOST")
	overrideString(&c.Env.MySQL.Username, "CODEFEVER_MYSQL_USER")
	overrideString(&c.Env.MySQL.Password, "CODEFEVER_MYSQL_PASSWORD")
	overrideString(&c.Env.MySQL.DB, "CODEFEVER_MYSQL_DB")
	overrideString(&c.Env.Auth.HashFormat, "CODEFEVER_PASSWORD_HASH")
	if v := os.Getenv("CODEFEVER_MYSQL_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Env.MySQL.Port = n
		}
	}
}

// overrideString 用环境变量覆盖非空配置项。
func overrideString(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

// Resolve 把配置中的相对路径解析为绝对路径。
func (c *Config) Resolve(rel string) string {
	if rel == "" {
		return c.BaseDir
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(c.BaseDir, rel)
}

// GitStoragePath 返回裸仓库根目录的绝对路径。
func (c *Config) GitStoragePath() string { return c.Resolve(c.Env.Path.Git) }

// FileStoragePath 返回附件存储根目录的绝对路径。
func (c *Config) FileStoragePath() string { return c.Resolve(c.Env.Path.File) }

// DSN 构造 MySQL 连接串。
//
// parseTime=true 让 DATETIME/TIMESTAMP 列直接扫描进 time.Time；
// 多语句关闭以降低 SQL 注入面。
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=false",
		c.Env.MySQL.Username,
		c.Env.MySQL.Password,
		c.Env.MySQL.Host,
		c.Env.MySQL.Port,
		c.Env.MySQL.DB,
	)
}

// IsProduction 判断是否生产环境。
func (c *Config) IsProduction() bool {
	return !strings.EqualFold(c.Env.Env, "development")
}

// PasswordHashFormat 返回口令哈希的写入格式。
//
// 默认 legacy（PHP 侧同款 md5(md5(x))）。在 PHP 与 Go 并存的回滚窗口内
// 必须保持 legacy：PHP 侧只会用 md5(md5($password)) 校验口令，Go 一旦
// 写入 argon2id 哈希，该账号就无法在 PHP 侧登录，回滚目标随之失效。
// 待 PHP 下线后再切换为 argon2id。
func (c *Config) PasswordHashFormat() auth.HashFormat {
	return auth.ParseHashFormat(c.Env.Auth.HashFormat)
}

// readYAML 读取并解析 YAML 文件。
func readYAML(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}
