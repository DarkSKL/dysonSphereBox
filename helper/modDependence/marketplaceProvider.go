package modDependence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"DysonSphereBox/apiDefinition"
	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence"
	modDependenceResponse "DysonSphereBox/model/modDependence/response"
)

const (
	remotePackageCacheKeyDSP = "dyson-sphere-program"
	remotePackageCacheTTL    = 6 * time.Hour
)

// RemotePackageQuery 表示统一市场查询条件。
// 用途：服务层传入来源、分页与搜索词，Provider 负责把来源响应转换为统一模型。
type RemotePackageQuery struct {
	Source            modDependenceResponse.ModSource
	Page              int
	PageSize          int
	Keyword           string
	IncludeDeprecated bool
	UseCache          bool
}

// ListRemotePackages 按来源返回统一 Mod 包分页数据。
// 边界：国内源沿用远端分页；Thunderstore 使用全量缓存后在本地分页。
func ListRemotePackages(ctx context.Context, query RemotePackageQuery) (modDependenceResponse.RemotePackageListData, error) {
	query = normalizeRemotePackageQuery(query)
	switch query.Source {
	case modDependenceResponse.ModSourceOfficial:
		return listOfficialRemotePackages(ctx, query)
	case modDependenceResponse.ModSourceThunderstore:
		return listThunderstoreRemotePackages(ctx, query)
	case modDependenceResponse.ModSourceManual:
		return modDependenceResponse.RemotePackageListData{
			List:     []modDependenceResponse.RemoteModPackageDTO{},
			Total:    0,
			Page:     query.Page,
			PageSize: query.PageSize,
			Source:   query.Source,
		}, nil
	default:
		return modDependenceResponse.RemotePackageListData{}, fmt.Errorf("unsupported mod source: %s", query.Source)
	}
}

// ResolveRemotePackage 根据统一 key 解析单个远端包。
// 边界：目前 Thunderstore 通过包索引定位；国内源包由前端列表 DTO 直接传入安装器。
func ResolveRemotePackage(ctx context.Context, key string) (*modDependenceResponse.RemoteModPackageDTO, error) {
	parts := strings.Split(key, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid package key: %s", key)
	}

	source := modDependenceResponse.ModSource(parts[0])
	switch source {
	case modDependenceResponse.ModSourceThunderstore:
		packages, err := loadThunderstorePackages(ctx, true)
		if err != nil {
			return nil, err
		}
		for _, pkg := range packages {
			for _, version := range pkg.Versions {
				candidate := thunderstorePackageKey(pkg, version)
				if candidate == key {
					result := thunderstorePackageToRemote(pkg, version, packages)
					return &result, nil
				}
			}
		}
		return nil, fmt.Errorf("Thunderstore package not found: %s", key)
	default:
		return nil, fmt.Errorf("resolve package is not implemented for source: %s", source)
	}
}

func normalizeRemotePackageQuery(query RemotePackageQuery) RemotePackageQuery {
	if query.Source == "" {
		query.Source = modDependenceResponse.ModSourceOfficial
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	return query
}

func listOfficialRemotePackages(ctx context.Context, query RemotePackageQuery) (modDependenceResponse.RemotePackageListData, error) {
	values := url.Values{}
	values.Set("page", strconv.Itoa(query.Page))
	values.Set("pageSize", strconv.Itoa(query.PageSize))
	requestPath := apiDefinition.API_URL_MOD_DEPENDENCE_LIST + "?" + values.Encode()

	request, err := core.GLOBAL.HTTPClient.NewRequest(ctx, http.MethodGet, requestPath, nil)
	if err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}

	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return modDependenceResponse.RemotePackageListData{}, fmt.Errorf("get official mod package list failed: %s", response.Status)
	}

	var result modDependenceResponse.PackageListResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}
	if result.Code != 0 {
		return modDependenceResponse.RemotePackageListData{}, fmt.Errorf("get official mod package list failed: %s", result.Msg)
	}

	list := make([]modDependenceResponse.RemoteModPackageDTO, 0, len(result.Data.List))
	for _, pack := range result.Data.List {
		remotePack := officialPackageToRemote(pack)
		if matchesPackageKeyword(remotePack, query.Keyword) {
			list = append(list, remotePack)
		}
	}

	return modDependenceResponse.RemotePackageListData{
		List:     list,
		Total:    result.Data.Total,
		Page:     result.Data.Page,
		PageSize: result.Data.PageSize,
		Source:   query.Source,
	}, nil
}

func listThunderstoreRemotePackages(ctx context.Context, query RemotePackageQuery) (modDependenceResponse.RemotePackageListData, error) {
	packages, err := loadThunderstorePackages(ctx, query.UseCache)
	if err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}

	list := make([]modDependenceResponse.RemoteModPackageDTO, 0, len(packages))
	for _, pkg := range packages {
		if isThunderstoreExternalOrFrameworkPackage(pkg) {
			continue
		}
		if pkg.IsDeprecated && !query.IncludeDeprecated {
			continue
		}
		version, ok := latestActiveVersion(pkg.Versions)
		if !ok {
			continue
		}
		if !thunderstoreVersionDependsOnBepInEx(pkg, version, packages, map[string]struct{}{}) {
			continue
		}
		remotePack := thunderstorePackageToRemote(pkg, version, packages)
		if matchesPackageKeyword(remotePack, query.Keyword) {
			list = append(list, remotePack)
		}
	}

	sort.SliceStable(list, func(left int, right int) bool {
		return list[left].UpdateTime > list[right].UpdateTime
	})

	total := len(list)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}

	return modDependenceResponse.RemotePackageListData{
		List:     list[start:end],
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		Source:   query.Source,
	}, nil
}

func loadThunderstorePackages(ctx context.Context, useCache bool) ([]thunderstorePackage, error) {
	if useCache {
		if cachedPackages, ok := readThunderstorePackageCache(); ok {
			return cachedPackages, nil
		}
	}

	client := http.Client{Timeout: 60 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, thunderstoreDSPPackageIndexURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		if cachedPackages, ok := readThunderstorePackageCache(); ok {
			return cachedPackages, nil
		}
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if cachedPackages, ok := readThunderstorePackageCache(); ok {
			return cachedPackages, nil
		}
		return nil, fmt.Errorf("Thunderstore returned HTTP %d", response.StatusCode)
	}

	var packages []thunderstorePackage
	if err := json.NewDecoder(response.Body).Decode(&packages); err != nil {
		return nil, err
	}
	writeThunderstorePackageCache(packages)
	return packages, nil
}

func readThunderstorePackageCache() ([]thunderstorePackage, bool) {
	db, err := requireDatabase()
	if err != nil {
		return nil, false
	}

	var cache struct {
		Content   []byte
		ExpiresAt time.Time
	}
	result := db.
		Table("mod_remote_package_cache").
		Select("content, expires_at").
		Where("source = ? AND cache_key = ?", string(modDependenceResponse.ModSourceThunderstore), remotePackageCacheKeyDSP).
		Take(&cache)
	if result.Error != nil || time.Now().After(cache.ExpiresAt) {
		return nil, false
	}

	var packages []thunderstorePackage
	if err := json.Unmarshal(cache.Content, &packages); err != nil {
		return nil, false
	}
	return packages, true
}

func writeThunderstorePackageCache(packages []thunderstorePackage) {
	db, err := requireDatabase()
	if err != nil {
		return
	}

	content, err := json.Marshal(packages)
	if err != nil {
		return
	}

	now := time.Now()
	record := modDependenceModel.ModRemotePackageCache{
		Source:    string(modDependenceResponse.ModSourceThunderstore),
		CacheKey:  remotePackageCacheKeyDSP,
		Content:   content,
		UpdatedAt: now,
		ExpiresAt: now.Add(remotePackageCacheTTL),
	}
	_ = db.
		Where("source = ? AND cache_key = ?", record.Source, record.CacheKey).
		Assign(record).
		FirstOrCreate(&record).
		Error
}

func officialPackageToRemote(pack modDependenceResponse.ModDependencyPackageDTO) modDependenceResponse.RemoteModPackageDTO {
	version := stringValue(pack.PackageVersion)
	key := fmt.Sprintf("official:%d:%s", pack.ID, sanitizeKeySegment(version, "version"))
	dependencies := make([]modDependenceResponse.RemoteModDependencyDTO, 0, len(pack.Dependencies))
	for _, dependency := range pack.Dependencies {
		dependencies = append(dependencies, officialDependencyToRemote(dependency))
	}

	return modDependenceResponse.RemoteModPackageDTO{
		Key:                 key,
		LegacyID:            pack.ID,
		Source:              modDependenceResponse.ModSourceOfficial,
		SourcePackageID:     strconv.FormatUint(uint64(pack.ID), 10),
		Name:                stringValue(pack.PackageName),
		DisplayName:         stringValue(pack.PackageName),
		Version:             version,
		Description:         stringValue(pack.PackageDescription),
		Author:              stringValue(pack.Author),
		DownloadURL:         resolveOfficialRemoteURL(stringValue(pack.Url)),
		PackageURL:          resolveOfficialRemoteURL(stringValue(pack.Url)),
		CoverURL:            stringValue(pack.CoverUrl),
		DownloadCount:       int64Value(pack.DownloadCount),
		DependencyCount:     int64Value(pack.DependencyCount),
		PackageSize:         int64Value(pack.PackageSize),
		GameVersions:        pack.GameVersions,
		Trustworthiness:     int64Value(pack.Trustworthiness),
		UpdateTime:          stringValue(pack.UpdateTime),
		ArtifactKind:        modDependenceResponse.ArtifactKindRemoteZip,
		Dependencies:        dependencies,
		RawDependencySource: "official",
	}
}

func officialDependencyToRemote(dependency modDependenceResponse.ModDependencyDTO) modDependenceResponse.RemoteModDependencyDTO {
	version := stringValue(dependency.ModVersion)
	key := fmt.Sprintf("official:%d:%s", dependency.ID, sanitizeKeySegment(version, "version"))
	return modDependenceResponse.RemoteModDependencyDTO{
		Key:                key,
		Source:             modDependenceResponse.ModSourceOfficial,
		SourceDependencyID: strconv.FormatUint(uint64(dependency.ID), 10),
		Name:               stringValue(dependency.ModName),
		DisplayName:        stringValue(dependency.ModName),
		Version:            version,
		Description:        stringValue(dependency.ModDescription),
		DownloadURL:        resolveOfficialRemoteURL(stringValue(dependency.Url)),
		PackageURL:         resolveOfficialRemoteURL(stringValue(dependency.Url)),
		ArtifactKind:       modDependenceResponse.ArtifactKindRemoteZip,
		Raw:                strconv.FormatUint(uint64(dependency.ID), 10),
	}
}

func resolveOfficialRemoteURL(rawURL string) string {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" || core.GLOBAL.HTTPClient == nil {
		return trimmedURL
	}
	resolvedURL, err := core.GLOBAL.HTTPClient.ResolveURL(trimmedURL)
	if err != nil {
		return trimmedURL
	}
	return resolvedURL
}

func thunderstorePackageToRemote(pkg thunderstorePackage, version thunderstoreVersion, packages []thunderstorePackage) modDependenceResponse.RemoteModPackageDTO {
	dependencies := make([]modDependenceResponse.RemoteModDependencyDTO, 0, len(version.Dependencies))
	for _, dependency := range version.Dependencies {
		dependencies = append(dependencies, parseThunderstoreDependency(dependency, packages))
	}
	return modDependenceResponse.RemoteModPackageDTO{
		Key:                 thunderstorePackageKey(pkg, version),
		Source:              modDependenceResponse.ModSourceThunderstore,
		SourcePackageID:     thunderstoreFullName(pkg),
		Namespace:           pkg.Owner,
		Name:                pkg.Name,
		DisplayName:         pkg.Name,
		Version:             version.VersionNumber,
		Description:         version.Description,
		Author:              pkg.Owner,
		DownloadURL:         version.DownloadURL,
		PackageURL:          pkg.PackageURL,
		CoverURL:            version.Icon,
		DownloadCount:       version.Downloads,
		DependencyCount:     int64(len(version.Dependencies) + 1),
		PackageSize:         version.FileSize,
		Tags:                append([]string(nil), pkg.Categories...),
		Trustworthiness:     pkg.RatingScore,
		UpdateTime:          version.DateCreated,
		Deprecated:          pkg.IsDeprecated,
		ArtifactKind:        modDependenceResponse.ArtifactKindRemoteZip,
		Dependencies:        dependencies,
		RawDependencySource: "thunderstore",
	}
}

func parseThunderstoreDependency(raw string, packages []thunderstorePackage) modDependenceResponse.RemoteModDependencyDTO {
	fullName := ""
	version := ""
	for _, pkg := range packages {
		candidate := thunderstoreFullName(pkg)
		prefix := candidate + "-"
		if strings.HasPrefix(raw, prefix) && len(candidate) > len(fullName) {
			fullName = candidate
			version = strings.TrimPrefix(raw, prefix)
		}
	}
	if fullName == "" {
		parts := strings.Split(raw, "-")
		if len(parts) >= 3 {
			fullName = parts[0] + "-" + parts[1]
			version = strings.Join(parts[2:], "-")
		} else {
			fullName = raw
		}
	}

	namespace, name := splitThunderstoreFullName(fullName)
	key := fmt.Sprintf("thunderstore:%s:%s:%s", namespace, name, version)
	dependency := modDependenceResponse.RemoteModDependencyDTO{
		Key:                key,
		Source:             modDependenceResponse.ModSourceThunderstore,
		SourceDependencyID: fullName,
		Namespace:          namespace,
		Name:               name,
		DisplayName:        name,
		Version:            version,
		ArtifactKind:       modDependenceResponse.ArtifactKindRemoteZip,
		Raw:                raw,
	}

	for _, pkg := range packages {
		if thunderstoreFullName(pkg) != fullName {
			continue
		}
		for _, pkgVersion := range pkg.Versions {
			if pkgVersion.VersionNumber != version || !pkgVersion.IsActive {
				continue
			}
			dependency.Description = pkgVersion.Description
			dependency.DownloadURL = pkgVersion.DownloadURL
			dependency.PackageURL = pkg.PackageURL
			dependency.Deprecated = pkg.IsDeprecated
			return dependency
		}
	}

	return dependency
}

func isThunderstoreExternalOrFrameworkPackage(pkg thunderstorePackage) bool {
	if pkg.Owner == bepInExOwner && pkg.Name == bepInExName {
		return true
	}
	for _, category := range pkg.Categories {
		if strings.EqualFold(category, "Tools") {
			return true
		}
	}
	return false
}

func thunderstoreVersionDependsOnBepInEx(pkg thunderstorePackage, version thunderstoreVersion, packages []thunderstorePackage, visited map[string]struct{}) bool {
	packageID := thunderstoreFullName(pkg) + "-" + version.VersionNumber
	if _, exists := visited[packageID]; exists {
		return false
	}
	visited[packageID] = struct{}{}

	for _, dependency := range version.Dependencies {
		dependencyInfo := parseThunderstoreDependency(dependency, packages)
		if dependencyInfo.Namespace == bepInExOwner && dependencyInfo.Name == bepInExName {
			return true
		}
		dependencyPackage, dependencyVersion, ok := findThunderstorePackageVersion(packages, dependencyInfo.Namespace, dependencyInfo.Name, dependencyInfo.Version)
		if !ok {
			continue
		}
		if thunderstoreVersionDependsOnBepInEx(dependencyPackage, dependencyVersion, packages, visited) {
			return true
		}
	}

	return false
}

func findThunderstorePackageVersion(packages []thunderstorePackage, namespace string, name string, version string) (thunderstorePackage, thunderstoreVersion, bool) {
	for _, pkg := range packages {
		if pkg.Owner != namespace || pkg.Name != name {
			continue
		}
		for _, pkgVersion := range pkg.Versions {
			if pkgVersion.VersionNumber == version && pkgVersion.IsActive {
				return pkg, pkgVersion, true
			}
		}
	}
	return thunderstorePackage{}, thunderstoreVersion{}, false
}

func thunderstorePackageKey(pkg thunderstorePackage, version thunderstoreVersion) string {
	return fmt.Sprintf("thunderstore:%s:%s:%s", pkg.Owner, pkg.Name, version.VersionNumber)
}

func thunderstoreFullName(pkg thunderstorePackage) string {
	if pkg.FullName != "" {
		return pkg.FullName
	}
	return pkg.Owner + "-" + pkg.Name
}

func splitThunderstoreFullName(fullName string) (string, string) {
	parts := strings.SplitN(fullName, "-", 2)
	if len(parts) != 2 {
		return "", fullName
	}
	return parts[0], parts[1]
}

func matchesPackageKeyword(pack modDependenceResponse.RemoteModPackageDTO, keyword string) bool {
	keyword = strings.TrimSpace(strings.ToLower(keyword))
	if keyword == "" {
		return true
	}

	candidates := []string{
		pack.DisplayName,
		pack.Name,
		pack.Description,
		pack.Author,
		pack.Namespace,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), keyword) {
			return true
		}
	}
	return false
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func sanitizeKeySegment(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return strings.ReplaceAll(value, ":", "-")
}
