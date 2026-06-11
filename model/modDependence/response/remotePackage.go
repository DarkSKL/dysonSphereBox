package response

// ModSource 表示统一 Mod 市场数据来源。
// 用途：前端、Provider 和安装器通过该值区分国内源、Thunderstore 与手动导入来源。
type ModSource string

const (
	// ModSourceOfficial 表示当前自有接口来源。
	ModSourceOfficial ModSource = "official"
	// ModSourceThunderstore 表示 Thunderstore 来源。
	ModSourceThunderstore ModSource = "thunderstore"
	// ModSourceManual 表示用户手动导入来源。
	ModSourceManual ModSource = "manual"
)

// ArtifactKind 表示安装器获取 Mod 产物的方式。
// 边界：远端 zip 需要先下载，本地 zip 与本地目录由手动导入流程提供。
type ArtifactKind string

const (
	// ArtifactKindRemoteZip 表示通过 URL 下载 zip 包。
	ArtifactKindRemoteZip ArtifactKind = "remote_zip"
	// ArtifactKindLocalZip 表示用户选择的本地 zip 包。
	ArtifactKindLocalZip ArtifactKind = "local_zip"
	// ArtifactKindLocalDir 表示用户选择的本地目录。
	ArtifactKindLocalDir ArtifactKind = "local_dir"
)

// RemotePackageListData 表示 Thunderstore 等来源适配后的 Mod 包分页数据。
// 用途：前端 Mod 页只消费该结构，不再关心不同来源的原始响应形状。
type RemotePackageListData struct {
	List     []RemoteModPackageDTO `json:"list"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Source   ModSource             `json:"source"`
}

// RemoteModDependencyDTO 表示 Thunderstore 等来源适配后的依赖组件。
// 边界：Key 是跨来源稳定身份；Raw 保留原始依赖声明，便于排查来源解析问题。
type RemoteModDependencyDTO struct {
	Key                string       `json:"key"`
	Source             ModSource    `json:"source"`
	SourceDependencyID string       `json:"sourceDependencyId"`
	Namespace          string       `json:"namespace"`
	Name               string       `json:"name"`
	DisplayName        string       `json:"displayName"`
	Version            string       `json:"version"`
	Description        string       `json:"description"`
	DownloadURL        string       `json:"downloadUrl"`
	PackageURL         string       `json:"packageUrl"`
	ArtifactKind       ArtifactKind `json:"artifactKind"`
	LocalPath          string       `json:"localPath"`
	Raw                string       `json:"raw"`
	Deprecated         bool         `json:"deprecated"`
}

// RemoteModPackageDTO 表示 Thunderstore 等来源适配后的可安装 Mod 包。
// 边界：保留 LegacyID 只用于兼容旧国内源状态；新逻辑必须以 Key 为唯一身份。
type RemoteModPackageDTO struct {
	Key                 string                   `json:"key"`
	LegacyID            uint                     `json:"legacyId"`
	Source              ModSource                `json:"source"`
	SourcePackageID     string                   `json:"sourcePackageId"`
	Namespace           string                   `json:"namespace"`
	Name                string                   `json:"name"`
	DisplayName         string                   `json:"displayName"`
	Version             string                   `json:"version"`
	Description         string                   `json:"description"`
	Author              string                   `json:"author"`
	DownloadURL         string                   `json:"downloadUrl"`
	PackageURL          string                   `json:"packageUrl"`
	CoverURL            string                   `json:"coverUrl"`
	DownloadCount       int64                    `json:"downloadCount"`
	DependencyCount     int64                    `json:"dependencyCount"`
	PackageSize         int64                    `json:"packageSize"`
	Tags                []string                 `json:"tags"`
	GameVersions        interface{}              `json:"gameVersions"`
	Trustworthiness     int64                    `json:"trustworthiness"`
	UpdateTime          string                   `json:"updateTime"`
	Deprecated          bool                     `json:"deprecated"`
	ArtifactKind        ArtifactKind             `json:"artifactKind"`
	LocalPath           string                   `json:"localPath"`
	Dependencies        []RemoteModDependencyDTO `json:"dependencies"`
	RawDependencySource string                   `json:"rawDependencySource"`
}
