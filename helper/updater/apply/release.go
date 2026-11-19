package apply

// 文件职责：下载、校验并暂存指定 Release 的可执行文件。
// 使用：updater.PrepareAppUpdate 在确认存在更新后调用。
// 边界：不替换当前程序；只有退出交接后的独立进程可以安装暂存文件。

import (
	"bytes"
	"context"
	"fmt"
	"io"

	giteeSource "DysonSphereBox/helper/updater/gitee/source"
	"DysonSphereBox/helper/updater/progress"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// PrepareRelease 职责：下载、校验、解压并准备更新；使用：系统更新准备流程调用；边界：返回成功也不修改当前程序，保留既有 checksum 校验。
// 逻辑流：1. 下载并报告真实进度 => 2. 校验资产 => 3. 解压目标命令 => 4. 写入独占暂存目录。
func PrepareRelease(ctx context.Context, source *giteeSource.Source, release *selfupdate.Release, executablePath string, config Config) (*PreparedUpdate, error) {
	reader, err := source.DownloadReleaseAsset(ctx, release, release.AssetID)
	if err != nil {
		return nil, fmt.Errorf("download release asset %q: %w", release.AssetName, err)
	}
	defer reader.Close()

	assetData, err := progress.ReadAll(reader, int64(release.AssetByteSize), config.DownloadProgress)
	if err != nil {
		return nil, fmt.Errorf("read release asset %q: %w", release.AssetName, err)
	}
	if err := validateReleaseAsset(ctx, source, release, assetData, config.ChecksumName); err != nil {
		return nil, err
	}
	executableReader, err := selfupdate.DecompressCommand(
		bytes.NewReader(assetData), release.AssetName, archiveExecutableName(config.CommandName), release.OS, release.Arch,
	)
	if err != nil {
		return nil, err
	}
	if closer, ok := executableReader.(io.Closer); ok {
		defer closer.Close()
	}
	return prepareExecutable(executableReader, executablePath)
}
