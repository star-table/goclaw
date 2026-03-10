package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/smallnest/goclaw/console/internal/model"
	"gopkg.in/yaml.v3"
)

// SkillImporter handles importing skills from various sources
type SkillImporter struct {
	httpClient     *http.Client
	hubBaseURL     string
	hubSearchPath  string
	hubDetailPath  string
	hubVersionPath string
	hubFilePath    string
	httpTimeout    time.Duration
	httpRetries    int
	backoffBase    time.Duration
	backoffCap     time.Duration
}

// HubSkillResult represents a skill from the hub
type HubSkillResult struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	SourceURL   string `json:"source_url"`
}

// HubInstallResult represents the result of installing a skill
type HubInstallResult struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	SourceURL string `json:"source_url"`
}

// SkillBundle represents a skill bundle from any source
type SkillBundle struct {
	Name       string                 `json:"name"`
	Content    string                 `json:"content"` // SKILL.md content
	References map[string]interface{} `json:"references"`
	Scripts    map[string]interface{} `json:"scripts"`
	ExtraFiles map[string]interface{} `json:"extra_files"`
	Files      map[string]string      `json:"files"` // Flat file mapping
}

// NewSkillImporter creates a new skill importer
func NewSkillImporter() *SkillImporter {
	return &SkillImporter{
		httpClient:     &http.Client{Timeout: getEnvDuration("COPAW_SKILLS_HUB_HTTP_TIMEOUT", 15*time.Second)},
		hubBaseURL:     getEnvString("COPAW_SKILLS_HUB_BASE_URL", "https://clawhub.ai"),
		hubSearchPath:  getEnvString("COPAW_SKILLS_HUB_SEARCH_PATH", "/api/v1/search"),
		hubDetailPath:  getEnvString("COPAW_SKILLS_HUB_DETAIL_PATH", "/api/v1/skills/{slug}"),
		hubVersionPath: getEnvString("COPAW_SKILLS_HUB_VERSION_PATH", "/api/v1/skills/{slug}/versions/{version}"),
		hubFilePath:    getEnvString("COPAW_SKILLS_HUB_FILE_PATH", "/api/v1/skills/{slug}/file"),
		httpTimeout:    getEnvDuration("COPAW_SKILLS_HUB_HTTP_TIMEOUT", 15*time.Second),
		httpRetries:    getEnvInt("COPAW_SKILLS_HUB_HTTP_RETRIES", 3),
		backoffBase:    getEnvDuration("COPAW_SKILLS_HUB_BACKOFF_BASE", 800*time.Millisecond),
		backoffCap:     getEnvDuration("COPAW_SKILLS_HUB_BACKOFF_CAP", 6*time.Second),
	}
}

// InstallSkillFromHub installs a skill from a hub URL
func (i *SkillImporter) InstallSkillFromHub(bundleURL string, version string, enable bool, overwrite bool, skillSvc *SkillService) (*HubInstallResult, error) {
	if bundleURL == "" || !isHTTPURL(bundleURL) {
		return nil, fmt.Errorf("bundle_url must be a valid http(s) URL")
	}

	sourceURL := bundleURL
	var bundle *SkillBundle
	var err error

	// Try different sources based on URL pattern
	switch {
	case i.isSkillsShURL(bundleURL):
		bundle, sourceURL, err = i.fetchFromSkillsSh(bundleURL, version)
	case i.isGitHubURL(bundleURL):
		bundle, sourceURL, err = i.fetchFromGitHub(bundleURL, version)
	case i.isSkillsMPURL(bundleURL):
		bundle, sourceURL, err = i.fetchFromSkillsMP(bundleURL, version)
	case i.isClawHubURL(bundleURL):
		slug := i.extractClawHubSlug(bundleURL)
		bundle, sourceURL, err = i.fetchFromClawHub(slug, version)
	default:
		// Fallback: try to fetch as direct bundle JSON
		bundle, err = i.fetchBundleJSON(bundleURL)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill bundle: %w", err)
	}

	// Normalize bundle
	name, content, references, scripts, extraFiles, err := i.normalizeBundle(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize bundle: %w", err)
	}

	if name == "" {
		fallback := filepath.Base(strings.TrimSuffix(bundleURL, "/"))
		name = safeFallbackName(fallback)
	}

	// Create skill using SkillService
	req := &model.CreateSkillRequest{
		Name:       name,
		Content:    content,
		Overwrite:  overwrite,
		References: references,
		Scripts:    scripts,
		ExtraFiles: extraFiles,
	}

	resp := skillSvc.CreateSkill(req)
	if !resp.Created {
		return nil, fmt.Errorf("failed to create skill '%s'. Try overwrite=true if it already exists", name)
	}

	// Enable skill if requested
	enabled := false
	if enable {
		enableResp, err := skillSvc.EnableSkill(name)
		if err != nil {
			fmt.Printf("Warning: Skill '%s' imported but enable failed: %v\n", name, err)
		} else {
			enabled = enableResp.Enabled
		}
	}

	return &HubInstallResult{
		Name:      name,
		Enabled:   enabled,
		SourceURL: sourceURL,
	}, nil
}

// SearchHubSkills searches for skills in the hub
func (i *SkillImporter) SearchHubSkills(query string, limit int) ([]*HubSkillResult, error) {
	searchURL := i.joinURL(i.hubBaseURL, i.hubSearchPath)
	params := url.Values{
		"q":     {query},
		"limit": {fmt.Sprintf("%d", limit)},
	}

	data, err := i.httpGetJSON(searchURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}

	items := normSearchItems(data)
	results := make([]*HubSkillResult, 0)

	for _, item := range items {
		slug := strings.TrimSpace(getString(item, "slug", "name"))
		if slug == "" {
			continue
		}
		results = append(results, &HubSkillResult{
			Slug:        slug,
			Name:        getString(item, "name", "displayName", "slug"),
			Description: getString(item, "description", "summary"),
			Version:     getString(item, "version"),
			SourceURL:   getString(item, "url"),
		})
	}

	return results, nil
}

// URL解析器

func (i *SkillImporter) isSkillsShURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return host == "skills.sh" || host == "www.skills.sh"
}

func (i *SkillImporter) isGitHubURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return host == "github.com" || host == "www.github.com"
}

func (i *SkillImporter) isSkillsMPURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return host == "skillsmp.com" || host == "www.skillsmp.com"
}

func (i *SkillImporter) isClawHubURL(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return strings.Contains(host, "clawhub")
}

func (i *SkillImporter) extractSkillsShSpec(urlStr string) (owner, repo, skill string, ok bool) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", "", "", false
	}
	host := strings.ToLower(parsed.Host)
	if host != "skills.sh" && host != "www.skills.sh" {
		return "", "", "", false
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func (i *SkillImporter) extractGitHubSpec(urlStr string) (owner, repo, branch, pathHint string, ok bool) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", "", "", "", false
	}
	host := strings.ToLower(parsed.Host)
	if host != "github.com" && host != "www.github.com" {
		return "", "", "", "", false
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", "", "", false
	}

	owner, repo = parts[0], parts[1]
	if len(parts) >= 4 && (parts[2] == "tree" || parts[2] == "blob") {
		branch = parts[3]
		if len(parts) > 4 {
			pathHint = strings.Join(parts[4:], "/")
		}
	} else if len(parts) > 2 {
		pathHint = strings.Join(parts[2:], "/")
	}

	return owner, repo, branch, pathHint, true
}

func (i *SkillImporter) extractSkillsMPSlug(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	if host != "skillsmp.com" && host != "www.skillsmp.com" {
		return ""
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	// Look for /skills/<slug> pattern
	for idx, part := range parts {
		if part == "skills" && idx+1 < len(parts) {
			return parts[idx+1]
		}
	}
	return ""
}

func (i *SkillImporter) extractClawHubSlug(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	if !strings.Contains(host, "clawhub") {
		return ""
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// HTTP客户端

func (i *SkillImporter) httpGet(urlStr string) (string, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(parsed.Host)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "goclaw-skills-hub/1.0")

	// Add GitHub token if available
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		githubToken = os.Getenv("GH_TOKEN")
	}
	if githubToken != "" && strings.Contains(host, "api.github.com") {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}

	retries := i.httpRetries
	var lastErr error

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			delay := i.computeBackoff(attempt)
			time.Sleep(delay)
		}

		resp, err := i.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return string(body), nil
		}

		// Check for rate limit on GitHub
		if resp.StatusCode == http.StatusForbidden && strings.Contains(host, "api.github.com") {
			bodyStr := string(body)
			if strings.Contains(strings.ToLower(bodyStr), "rate limit") {
				return "", fmt.Errorf("GitHub API rate limit exceeded. Set GITHUB_TOKEN or GH_TOKEN to increase the limit")
			}
		}

		// Retry on certain status codes
		if attempt < retries && isRetryableStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			continue
		}

		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("failed after %d retries", retries)
}

func (i *SkillImporter) httpGetJSON(urlStr string) (map[string]interface{}, error) {
	body, err := i.httpGet(urlStr)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (i *SkillImporter) computeBackoff(attempt int) time.Duration {
	backoff := i.backoffBase * time.Duration(1<<(attempt-1))
	if backoff > i.backoffCap {
		return i.backoffCap
	}
	return backoff
}

// GitHub API集成

func (i *SkillImporter) fetchFromGitHub(urlStr, version string) (*SkillBundle, string, error) {
	owner, repo, branchInURL, pathHint, ok := i.extractGitHubSpec(urlStr)
	if !ok {
		return nil, "", fmt.Errorf("invalid GitHub URL format")
	}

	pathHint = strings.Trim(pathHint, "/")
	if strings.HasSuffix(pathHint, "/SKILL.md") {
		pathHint = strings.TrimSuffix(pathHint, "/SKILL.md")
	} else if pathHint == "SKILL.md" {
		pathHint = ""
	}

	branch := version
	if branch == "" {
		branch = branchInURL
	}

	return i.fetchFromRepoAndSkillHint(owner, repo, pathHint, branch)
}

func (i *SkillImporter) fetchFromSkillsSh(urlStr, version string) (*SkillBundle, string, error) {
	owner, repo, skill, ok := i.extractSkillsShSpec(urlStr)
	if !ok {
		return nil, "", fmt.Errorf("invalid skills.sh URL format")
	}

	bundle, err := i.fetchFromRepo(owner, repo, skill, version)
	if err != nil {
		return nil, "", err
	}

	sourceURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	return bundle, sourceURL, nil
}

func (i *SkillImporter) fetchFromSkillsMP(urlStr, version string) (*SkillBundle, string, error) {
	slug := i.extractSkillsMPSlug(urlStr)
	if slug == "" {
		return nil, "", fmt.Errorf("invalid skillsmp URL format")
	}

	// Parse slug to extract owner/repo/skill
	owner, repo, skillHint, ok := i.parseSkillsMPSlug(slug)
	if !ok {
		return nil, "", fmt.Errorf("could not parse skillsmp slug")
	}

	bundle, sourceURL, err := i.fetchFromRepoAndSkillHint(owner, repo, skillHint, version)
	if err != nil {
		return nil, "", err
	}

	return bundle, sourceURL, nil
}

func (i *SkillImporter) fetchFromClawHub(slug, version string) (*SkillBundle, string, error) {
	if slug == "" {
		return nil, "", fmt.Errorf("slug is required for clawhub install")
	}

	detailURL := i.joinURL(i.hubBaseURL, strings.Replace(i.hubDetailPath, "{slug}", slug, 1))
	data, err := i.httpGetJSON(detailURL)
	if err != nil {
		return nil, "", err
	}

	// Hydrate payload with file contents
	hydrated, err := i.hydrateClawHubPayload(data, slug, version)
	if err != nil {
		return nil, "", err
	}

	return hydrated, detailURL, nil
}

func (i *SkillImporter) fetchBundleJSON(urlStr string) (*SkillBundle, error) {
	data, err := i.httpGetJSON(urlStr)
	if err != nil {
		return nil, err
	}
	return i.jsonToBundle(data)
}

func (i *SkillImporter) fetchFromRepo(owner, repo, skill, version string) (*SkillBundle, error) {
	branchCandidates := []string{"main", "master"}
	if version != "" {
		branchCandidates = append([]string{version}, branchCandidates...)
	}

	var selectedRoot string
	var skillMDContent string
	var branch string

	for _, candidateBranch := range branchCandidates {
		branch = candidateBranch
		roots := []string{
			filepath.Join("skills", skill),
			skill,
			"",
		}

		for _, root := range roots {
			skillMDPath := filepath.Join(root, "SKILL.md")
			content, err := i.githubGetContent(owner, repo, skillMDPath, branch)
			if err != nil {
				continue
			}
			selectedRoot = root
			skillMDContent = content
			break
		}

		if skillMDContent != "" {
			break
		}
	}

	if skillMDContent == "" {
		return nil, fmt.Errorf("could not find SKILL.md in source repository")
	}

	files := map[string]string{
		"SKILL.md": skillMDContent,
	}

	// Collect references and scripts
	for _, subdir := range []string{"references", "scripts"} {
		subFiles, err := i.githubCollectTreeFiles(owner, repo, branch, selectedRoot, subdir)
		if err != nil {
			continue
		}
		for k, v := range subFiles {
			files[k] = v
		}
	}

	return &SkillBundle{
		Name:  skill,
		Files: files,
	}, nil
}

func (i *SkillImporter) fetchFromRepoAndSkillHint(owner, repo, skillHint, version string) (*SkillBundle, string, error) {
	bundle, err := i.fetchFromRepo(owner, repo, skillHint, version)
	if err != nil {
		return nil, "", err
	}

	sourceURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	return bundle, sourceURL, nil
}

func (i *SkillImporter) githubGetContent(owner, repo, path, ref string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)
	data, err := i.httpGetJSON(url)
	if err != nil {
		return "", err
	}

	// Try download_url first
	downloadURL := getString(data, "download_url")
	if downloadURL != "" {
		content, err := i.httpGet(downloadURL)
		if err != nil {
			return "", err
		}
		return content, nil
	}

	// Fall back to base64 content
	content := getString(data, "content")
	if content != "" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content, "\n", ""))
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}

	return "", fmt.Errorf("unable to read file content from GitHub")
}

func (i *SkillImporter) githubCollectTreeFiles(owner, repo, ref, root, subdir string) (map[string]string, error) {
	files := make(map[string]string)
	path := filepath.Join(root, subdir)

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)
	data, err := i.httpGetJSON(url)
	if err != nil {
		return files, err
	}

	// If it's a single file
	if _, ok := data["type"]; ok {
		if data["type"] == "file" {
			relPath := strings.TrimPrefix(getString(data, "path"), root+"/")
			content, err := i.githubGetContent(owner, repo, getString(data, "path"), ref)
			if err == nil {
				files[relPath] = content
			}
		}
		return files, nil
	}

	// It's a directory listing
	items, ok := data["items"].([]interface{})
	if !ok {
		return files, nil
	}

	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType := getString(itemMap, "type")
		itemPath := getString(itemMap, "path")

		if itemType == "dir" {
			subFiles, _ := i.githubCollectTreeFiles(owner, repo, ref, root, strings.TrimPrefix(itemPath, root+"/"))
			for k, v := range subFiles {
				files[k] = v
			}
		} else if itemType == "file" {
			relPath := strings.TrimPrefix(itemPath, root+"/")
			if strings.HasPrefix(relPath, "references/") || strings.HasPrefix(relPath, "scripts/") {
				content, err := i.githubGetContent(owner, repo, itemPath, ref)
				if err == nil {
					files[relPath] = content
				}
			}
		}
	}

	return files, nil
}

// ClawHub集成

func (i *SkillImporter) hydrateClawHubPayload(data map[string]interface{}, slug, requestedVersion string) (*SkillBundle, error) {
	// If already has content, return as-is
	if content := getString(data, "content", "skill_md", "skillMd"); content != "" {
		return i.jsonToBundle(data)
	}

	skill, ok := data["skill"].(map[string]interface{})
	if !ok {
		return i.jsonToBundle(data)
	}

	skillSlug := strings.TrimSpace(getString(skill, "slug"))
	if skillSlug == "" {
		skillSlug = slug
	}
	if skillSlug == "" {
		return i.jsonToBundle(data)
	}

	// Get version info
	versionHint := requestedVersion
	if versionHint == "" {
		if latest, ok := data["latestVersion"].(map[string]interface{}); ok {
			versionHint = getString(latest, "version")
		}
	}

	// Fetch version details if needed
	var versionData map[string]interface{}
	if versionHint != "" {
		versionURL := i.joinURL(i.hubBaseURL, strings.Replace(strings.Replace(i.hubVersionPath, "{slug}", skillSlug, 1), "{version}", versionHint, 1))
		versionData, _ = i.httpGetJSON(versionURL)
	}

	if versionData == nil {
		versionData = data
	}

	versionObj, ok := versionData["version"].(map[string]interface{})
	if !ok {
		return i.jsonToBundle(data)
	}

	filesMeta, ok := versionObj["files"].([]interface{})
	if !ok {
		return i.jsonToBundle(data)
	}

	versionStr := getString(versionObj, "version")
	fileURL := i.joinURL(i.hubBaseURL, strings.Replace(i.hubFilePath, "{slug}", skillSlug, 1))

	files := make(map[string]string)
	for _, item := range filesMeta {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		path := getString(itemMap, "path")
		if path == "" {
			continue
		}

		params := url.Values{"path": {path}}
		if versionStr != "" {
			params.Set("version", versionStr)
		}

		content, err := i.httpGet(fileURL + "?" + params.Encode())
		if err != nil {
			continue
		}
		files[path] = content
	}

	if files["SKILL.md"] == "" {
		return i.jsonToBundle(data)
	}

	return &SkillBundle{
		Name:  getString(skill, "displayName", "name", skillSlug),
		Files: files,
	}, nil
}

// Bundle解析

func (i *SkillImporter) jsonToBundle(data map[string]interface{}) (*SkillBundle, error) {
	payload := data
	if skill, ok := data["skill"].(map[string]interface{}); ok {
		payload = skill
	}

	bundle := &SkillBundle{
		Name:       getString(payload, "name"),
		Content:    getString(payload, "content", "skill_md", "skillMd"),
		References: make(map[string]interface{}),
		Scripts:    make(map[string]interface{}),
		ExtraFiles: make(map[string]interface{}),
		Files:      make(map[string]string),
	}

	// Parse files
	if files, ok := payload["files"].(map[string]interface{}); ok {
		for k, v := range files {
			if content, ok := v.(string); ok {
				bundle.Files[k] = content
			}
		}
	}

	// Parse references and scripts
	if refs, ok := payload["references"].(map[string]interface{}); ok {
		bundle.References = sanitizeTree(refs)
	}
	if scripts, ok := payload["scripts"].(map[string]interface{}); ok {
		bundle.Scripts = sanitizeTree(scripts)
	}

	return bundle, nil
}

func (i *SkillImporter) normalizeBundle(bundle *SkillBundle) (name, content string, references, scripts, extraFiles map[string]interface{}, err error) {
	content = bundle.Content
	if content == "" && bundle.Files["SKILL.md"] != "" {
		content = bundle.Files["SKILL.md"]
	}

	if content == "" {
		return "", "", nil, nil, nil, fmt.Errorf("hub bundle missing SKILL.md content")
	}

	// Extract name from frontmatter if not provided
	name = bundle.Name
	if name == "" {
		name = extractNameFromFrontMatter(content)
	}
	if name == "" {
		return "", "", nil, nil, nil, fmt.Errorf("hub bundle missing skill name")
	}

	// Build trees from flat files
	references = bundle.References
	scripts = bundle.Scripts
	extraFiles = bundle.ExtraFiles

	if len(bundle.Files) > 0 {
		refs2, scr2 := filesToTree(bundle.Files)
		if len(references) == 0 {
			references = refs2
		}
		if len(scripts) == 0 {
			scripts = scr2
		}

		// Collect extra files
		for rel, fileContent := range bundle.Files {
			if rel == "SKILL.md" {
				continue
			}
			parts := safePathParts(rel)
			if len(parts) == 0 {
				continue
			}
			if parts[0] == "references" || parts[0] == "scripts" {
				continue
			}
			treeInsert(extraFiles, parts, fileContent)
		}
	}

	return name, content, references, scripts, extraFiles, nil
}

// 辅助函数

func (i *SkillImporter) parseSkillsMPSlug(slug string) (owner, repo, skillHint string, ok bool) {
	if slug == "" {
		return "", "", "", false
	}

	// Remove -skill-md suffix
	if strings.HasSuffix(slug, "-skill-md") {
		slug = strings.TrimSuffix(slug, "-skill-md")
	}

	tokens := strings.Split(slug, "-")
	if len(tokens) < 3 {
		return "", "", "", false
	}

	owner = tokens[0]

	// Try different split points to find valid repo
	for i := min(len(tokens)-1, 6); i > 0; i-- {
		repo = strings.Join(tokens[1:i+1], "-")
		if repo != "" {
			// In a real implementation, we'd check if repo exists
			// For now, use a heuristic
			remainder := tokens[i+1:]
			skillHint = strings.Join(remainder, "-")
			return owner, repo, skillHint, true
		}
	}

	// Fallback
	return owner, tokens[1], strings.Join(tokens[2:], "-"), true
}

func (i *SkillImporter) joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

// 工具函数

func isHTTPURL(s string) bool {
	parsed, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func isRetryableStatus(status int) bool {
	retryable := map[int]bool{
		408: true, 409: true, 425: true, 429: true,
		500: true, 502: true, 503: true, 504: true,
	}
	return retryable[status]
}

func safeFallbackName(raw string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	out := re.ReplaceAllString(raw, "-")
	out = strings.Trim(out, "-_")
	if out == "" {
		return "imported-skill"
	}
	return out
}

func safePathParts(path string) []string {
	if path == "" || strings.HasPrefix(path, "/") {
		return nil
	}
	parts := strings.Split(path, "/")
	var result []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if p == "." || p == ".." {
			return nil
		}
		result = append(result, p)
	}
	return result
}

func treeInsert(tree map[string]interface{}, parts []string, content string) {
	if len(parts) == 0 {
		return
	}
	node := tree
	for _, part := range parts[:len(parts)-1] {
		child, ok := node[part].(map[string]interface{})
		if !ok {
			child = make(map[string]interface{})
			node[part] = child
		}
		node = child
	}
	node[parts[len(parts)-1]] = content
}

func filesToTree(files map[string]string) (references, scripts map[string]interface{}) {
	references = make(map[string]interface{})
	scripts = make(map[string]interface{})

	for rel, content := range files {
		parts := safePathParts(rel)
		if len(parts) == 0 {
			continue
		}
		if parts[0] == "references" && len(parts) > 1 {
			treeInsert(references, parts[1:], content)
		} else if parts[0] == "scripts" && len(parts) > 1 {
			treeInsert(scripts, parts[1:], content)
		}
	}

	return references, scripts
}

func sanitizeTree(tree interface{}) map[string]interface{} {
	m, ok := tree.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}

	out := make(map[string]interface{})
	for k, v := range m {
		if k == "." || k == ".." || strings.Contains(k, "/") || strings.Contains(k, "\\") {
			continue
		}
		switch val := v.(type) {
		case map[string]interface{}:
			out[k] = sanitizeTree(val)
		case string:
			out[k] = val
		}
	}
	return out
}

func extractNameFromFrontMatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return ""
	}

	var meta map[string]interface{}
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return ""
	}

	name, _ := meta["name"].(string)
	return name
}

func normSearchItems(data interface{}) []map[string]interface{} {
	if items, ok := data.([]interface{}); ok {
		result := make([]map[string]interface{}, 0)
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result
	}

	if m, ok := data.(map[string]interface{}); ok {
		for _, key := range []string{"items", "skills", "results", "data"} {
			if val, ok := m[key].([]interface{}); ok {
				result := make([]map[string]interface{}, 0)
				for _, item := range val {
					if itemMap, ok := item.(map[string]interface{}); ok {
						result = append(result, itemMap)
					}
				}
				return result
			}
		}
		// Single item
		if _, hasName := m["name"]; hasName {
			if _, hasSlug := m["slug"]; hasSlug {
				return []map[string]interface{}{m}
			}
		}
	}

	return nil
}

func getString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
