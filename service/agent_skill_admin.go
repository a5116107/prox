package service

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type agentSkillRepoRef struct {
	CanonicalURL string `json:"canonical_url"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Branch       string `json:"branch"`
}

type agentSkillCatalogItem struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	HasSkill  bool   `json:"has_skill"`
	HasPlugin bool   `json:"has_plugin"`
}

type agentSkillRegistryEntry struct {
	Name          string                  `json:"name"`
	RepoURL       string                  `json:"repo_url"`
	Owner         string                  `json:"owner"`
	Repo          string                  `json:"repo"`
	Branch        string                  `json:"branch"`
	InstalledAt   int64                   `json:"installed_at"`
	InstalledPath string                  `json:"installed_path"`
	SkillCount    int                     `json:"skill_count"`
	Items         []agentSkillCatalogItem `json:"items"`
}

type agentSkillRegistry struct {
	UpdatedAt int64                     `json:"updated_at"`
	Entries   []agentSkillRegistryEntry `json:"entries"`
}

var (
	agentSkillRepoURLRe     = regexp.MustCompile(`https?://github\.com/[^\s]+`)
	agentSkillBranchParamRe = regexp.MustCompile(`(?i)(?:branch|ref|分支)\s*[:=： ]\s*([A-Za-z0-9._/\-]+)`)
	agentSkillNameParamRe   = regexp.MustCompile(`(?i)(?:skill[_\s-]*name|plugin[_\s-]*name|技能名|插件名)\s*[:=： ]\s*([A-Za-z0-9._/\-]+)`)
	agentSkillUnsafeNameRe  = regexp.MustCompile(`[^A-Za-z0-9._/\-]+`)
)

func isAgentSkillInstallCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	if lower == "" {
		return false
	}
	if !strings.Contains(lower, "github.com/") {
		return false
	}
	return hasAny(lower, "安装", "install", "skill", "skills", "技能", "插件", "github", "仓库")
}

func isAgentSkillListCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	if lower == "" {
		return false
	}
	if isAgentSkillInstallCommand(lower) {
		return false
	}
	compact := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "").Replace(lower)
	return hasAny(lower,
		"skill list", "skills list", "list skills",
		"查看skill", "查看skills", "看看skill", "看看skills", "看看已装skill", "看看已装skills") || hasAny(compact,
		"skill列表", "skills列表", "skill list", "skills list", "list skills",
		"查看skill", "查看skills", "看看skill", "看看skills", "看看已装哪些skill", "看看已装哪些skills",
		"已装skill", "已装skills", "已装哪些skill", "已装哪些skills", "装了哪些skill", "装了哪些skills", "装了哪些技能",
		"已安装skill", "已安装skills", "已安装技能", "现在装了哪些skill", "现在装了哪些技能")
}

func extractAgentSkillInstallRequest(command string) (repoURL, branch, skillName string) {
	text := strings.TrimSpace(command)
	if text == "" {
		return "", "", ""
	}
	repoURL = strings.TrimSpace(agentSkillRepoURLRe.FindString(text))
	repoURL = strings.TrimRight(repoURL, "）)>]】,，。;；'")
	if m := agentSkillBranchParamRe.FindStringSubmatch(text); len(m) > 1 {
		branch = strings.TrimSpace(m[1])
	}
	if m := agentSkillNameParamRe.FindStringSubmatch(text); len(m) > 1 {
		skillName = strings.TrimSpace(m[1])
	}
	skillName = agentSkillUnsafeNameRe.ReplaceAllString(skillName, "-")
	skillName = strings.Trim(skillName, "-./")
	return repoURL, branch, skillName
}

func agentResolveSkillRepoRef(rawURL, branchHint string) (agentSkillRepoRef, error) {
	ref := agentSkillRepoRef{}
	trimmed := strings.TrimSpace(rawURL)
	trimmed = strings.TrimRight(trimmed, "）)>]】,，。;；'")
	if trimmed == "" {
		return ref, errors.New("missing GitHub repo url")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return ref, fmt.Errorf("invalid GitHub repo url: %w", err)
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host != "github.com" && host != "www.github.com" {
		return ref, errors.New("only github.com public skill repos are supported")
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(strings.Trim(u.Path, "/"), "/") {
		if strings.TrimSpace(part) != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) < 2 {
		return ref, errors.New("GitHub repo url must look like https://github.com/<owner>/<repo>")
	}
	ref.Owner = strings.TrimSpace(parts[0])
	ref.Repo = strings.TrimSpace(strings.TrimSuffix(parts[1], ".git"))
	if ref.Owner == "" || ref.Repo == "" {
		return ref, errors.New("GitHub repo url missing owner or repo")
	}
	ref.Branch = strings.TrimSpace(branchHint)
	if ref.Branch == "" && len(parts) >= 4 && strings.EqualFold(parts[2], "tree") {
		ref.Branch = strings.TrimSpace(parts[3])
	}
	if ref.Branch == "" {
		defaultBranch, err := agentDetectGitHubDefaultBranch(ref.Owner, ref.Repo)
		if err == nil && strings.TrimSpace(defaultBranch) != "" {
			ref.Branch = defaultBranch
		}
	}
	if ref.Branch == "" {
		ref.Branch = "main"
	}
	ref.CanonicalURL = fmt.Sprintf("https://github.com/%s/%s", ref.Owner, ref.Repo)
	return ref, nil
}

func executeAgentSkillInstall(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	command := firstAgentNonEmpty(action.Reason, agentPayloadString(payload, "command", "text", "message", "natural_task"))
	repoURL, branchHint, skillName := extractAgentSkillInstallRequest(command)
	repoURL = firstAgentNonEmpty(agentPayloadString(payload, "repo_url", "url", "repo"), repoURL)
	branchHint = firstAgentNonEmpty(agentPayloadString(payload, "branch", "ref"), branchHint)
	skillName = firstAgentNonEmpty(agentPayloadString(payload, "skill_name", "name"), skillName)
	if repoURL == "" {
		return nil, errors.New("安装 skill 需要 GitHub 仓库链接，例如：安装这个 skill https://github.com/owner/repo branch=main")
	}
	ref, err := agentResolveSkillRepoRef(repoURL, branchHint)
	if err != nil {
		return nil, err
	}
	root := agentSkillStorageRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	archivePath, err := agentDownloadGitHubArchive(ref)
	if err != nil {
		return nil, err
	}
	defer os.Remove(archivePath)
	extractRoot, err := agentExtractGitHubArchive(archivePath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(extractRoot)
	sourceRoot := agentNormalizeArchiveRoot(extractRoot)
	discovered, err := agentDiscoverInstalledSkills(sourceRoot)
	if err != nil {
		return nil, err
	}
	if len(discovered) == 0 {
		return nil, errors.New("仓库里没找到可安装的 skill：至少要有 SKILL.md 或 .codex-plugin/plugin.json")
	}
	destPath := agentRepoInstallDir(ref)
	stagingPath := destPath + ".tmp"
	_ = os.RemoveAll(stagingPath)
	if err := agentCopyDir(sourceRoot, stagingPath); err != nil {
		_ = os.RemoveAll(stagingPath)
		return nil, err
	}
	_ = os.RemoveAll(destPath)
	if err := os.Rename(stagingPath, destPath); err != nil {
		_ = os.RemoveAll(stagingPath)
		return nil, err
	}
	discovered, err = agentDiscoverInstalledSkills(destPath)
	if err != nil {
		return nil, err
	}
	entryName := firstAgentNonEmpty(skillName, fmt.Sprintf("%s/%s", ref.Owner, ref.Repo))
	if len(discovered) == 1 && strings.TrimSpace(skillName) == "" {
		entryName = firstAgentNonEmpty(discovered[0].Name, entryName)
	}
	registry, err := loadAgentSkillRegistry()
	if err != nil {
		return nil, err
	}
	entry := agentSkillRegistryEntry{
		Name:          entryName,
		RepoURL:       ref.CanonicalURL,
		Owner:         ref.Owner,
		Repo:          ref.Repo,
		Branch:        ref.Branch,
		InstalledAt:   time.Now().Unix(),
		InstalledPath: filepath.ToSlash(destPath),
		SkillCount:    len(discovered),
		Items:         discovered,
	}
	replaced := false
	for i := range registry.Entries {
		if strings.EqualFold(registry.Entries[i].RepoURL, entry.RepoURL) && strings.EqualFold(registry.Entries[i].Branch, entry.Branch) {
			registry.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		registry.Entries = append(registry.Entries, entry)
	}
	sort.SliceStable(registry.Entries, func(i, j int) bool {
		if registry.Entries[i].InstalledAt == registry.Entries[j].InstalledAt {
			return registry.Entries[i].Name < registry.Entries[j].Name
		}
		return registry.Entries[i].InstalledAt > registry.Entries[j].InstalledAt
	})
	if err := saveAgentSkillRegistry(registry); err != nil {
		return nil, err
	}
	itemNames := make([]string, 0, len(discovered))
	for _, item := range discovered {
		itemNames = append(itemNames, item.Name)
	}
	summary := fmt.Sprintf("外部 skill 已装好：%s（%s/%s@%s），共识别 %d 个技能目录：%s。",
		entry.Name, ref.Owner, ref.Repo, ref.Branch, len(discovered), strings.Join(itemNames, "、"))
	common.SysLog(fmt.Sprintf("[AgentSkill] installed repo=%s branch=%s count=%d path=%s", ref.CanonicalURL, ref.Branch, len(discovered), destPath))
	return map[string]any{
		"ok":             true,
		"summary":        summary,
		"repo_url":       ref.CanonicalURL,
		"owner":          ref.Owner,
		"repo":           ref.Repo,
		"branch":         ref.Branch,
		"skill_name":     entry.Name,
		"skill_count":    len(discovered),
		"items":          discovered,
		"installed_path": filepath.ToSlash(destPath),
	}, nil
}

func executeAgentSkillList(payload map[string]any) (map[string]any, error) {
	registry, err := loadAgentSkillRegistry()
	if err != nil {
		return nil, err
	}
	diskEntries, err := agentScanInstalledSkillRepos()
	if err == nil && len(diskEntries) > 0 {
		registry.Entries = agentMergeSkillEntries(registry.Entries, diskEntries)
	}
	count := len(registry.Entries)
	if count == 0 {
		return map[string]any{"ok": true, "summary": "当前还没有装任何外部 skill。", "count": 0, "entries": []agentSkillRegistryEntry{}}, nil
	}
	lines := []string{fmt.Sprintf("现在一共装了 %d 个外部 skill 仓库：", count)}
	for idx, entry := range registry.Entries {
		kindNote := ""
		if entry.SkillCount > 0 {
			kindNote = fmt.Sprintf("，%d 个技能目录", entry.SkillCount)
		}
		lines = append(lines, fmt.Sprintf("%d. %s（%s/%s@%s%s）", idx+1, firstAgentNonEmpty(entry.Name, entry.RepoURL, entry.InstalledPath), entry.Owner, entry.Repo, entry.Branch, kindNote))
	}
	common.SysLog(fmt.Sprintf("[AgentSkill] list count=%d", count))
	return map[string]any{
		"ok":      true,
		"summary": strings.Join(lines, "\n"),
		"count":   count,
		"entries": registry.Entries,
	}, nil
}

func agentSkillStorageRoot() string {
	if root := strings.TrimSpace(os.Getenv("AGENT_SKILL_STORAGE_ROOT")); root != "" {
		return root
	}
	if stat, err := os.Stat("/data"); err == nil && stat.IsDir() {
		return filepath.Join("/data", "agent-skills")
	}
	return filepath.Join(".", "data", "agent-skills")
}

func agentSkillRegistryPath() string {
	return filepath.Join(agentSkillStorageRoot(), "registry.json")
}

func loadAgentSkillRegistry() (agentSkillRegistry, error) {
	registry := agentSkillRegistry{Entries: make([]agentSkillRegistryEntry, 0)}
	path := agentSkillRegistryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return registry, nil
		}
		return registry, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return registry, nil
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return registry, err
	}
	if registry.Entries == nil {
		registry.Entries = make([]agentSkillRegistryEntry, 0)
	}
	return registry, nil
}

func saveAgentSkillRegistry(registry agentSkillRegistry) error {
	registry.UpdatedAt = time.Now().Unix()
	if registry.Entries == nil {
		registry.Entries = make([]agentSkillRegistryEntry, 0)
	}
	path := agentSkillRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func agentDetectGitHubDefaultBranch(owner, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "new-api-agent-skill-installer/1.0")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("github default branch lookup failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.DefaultBranch), nil
}

func agentDownloadGitHubArchive(ref agentSkillRepoRef) (string, error) {
	archiveURL := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/heads/%s", url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), url.PathEscape(ref.Branch))
	req, err := http.NewRequest(http.MethodGet, archiveURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/zip")
	req.Header.Set("User-Agent", "new-api-agent-skill-installer/1.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("github archive download failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	file, err := os.CreateTemp("", "agent-skill-*.zip")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func agentExtractGitHubArchive(zipPath string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	root, err := os.MkdirTemp("", "agent-skill-unzip-*")
	if err != nil {
		return "", err
	}
	cleanRoot := filepath.Clean(root)
	for _, file := range reader.File {
		entryName := filepath.Clean(file.Name)
		if entryName == "." || strings.HasPrefix(entryName, "..") {
			continue
		}
		targetPath := filepath.Join(root, entryName)
		cleanTarget := filepath.Clean(targetPath)
		if cleanTarget != cleanRoot && !strings.HasPrefix(cleanTarget, cleanRoot+string(os.PathSeparator)) {
			return "", fmt.Errorf("zip entry escaped target dir: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
			return "", err
		}
		in, err := file.Open()
		if err != nil {
			return "", err
		}
		out, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return "", err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return root, nil
}

func agentNormalizeArchiveRoot(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return root
	}
	dirs := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(entries) == 1 && len(dirs) == 1 {
		return filepath.Join(root, dirs[0])
	}
	return root
}

func agentDiscoverInstalledSkills(root string) ([]agentSkillCatalogItem, error) {
	seen := map[string]*agentSkillCatalogItem{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		switch d.Name() {
		case "SKILL.md":
			dir := filepath.Dir(path)
			rel, _ := filepath.Rel(root, dir)
			item := agentEnsureSkillCatalogItem(seen, dir, rel)
			item.HasSkill = true
		case "plugin.json":
			if filepath.Base(filepath.Dir(path)) != ".codex-plugin" {
				return nil
			}
			dir := filepath.Dir(filepath.Dir(path))
			rel, _ := filepath.Rel(root, dir)
			item := agentEnsureSkillCatalogItem(seen, dir, rel)
			item.HasPlugin = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	items := make([]agentSkillCatalogItem, 0, len(seen))
	for _, item := range seen {
		items = append(items, *item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Name < items[j].Name
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func agentEnsureSkillCatalogItem(seen map[string]*agentSkillCatalogItem, dir, rel string) *agentSkillCatalogItem {
	key := filepath.Clean(dir)
	if item, ok := seen[key]; ok {
		return item
	}
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" || rel == "." {
		rel = "."
	}
	name := filepath.Base(key)
	if rel == "." {
		name = filepath.Base(filepath.Clean(dir))
	}
	item := &agentSkillCatalogItem{Name: name, Path: rel}
	seen[key] = item
	return item
}

func agentRepoInstallDir(ref agentSkillRepoRef) string {
	key := fmt.Sprintf("%s__%s__%s", ref.Owner, ref.Repo, ref.Branch)
	key = regexp.MustCompile(`[^A-Za-z0-9._-]+`).ReplaceAllString(key, "-")
	key = strings.Trim(strings.ReplaceAll(key, "--", "-"), "-")
	if key == "" {
		key = "agent-skill"
	}
	return filepath.Join(agentSkillStorageRoot(), key)
}

func agentCopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return agentCopyFile(path, target)
	})
}

func agentCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func agentScanInstalledSkillRepos() ([]agentSkillRegistryEntry, error) {
	root := agentSkillStorageRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]agentSkillRegistryEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.Contains(entry.Name(), ".tmp") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		items, err := agentDiscoverInstalledSkills(path)
		if err != nil || len(items) == 0 {
			continue
		}
		info, _ := entry.Info()
		out = append(out, agentSkillRegistryEntry{
			Name:          entry.Name(),
			InstalledPath: filepath.ToSlash(path),
			InstalledAt:   info.ModTime().Unix(),
			SkillCount:    len(items),
			Items:         items,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].InstalledAt == out[j].InstalledAt {
			return out[i].Name < out[j].Name
		}
		return out[i].InstalledAt > out[j].InstalledAt
	})
	return out, nil
}

func agentMergeSkillEntries(primary, fallback []agentSkillRegistryEntry) []agentSkillRegistryEntry {
	seen := map[string]bool{}
	out := make([]agentSkillRegistryEntry, 0, len(primary)+len(fallback))
	for _, entry := range primary {
		key := strings.ToLower(firstAgentNonEmpty(entry.InstalledPath, entry.RepoURL, entry.Name))
		seen[key] = true
		out = append(out, entry)
	}
	for _, entry := range fallback {
		key := strings.ToLower(firstAgentNonEmpty(entry.InstalledPath, entry.RepoURL, entry.Name))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].InstalledAt == out[j].InstalledAt {
			return out[i].Name < out[j].Name
		}
		return out[i].InstalledAt > out[j].InstalledAt
	})
	return out
}
