package gitsvc

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"codefever-next/backend/internal/domain"
)

// ---------------------------------------------------------------------------
// 输出协议
// ---------------------------------------------------------------------------

const (
	// recordSep 是记录分隔符（RS，0x1E）。
	//
	// PHP 侧用 Helper::getDelimiter()（\ue000\ue001\ue000）做记录分隔，
	// 并把 JSON 编码进 --pretty。这里改用 git 原生的 --format，
	// 记录以 RS 开头、字段以 US 分隔 —— 语义等价，但不再需要手工拼 JSON。
	recordSep = "\x1e"
	// fieldSep 是字段分隔符（US，0x1F）。
	//
	// 刻意**不用 NUL**：Windows 的 CreateProcess 不接受命令行里的 NUL 字节，
	// 带 NUL 的 --format 会直接失败（ERROR_INVALID_PARAMETER）。
	// 0x1F 在提交信息里同样几乎不可能出现。
	fieldSep = "\x1f"
)

const (
	// fmtCommit 是提交信息的字段序列：
	// SHA / 标题(%s) / 正文(%b) / 作者邮箱(%ae) / 作者时间戳(%at)。
	//
	// 对应 PHP 侧 GitCommand::getCommitList() 的 pattern
	// ['sha'=>'%H','log'=>'%s','body'=>'%b','email'=>'%ae','time'=>'%at']。
	fmtCommit = recordSep + "%H" + fieldSep + "%s" + fieldSep + "%b" + fieldSep + "%ae" + fieldSep + "%at"

	// fmtCommitDetail 在提交字段之后追加父提交列表(%P，空格分隔)。
	//
	// 对应 PHP 侧 GitCommand::getCommitLog() 的 pattern
	// ['log'=>'%s','time'=>'%at','sha'=>'%H','email'=>'%ae','parent'=>'%P']。
	fmtCommitDetail = recordSep + "%H" + fieldSep + "%s" + fieldSep + "%b" + fieldSep + "%ae" +
		fieldSep + "%at" + fieldSep + "%P"

	// fmtTagCommit 是标签详情的字段序列，两端各有一个 RS，
	// 以便切出 git show 在提交信息之前打印的标签头
	// （对应 PHP 侧 GitCommand::showTagCommit() 的 DEL + JSON + DEL 结构）。
	fmtTagCommit = recordSep + "%H" + fieldSep + "%s" + fieldSep + "%ae" + fieldSep + "%at" + recordSep
)

// phpValue 复刻 PHP 侧 Repository_model::_logStringToArray() 对取值的规约。
//
// PHP 先把 CR/LF/TAB 换成两字符的转义序列，再把其余控制字符换成空格，
// 最后 json_decode 回来。因此 PHP 的正文里换行是**字面的 \n 两个字符**，
// 而不是真正的换行 —— 这里原样复刻，保证响应体逐字一致。
//
// 反斜杠与双引号无需处理：PHP 的 json_decode 会把它们还原成原字符，
// Go 侧保留原值即可，编码后的 JSON 形态相同。
func phpValue(s string) string {
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		// 对应 PHP 的 preg_replace('/[\x01-\x1f\x7f]/', ' ', $json)。
		if (c >= 0x01 && c <= 0x1f) || c == 0x7f {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// parseFormatRecords 解析 --format 的输出，返回每条记录的字段列表。
//
// 记录以 RS 开头、字段以 NUL 分隔，git 在每条记录之后补一个换行。
func parseFormatRecords(out string, fields int) [][]string {
	chunks := strings.Split(out, recordSep)
	records := make([][]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == "" {
			continue
		}
		chunk = strings.TrimSuffix(chunk, "\n")
		if chunk == "" {
			continue
		}
		parts := strings.Split(chunk, fieldSep)
		for len(parts) < fields {
			parts = append(parts, "")
		}
		records = append(records, parts[:fields])
	}
	return records
}

// commitFromFields 把 --format 的字段转成提交摘要。
func commitFromFields(f []string) domain.Commit {
	return domain.Commit{
		SHA:    phpValue(f[0]),
		Commit: phpValue(f[1]),
		Detail: phpValue(f[2]),
		Email:  phpValue(f[3]),
		Time:   parseInt64(f[4]),
	}
}

// parseInt64 解析整数，失败时返回 0（PHP 的 (int) 转换同样返回 0）。
func parseInt64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ---------------------------------------------------------------------------
// 提交
// ---------------------------------------------------------------------------

// commitListArgs 构造 git log 的 argv。
//
// 单独拆出来是为了能直接断言"用户输入只占一个 argv 元素"，
// 从而证明命令注入被阻断（见 service_test.go）。
func commitListArgs(revision, path, keyword string, page, perPage int) []string {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	start := (page - 1) * perPage

	args := []string{"log", "--format=" + fmtCommit}
	if keyword != "" {
		// 关键词是一个独立的 argv 元素：即使含 ; | & 等元字符，
		// 也只会被 git 当成正则模式，不会被 shell 解释。
		args = append(args, "--grep="+keyword)
	}
	args = append(args, "--skip", strconv.Itoa(start), "-"+strconv.Itoa(perPage))
	args = append(args, revision)
	if path != "" {
		// 路径类参数前置 --，避免被 git 当成选项。
		args = append(args, "--", path)
	}
	return args
}

// Commits 列出提交。
//
// 对应 PHP 侧 GitCommand::getCommitList()
// （application/libraries/service/Git/Command.php:193-227）
// 与 Repository_model::getCommits()
// （application/models/repository_model.php:1602-1633）。
//
// 关键词通过 `--grep=<keyword>` 作为**单个 argv 元素**传给 git。
// PHP 侧把它原样拼进 shell 字符串（`--grep=$keyword`），是命令注入面；
// 这里既保留了 git 的正则语义，又切断了注入路径。
func (s *Service) Commits(ctx context.Context, repoPath, revision, path, keyword string, page, perPage int) ([]domain.Commit, error) {
	if revision == "" {
		return nil, fmt.Errorf("revision 不能为空")
	}
	if err := rejectOptionLike(revision); err != nil {
		return nil, err
	}

	out, err := s.run(ctx, repoPath, commitListArgs(revision, path, keyword, page, perPage)...)
	if err != nil {
		return nil, err
	}
	records := parseFormatRecords(out, 5)
	commits := make([]domain.Commit, 0, len(records))
	for _, r := range records {
		commits = append(commits, commitFromFields(r))
	}
	return commits, nil
}

// CommitDetail 读取一次提交的详情（含父提交列表）。
//
// 对应 PHP 侧 GitCommand::getCommitLog()
// （application/libraries/service/Git/Command.php:276-293）
// 与 Repository_model::getCommitDetail()
// （application/models/repository_model.php:1728-1744）。
func (s *Service) CommitDetail(ctx context.Context, repoPath, sha string) (*domain.CommitDetail, error) {
	if sha == "" {
		return nil, fmt.Errorf("commitSHA 不能为空")
	}
	if err := rejectOptionLike(sha); err != nil {
		return nil, err
	}

	out, err := s.run(ctx, repoPath, "log", "--format="+fmtCommitDetail, "-1", sha)
	if err != nil {
		return nil, err
	}
	records := parseFormatRecords(out, 6)
	if len(records) == 0 {
		return nil, fmt.Errorf("提交不存在: %s", sha)
	}
	f := records[0]
	return &domain.CommitDetail{
		SHA:    phpValue(f[0]),
		Commit: phpValue(f[1]),
		Email:  phpValue(f[3]),
		Time:   parseInt64(f[4]),
		Parent: phpValue(f[5]),
	}, nil
}

// LastCommit 读取某个引用（可选地限定到某条路径）的最后一次提交。
//
// 对应 PHP 侧 GitCommand::getLastLog()
// （application/libraries/service/Git/Command.php:158-176）
// 与 Repository_model::getLastCommit()
// （application/models/repository_model.php:1470-1489）。
//
// lastSHA 是可选参数（PHP 侧的 $lastSha 默认空）：
// 长度大于 4 时用它替换引用，否则用 ref。
func (s *Service) LastCommit(ctx context.Context, repoPath, ref, path string, lastSHA ...string) (*domain.Commit, error) {
	if ref == "" {
		return nil, fmt.Errorf("ref 不能为空")
	}
	if err := rejectOptionLike(ref); err != nil {
		return nil, err
	}
	target := ref
	if len(lastSHA) > 0 && len(lastSHA[0]) > 4 {
		target = lastSHA[0]
	}

	args := []string{"log", "-1", "--left-only", "--format=" + fmtCommit, target}
	if path != "" {
		args = append(args, "--", path)
	}

	out, err := s.run(ctx, repoPath, args...)
	if err != nil {
		return nil, err
	}
	records := parseFormatRecords(out, 5)
	if len(records) == 0 {
		return nil, fmt.Errorf("引用没有提交: %s", ref)
	}
	c := commitFromFields(records[0])
	return &c, nil
}

// CommitListFromLocal 列出两个提交之间的提交（本地仓库）。
//
// 对应 PHP 侧 GitCommand::getCommitListFromLocal()
// （application/libraries/service/Git/Command.php:178-191）
// 与 Repository_model::getCommitWithLocal()
// （application/models/repository_model.php:1885-1913）。
func (s *Service) CommitListFromLocal(ctx context.Context, repoPath, source, target string) ([]domain.Commit, error) {
	if source == "" || target == "" {
		return nil, fmt.Errorf("提交范围不能为空")
	}
	if err := rejectOptionLike(source); err != nil {
		return nil, err
	}
	if err := rejectOptionLike(target); err != nil {
		return nil, err
	}

	out, err := s.run(ctx, repoPath, "log", "--cherry-pick", "--right-only",
		"--format="+fmtCommit, source+"..."+target)
	if err != nil {
		return nil, err
	}
	records := parseFormatRecords(out, 5)
	commits := make([]domain.Commit, 0, len(records))
	for _, r := range records {
		commits = append(commits, commitFromFields(r))
	}
	return commits, nil
}

// CommitInfoByHash 按提交哈希读取单条提交信息。
//
// 对应 PHP 侧 GitCommand::getCommitInfoByHash()
// （application/libraries/service/Git/Command.php:367-384）。
func (s *Service) CommitInfoByHash(ctx context.Context, repoPath, hash string) (*domain.Commit, error) {
	if hash == "" {
		return nil, fmt.Errorf("提交哈希不能为空")
	}
	if err := rejectOptionLike(hash); err != nil {
		return nil, err
	}
	out, err := s.run(ctx, repoPath, "log", "--format="+fmtCommit, "-1", hash)
	if err != nil {
		return nil, err
	}
	records := parseFormatRecords(out, 5)
	if len(records) == 0 {
		return nil, fmt.Errorf("提交不存在: %s", hash)
	}
	c := commitFromFields(records[0])
	return &c, nil
}

// RevisionRange 列出 start..end 区间内的提交。
//
// 对应 PHP 侧 GitCommand::getRevisionRange()
// （application/libraries/service/Git/Command.php:348-365）。
func (s *Service) RevisionRange(ctx context.Context, repoPath, start, end, branch string) ([]domain.Commit, error) {
	if start == "" || end == "" || branch == "" {
		return nil, fmt.Errorf("版本区间参数不完整")
	}
	for _, v := range []string{start, end, branch} {
		if err := rejectOptionLike(v); err != nil {
			return nil, err
		}
	}
	out, err := s.run(ctx, repoPath, "log", "--format="+fmtCommit, start+".."+end, branch)
	if err != nil {
		return nil, err
	}
	records := parseFormatRecords(out, 5)
	commits := make([]domain.Commit, 0, len(records))
	for _, r := range records {
		commits = append(commits, commitFromFields(r))
	}
	return commits, nil
}

// ---------------------------------------------------------------------------
// 引用
// ---------------------------------------------------------------------------

// branchLineRE 对应 PHP 侧 getBranchList() 的正则 '/^\*?\s+(.*)$/m'。
var branchLineRE = regexp.MustCompile(`(?m)^\*?\s+(.*)$`)

// Branches 列出分支名。
//
// 对应 PHP 侧 GitCommand::getBranchList()
// （application/libraries/service/Git/Command.php:257-260）
// 与 Repository_model::getBranchList()
// （application/models/repository_model.php:1537-1559）。
func (s *Service) Branches(ctx context.Context, repoPath string) ([]string, error) {
	out, err := s.run(ctx, repoPath, "branch")
	if err != nil {
		return nil, err
	}
	matches := branchLineRE.FindAllStringSubmatch(out, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names, nil
}

// Tags 列出标签名。
//
// 对应 PHP 侧 GitCommand::getTagList()
// （application/libraries/service/Git/Command.php:75-78）
// 与 Repository_model::getTagList()
// （application/models/repository_model.php:1746-1769）。
func (s *Service) Tags(ctx context.Context, repoPath string) ([]string, error) {
	out, err := s.run(ctx, repoPath, "tag", "-l")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// BranchesContainingSHA 列出包含指定提交的分支。
//
// 对应 PHP 侧 GitCommand::getBranchListBySHA()
// （application/libraries/service/Git/Command.php:321-328）
// 与 Repository_model::getBranchListBySHA1()
// （application/models/repository_model.php:1975-1995）。
func (s *Service) BranchesContainingSHA(ctx context.Context, repoPath, sha string) ([]string, error) {
	if sha == "" {
		return nil, fmt.Errorf("提交哈希不能为空")
	}
	if err := rejectOptionLike(sha); err != nil {
		return nil, err
	}
	out, err := s.run(ctx, repoPath, "branch", "-a", "--contains", sha)
	if err != nil {
		return nil, err
	}
	matches := branchLineRE.FindAllStringSubmatch(out, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names, nil
}

// TagsContainingSHA 列出包含指定提交的标签。
//
// 对应 PHP 侧 GitCommand::getTagListBySHA()
// （application/libraries/service/Git/Command.php:330-337）
// 与 Repository_model::getTagListBySHA1()
// （application/models/repository_model.php:1997-2020）。
func (s *Service) TagsContainingSHA(ctx context.Context, repoPath, sha string) ([]string, error) {
	if sha == "" {
		return nil, fmt.Errorf("提交哈希不能为空")
	}
	if err := rejectOptionLike(sha); err != nil {
		return nil, err
	}
	out, err := s.run(ctx, repoPath, "tag", "--contains", sha)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// CommitCount 统计某个引用下的提交数。
//
// 对应 PHP 侧 GitCommand::getCommitCount()
// （application/libraries/service/Git/Command.php:248-255）
// 与 Repository_model::getCommitCount()
// （application/models/repository_model.php:1504-1519）。
func (s *Service) CommitCount(ctx context.Context, repoPath, branch string) (int, error) {
	if branch == "" {
		return 0, fmt.Errorf("分支不能为空")
	}
	if err := rejectOptionLike(branch); err != nil {
		return 0, err
	}
	out, err := s.run(ctx, repoPath, "rev-list", branch, "--count")
	if err != nil {
		return 0, err
	}
	return int(parseInt64(strings.TrimSpace(out))), nil
}

// BranchCompareCount 统计 base..branch 之间的提交数。
//
// 对应 PHP 侧 GitCommand::getBranchCompare()
// （application/libraries/service/Git/Command.php:267-274）
// 与 Repository_model::isMerged()
// （application/models/repository_model.php:1706-1726）。
func (s *Service) BranchCompareCount(ctx context.Context, repoPath, base, branch string) (int, error) {
	if base == "" || branch == "" {
		return 0, fmt.Errorf("分支不能为空")
	}
	for _, v := range []string{base, branch} {
		if err := rejectOptionLike(v); err != nil {
			return 0, err
		}
	}
	out, err := s.run(ctx, repoPath, "rev-list", base+".."+branch, "--count")
	if err != nil {
		return 0, err
	}
	return int(parseInt64(strings.TrimSpace(out))), nil
}

// ObjectsCount 是 git count-objects -v 的结果。
type ObjectsCount struct {
	Count         int
	Size          int
	InPack        int
	Packs         int
	SizePack      int
	PrunePackable int
	Garbage       int
	SizeGarbage   int
}

// objectsCountRE 对应 PHP 侧 getObjectsCount() 的正则 '/([\w\-]+)\:\s+(\d+)$/m'。
var objectsCountRE = regexp.MustCompile(`(?m)([\w\-]+):\s+(\d+)$`)

// ObjectsCount 统计仓库的松散对象占用。
//
// 对应 PHP 侧 GitCommand::getCountObjects()
// （application/libraries/service/Git/Command.php:262-265）
// 与 Repository_model::getObjectsCount()
// （application/models/repository_model.php:1577-1600）。
func (s *Service) ObjectsCount(ctx context.Context, repoPath string) (*ObjectsCount, error) {
	out, err := s.run(ctx, repoPath, "count-objects", "-v")
	if err != nil {
		return nil, err
	}
	data := &ObjectsCount{}
	for _, m := range objectsCountRE.FindAllStringSubmatch(out, -1) {
		v := int(parseInt64(m[2]))
		switch m[1] {
		case "count":
			data.Count = v
		case "size":
			data.Size = v
		case "in-pack":
			data.InPack = v
		case "packs":
			data.Packs = v
		case "size-pack":
			data.SizePack = v
		case "prune-packable":
			data.PrunePackable = v
		case "garbage":
			data.Garbage = v
		case "size-garbage":
			data.SizeGarbage = v
		}
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// 对象
// ---------------------------------------------------------------------------

// Object 是一个 git 对象的解析结果。
type Object struct {
	// Type 是 commit / tag / tree / blob。
	Type string
	// Entries 是 tree / commit 对象的条目列表；blob 时为空。
	Entries []domain.ObjectEntry
	// Raw 是 blob 对象的原始内容。
	Raw string
	// Size 是 blob 的大小；非 blob 时为 -1。
	Size int
}

// treeLineRE 对应 PHP 侧 Helper::parseTable() 的行正则
// '/^(\d+)\s+(commit|tag|tree|blob)\s+([0-9a-f]{40})\t(.*)$/i'。
var treeLineRE = regexp.MustCompile(`(?i)^(\d+)\s+(commit|tag|tree|blob)\s+([0-9a-f]{40})\t(.*)$`)

// quotedNameRE 对应 PHP 侧 Helper::parseTable() 的引号判定 '/^"(.*)"$/i'。
var quotedNameRE = regexp.MustCompile(`(?i)^"(.*)"$`)

// Object 读取一个 git 对象。
//
// 对应 PHP 侧 Repository_model::catObject()
// （application/models/repository_model.php:1371-1413）：
// 先 cat-file -t 判类型，commit/tag 追加 ^{tree}，blob 先查大小，
// 再 cat-file -p 取内容；tree 的内容按 ls-tree 的行格式解析。
func (s *Service) Object(ctx context.Context, repoPath, parent string) (*Object, error) {
	if parent == "" {
		return nil, fmt.Errorf("对象不能为空")
	}
	if err := rejectOptionLike(parent); err != nil {
		return nil, err
	}

	objectType, err := s.ObjectType(ctx, repoPath, parent)
	if err != nil {
		return nil, err
	}
	if objectType == "" {
		return nil, fmt.Errorf("对象不存在: %s", parent)
	}

	target := parent
	if objectType == "commit" || objectType == "tag" {
		target = parent + "^{tree}"
	}

	size := -1
	if objectType == "blob" {
		size, err = s.ObjectSize(ctx, repoPath, target)
		if err != nil {
			return nil, err
		}
	}

	out, err := s.run(ctx, repoPath, "cat-file", "-p", target)
	if err != nil {
		return nil, err
	}
	if objectType == "blob" {
		return &Object{Type: objectType, Raw: out, Size: size}, nil
	}
	return &Object{Type: objectType, Entries: parseTreeEntries(out), Size: -1}, nil
}

// ObjectType 返回对象类型（commit / tag / tree / blob）。
//
// 对应 PHP 侧 GitCommand::getObjectType()
// （application/libraries/service/Git/Command.php:140-147）。
func (s *Service) ObjectType(ctx context.Context, repoPath, object string) (string, error) {
	if object == "" {
		return "", fmt.Errorf("对象不能为空")
	}
	if err := rejectOptionLike(object); err != nil {
		return "", err
	}
	out, err := s.run(ctx, repoPath, "cat-file", "-t", object)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ObjectSize 返回对象大小（字节）。
//
// 对应 PHP 侧 GitCommand::getObjectSize()
// （application/libraries/service/Git/Command.php:149-156）。
func (s *Service) ObjectSize(ctx context.Context, repoPath, object string) (int, error) {
	if object == "" {
		return 0, fmt.Errorf("对象不能为空")
	}
	if err := rejectOptionLike(object); err != nil {
		return 0, err
	}
	out, err := s.run(ctx, repoPath, "cat-file", "-s", object)
	if err != nil {
		return 0, err
	}
	return int(parseInt64(strings.TrimSpace(out))), nil
}

// parseTreeEntries 解析 cat-file -p 的 tree 输出。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:92-110
// 的 parseTable()。
func parseTreeEntries(out string) []domain.ObjectEntry {
	entries := make([]domain.ObjectEntry, 0)
	for _, line := range strings.Split(out, "\n") {
		m := treeLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[4]
		if q := quotedNameRE.FindStringSubmatch(name); q != nil {
			name = parseEscapedSequences(q[1])
		}
		entries = append(entries, domain.ObjectEntry{
			Mode:   m[1],
			Type:   m[2],
			Object: m[3],
			Name:   name,
		})
	}
	return entries
}

// octalEscapeRE / hexEscapeRE 对应 PHP 侧 Helper::parseEscapedSequences()
// （application/libraries/service/Utility/Helper.php:112-135）的两个正则。
//
// 注意 PHP 的原正则要求**连续 3 个**转义序列才匹配，
// 因此 git 对双字节字符打出的 "\303\251" 不会被还原 —— 这里原样复刻该行为。
var (
	octalEscapeRE = regexp.MustCompile(`(\\[0-7][0-7][0-7]){3}`)
	hexEscapeRE   = regexp.MustCompile(`(?i)(\\x[0-9a-f][0-9a-f]){3}`)
	singleOctalRE = regexp.MustCompile(`\\[0-7][0-7][0-7]`)
	singleHexRE   = regexp.MustCompile(`(?i)\\x[0-9a-f][0-9a-f]`)
)

// parseEscapedSequences 还原 git 对特殊字符名做的转义。
func parseEscapedSequences(in string) string {
	in = octalEscapeRE.ReplaceAllStringFunc(in, func(m string) string {
		var b strings.Builder
		for _, part := range singleOctalRE.FindAllString(m, -1) {
			n, err := strconv.ParseUint(part[1:], 8, 16)
			if err != nil {
				continue
			}
			b.WriteByte(byte(n))
		}
		return b.String()
	})
	in = hexEscapeRE.ReplaceAllStringFunc(in, func(m string) string {
		var b strings.Builder
		for _, part := range singleHexRE.FindAllString(m, -1) {
			n, err := strconv.ParseUint(part[2:], 16, 8)
			if err != nil {
				continue
			}
			b.WriteByte(byte(n))
		}
		return b.String()
	})
	return in
}

// ---------------------------------------------------------------------------
// 标签
// ---------------------------------------------------------------------------

// TagDetail 是标签的详情。
type TagDetail struct {
	// Detail 是标签说明（git show 在提交信息之前打印的头部，去掉前 3 行）。
	Detail string
	// Commit 是标签指向的提交。
	Commit domain.Commit
}

// TagDetail 读取标签详情。
//
// 对应 PHP 侧 GitCommand::showTagCommit()
// （application/libraries/service/Git/Command.php:34-50）
// 与 Repository_model::getTagDetail()
// （application/models/repository_model.php:1790-1821）。
func (s *Service) TagDetail(ctx context.Context, repoPath, tag string) (*TagDetail, error) {
	if tag == "" {
		return nil, fmt.Errorf("标签不能为空")
	}
	if err := rejectOptionLike(tag); err != nil {
		return nil, err
	}

	out, err := s.run(ctx, repoPath, "show", tag, "--format="+fmtTagCommit, "--no-patch")
	if err != nil {
		return nil, err
	}

	first := strings.Index(out, recordSep)
	last := strings.LastIndex(out, recordSep)
	if first < 0 || last <= first {
		return nil, fmt.Errorf("标签详情解析失败: %s", tag)
	}

	result := &TagDetail{}
	if header := out[:first]; header != "" {
		// PHP 侧 array_slice($messages, 3)：跳过 "tag <name>" / "Tagger:" / "Date:" 三行。
		lines := strings.Split(header, "\n")
		if len(lines) > 3 {
			result.Detail = strings.Join(lines[3:], " ")
		}
	}

	fields := strings.Split(strings.TrimSuffix(out[first+len(recordSep):last], "\n"), fieldSep)
	for len(fields) < 4 {
		fields = append(fields, "")
	}
	result.Commit = domain.Commit{
		SHA:    phpValue(fields[0]),
		Commit: phpValue(fields[1]),
		Email:  phpValue(fields[2]),
		Time:   parseInt64(fields[3]),
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// 差异
// ---------------------------------------------------------------------------

// Diff 读取两个提交之间的差异文本。
//
// 对应 PHP 侧 GitCommand::getDiffDetail()
// （application/libraries/service/Git/Command.php:308-319）
// 与 Repository_model::getDiffWithLocal()
// （application/models/repository_model.php:1965-1973）。
//
// original 为空或全 0 时，PHP 用 `show <modified>` 展示整个提交；
// 否则用 `diff <original>...<modified>`。
func (s *Service) Diff(ctx context.Context, repoPath, original, modified string) (string, error) {
	if modified == "" {
		return "", fmt.Errorf("modified 不能为空")
	}
	for _, v := range []string{original, modified} {
		if err := rejectOptionLike(v); err != nil {
			return "", err
		}
	}
	if isZeroSHA(original) {
		out, err := s.run(ctx, repoPath, "show", modified)
		if err != nil {
			return "", err
		}
		return out, nil
	}
	out, err := s.run(ctx, repoPath, "diff", original+"..."+modified)
	if err != nil {
		return "", err
	}
	return out, nil
}

// DiffFileCount 统计两个提交之间变动的文件数。
//
// 对应 PHP 侧 GitCommand::getDiffCount()
// （application/libraries/service/Git/Command.php:295-306）
// 与 Repository_model::getDiffWithLocalCount()
// （application/models/repository_model.php:1950-1963）。
//
// PHP 侧在 original 为全 0 时会把多出来的两行（提交哈希与空行）扣掉。
func (s *Service) DiffFileCount(ctx context.Context, repoPath, original, modified string) (int, error) {
	if modified == "" {
		return 0, fmt.Errorf("modified 不能为空")
	}
	for _, v := range []string{original, modified} {
		if err := rejectOptionLike(v); err != nil {
			return 0, err
		}
	}

	var out string
	var err error
	if isZeroSHA(original) {
		out, err = s.run(ctx, repoPath, "show", modified, "--name-only", "--pretty=%H")
	} else {
		out, err = s.run(ctx, repoPath, "diff", original+"..."+modified, "--name-only")
	}
	if err != nil {
		return 0, err
	}

	trimmed := strings.TrimRight(out, "\n")
	if trimmed == "" {
		return 0, nil
	}
	count := len(strings.Split(trimmed, "\n"))
	if isZeroSHA(original) {
		count -= 2
	}
	if count < 0 {
		count = 0
	}
	return count, nil
}

// isZeroSHA 判断是否为全 0 的 SHA（PHP 侧的 preg_match('/^0+$/i')）。
func isZeroSHA(sha string) bool {
	if sha == "" {
		return true
	}
	for i := 0; i < len(sha); i++ {
		if sha[i] != '0' {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// blame
// ---------------------------------------------------------------------------

// blameHeaderRE 对应 PHP 侧 Helper::parseBlameData() 的头部正则
// '/^([0-9a-f]{40})\s+(\d+)\s+(\d+)\s+(\d+)$/'。
var blameHeaderRE = regexp.MustCompile(`^([0-9a-f]{40})\s+(\d+)\s+(\d+)\s+(\d+)$`)

var (
	blameSummaryRE = regexp.MustCompile(`^summary (.+)$`)
	blameMailRE    = regexp.MustCompile(`^committer-mail <(.+)>$`)
	blameTimeRE    = regexp.MustCompile(`^committer-time (\d+)$`)
	blameLineRE    = regexp.MustCompile(`^\t(.*)$`)
)

// Blame 读取文件的逐行归属信息。
//
// 对应 PHP 侧 Repository_model::getBlameInfo()
// （application/models/repository_model.php:1415-1459）。
//
// PHP 侧的做法是 clone 一份工作区再 `git checkout` + `git blame -p`；
// 裸仓库上直接 `git blame -p <rev> -- <path>` 得到的结果完全一致，
// 因此这里省掉了 clone/checkout 两步。
func (s *Service) Blame(ctx context.Context, repoPath, revision, path string) (*domain.BlameResult, error) {
	if revision == "" {
		return nil, fmt.Errorf("revision 不能为空")
	}
	if err := rejectOptionLike(revision); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("path 不能为空")
	}

	args := []string{"blame", "-p", revision, "--", path}
	out, err := s.run(ctx, repoPath, args...)
	if err != nil {
		return nil, err
	}
	return parseBlame(out), nil
}

// parseBlame 解析 git blame -p 的输出。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:225-274
// 的 parseBlameData()。
func parseBlame(out string) *domain.BlameResult {
	result := &domain.BlameResult{Blame: []domain.BlameBlock{}, Lines: []string{}}

	var pending *domain.BlameBlock
	for _, line := range strings.Split(out, "\n") {
		switch {
		case blameHeaderRE.MatchString(line):
			m := blameHeaderRE.FindStringSubmatch(line)
			pending = &domain.BlameBlock{
				Start:  int(parseInt64(m[3])),
				Length: int(parseInt64(m[4])),
				Commit: domain.Commit{SHA: m[1]},
			}
		case blameSummaryRE.MatchString(line):
			if pending != nil {
				pending.Commit.Commit = blameSummaryRE.FindStringSubmatch(line)[1]
			}
		case blameMailRE.MatchString(line):
			if pending != nil {
				pending.Commit.Email = blameMailRE.FindStringSubmatch(line)[1]
			}
		case blameTimeRE.MatchString(line):
			if pending != nil {
				pending.Commit.Time = parseInt64(blameTimeRE.FindStringSubmatch(line)[1])
			}
		case blameLineRE.MatchString(line):
			if pending != nil && pending.Commit.SHA != "" {
				// PHP 侧对同一 SHA 复用首次出现的提交信息。
				for _, b := range result.Blame {
					if b.Commit.SHA == pending.Commit.SHA {
						pending.Commit = b.Commit
						break
					}
				}
				result.Blame = append(result.Blame, *pending)
				pending = nil
			}
			result.Lines = append(result.Lines, blameLineRE.FindStringSubmatch(line)[1])
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 差异解析
// ---------------------------------------------------------------------------

// 逐字取自 application/config/constants.php。
const (
	// maxDisplayDiffLine 是单文件 diff 允许的最大行数。
	maxDisplayDiffLine = 500
	// maxDisplayDiffLength 是单文件 diff 允许的最大字节数（8K）。
	maxDisplayDiffLength = 0x2000
)

var (
	diffHeaderRE   = regexp.MustCompile(`^diff\s+--git\s+(")?a/(.*)\s+(")?b/(.*)`)
	diffIndexRE    = regexp.MustCompile(`^index\s+([0-9a-f]+)\.+([0-9a-f]+)(?:\s+(\d+))?`)
	diffOldMode    = regexp.MustCompile(`^old\s+mode\s+(\d+)`)
	diffNewMode    = regexp.MustCompile(`^new\s+mode\s+(\d+)`)
	diffDelMode    = regexp.MustCompile(`^deleted\s+file\s+mode\s+(\d+)`)
	diffAddMode    = regexp.MustCompile(`^new\s+file\s+mode\s+(\d+)`)
	diffBinaryRE   = regexp.MustCompile(`^Binary\s+files\s(.*)\sand\s(.*)\sdiffer`)
	diffRenameFrom = regexp.MustCompile(`^rename\s+from\s+(")?(.+)`)
	diffRenameTo   = regexp.MustCompile(`^rename\s+to\s+(")?(.+)`)
	diffHunkRE     = regexp.MustCompile(`^@@\s-+\d+(,\d+)?\s+\++\d+(,\d+)?\s+@@.*$`)
	diffLineRE     = regexp.MustCompile(`^[\s\+-].*$`)
)

// ParseDiff 解析 git 的 diff 文本。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:137-223
// 的 parseDiffString()。
//
// 几处"看起来像 bug 但必须保留"的行为：
//   - deletions 用的是 `--` 自减，所以删除行会让 deletions 变成**负数**；
//   - 只有在已经出现 @@ 块头之后，后续行才会被计入 diff.line；
//   - 超出阈值时 type 变成 oversize，raw 置空、line 清空。
func ParseDiff(diff string) []domain.DiffFile {
	result := []domain.DiffFile{}
	var item *domain.DiffFile

	flush := func() {
		if item == nil {
			return
		}
		raw := strings.Join(item.Diff.Line, "\n")
		if len(item.Diff.Line) > maxDisplayDiffLine || len(raw) > maxDisplayDiffLength {
			item.Type = "oversize"
			item.Diff.Raw = nil
			item.Diff.Line = []string{}
		} else {
			rawCopy := raw
			item.Diff.Raw = &rawCopy
		}
		result = append(result, *item)
		item = nil
	}

	for _, line := range strings.Split(diff, "\n") {
		if m := diffHeaderRE.FindStringSubmatch(line); m != nil {
			flush()
			originalName := m[2]
			if m[1] != "" {
				originalName = parseEscapedSequences(strings.TrimRight(originalName, "\""))
			}
			modifiedName := m[4]
			if m[3] != "" {
				modifiedName = parseEscapedSequences(strings.TrimRight(modifiedName, "\""))
			}
			item = &domain.DiffFile{
				Original: domain.DiffSide{Name: &originalName},
				Modified: domain.DiffSide{Name: &modifiedName},
				Diff:     domain.DiffBody{Line: []string{}},
				Type:     "text",
			}
		}
		if item == nil {
			continue
		}

		switch {
		case diffIndexRE.MatchString(line):
			m := diffIndexRE.FindStringSubmatch(line)
			sha1, sha2 := m[1], m[2]
			item.Original.SHA = &sha1
			item.Modified.SHA = &sha2
			if m[3] != "" {
				mode := m[3]
				item.Original.Mode = &mode
				item.Modified.Mode = &mode
			}
		case diffOldMode.MatchString(line):
			mode := diffOldMode.FindStringSubmatch(line)[1]
			item.Original.Mode = &mode
		case diffNewMode.MatchString(line):
			mode := diffNewMode.FindStringSubmatch(line)[1]
			item.Modified.Mode = &mode
		case diffDelMode.MatchString(line):
			item.Modified.Name = nil
			mode := diffDelMode.FindStringSubmatch(line)[1]
			item.Original.Mode = &mode
		case diffAddMode.MatchString(line):
			item.Original.Name = nil
			mode := diffAddMode.FindStringSubmatch(line)[1]
			item.Modified.Mode = &mode
		case diffBinaryRE.MatchString(line):
			item.Type = "binary"
		case diffRenameFrom.MatchString(line):
			m := diffRenameFrom.FindStringSubmatch(line)
			name := m[2]
			if m[1] != "" {
				name = parseEscapedSequences(strings.TrimRight(name, "\""))
			}
			item.Original.Name = &name
		case diffRenameTo.MatchString(line):
			m := diffRenameTo.FindStringSubmatch(line)
			name := m[2]
			if m[1] != "" {
				name = parseEscapedSequences(strings.TrimRight(name, "\""))
			}
			item.Modified.Name = &name
		case diffHunkRE.MatchString(line):
			item.Diff.Line = append(item.Diff.Line, line)
		case len(item.Diff.Line) > 0 && diffLineRE.MatchString(line):
			item.Diff.Line = append(item.Diff.Line, line)
			switch line[0] {
			case '+':
				item.Diff.Additions++
			case '-':
				// PHP 侧写的是 `$deletions--`，因此这里是自减，删除行会得到负数。
				item.Diff.Deletions--
			}
		}
	}
	flush()
	return result
}

// ---------------------------------------------------------------------------
// 工具
// ---------------------------------------------------------------------------

// rejectOptionLike 拒绝以 "-" 开头的修订号，避免被 git 当成选项。
//
// 用户输入永远不会经过 shell（见 run），但 git 自身仍会把 "-xxx"
// 解释成选项，因此这里显式挡一道。
func rejectOptionLike(v string) error {
	if strings.HasPrefix(v, "-") {
		return fmt.Errorf("参数非法: %q", v)
	}
	return nil
}
