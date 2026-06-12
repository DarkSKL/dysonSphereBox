package apply

// 文件职责：下载、校验、解压并应用指定 Release 的可执行文件资产。
// 使用：updater.UpdateAppSelf 在确认存在可更新 Release 且解析出当前可执行路径后调用。
// 边界：不负责版本比较、Release 选择或前端提示；只处理 go-selfupdate 已选中的 Release 资产。

import (
	"bytes"
	"context"
	"fmt"
	"io"

	giteeSource "DysonSphereBox/helper/updater/gitee/source"
	selfupdate "github.com/creativeprojects/go-selfupdate"
	selfupdateApply "github.com/creativeprojects/go-selfupdate/update"
)

// ApplyRelease 下载并应用指定 Release 的可执行文件资产。
// 职责：完成更新资产下载、校验、解压和替换当前可执行文件。
// 使用：UpdateAppSelf 在确认存在可更新 Release 且解析出当前可执行路径后调用。
// 边界：只处理 go-selfupdate 已选中的 Release 资产，不负责版本比较或前端提示；Windows 扩展名修正失败会作为错误返回。
// 逻辑流：1. 下载 Release 资产 => 2. 读取完整资产内容 => 3. 使用校验链验证资产 => 4. 解压目标命令文件 => 5. 应用到当前可执行路径并修正 Windows 扩展名。
func ApplyRelease(ctx context.Context, source *giteeSource.Source, release *selfupdate.Release, executablePath string, config Config) error {
	reader, err := source.DownloadReleaseAsset(ctx, release, release.AssetID)
	if err != nil {
		return fmt.Errorf("download release asset %q: %w", release.AssetName, err)
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			fmt.Printf("failed to close reader: %s\n", err)
		}
	}(reader)

	assetData, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read release asset %q: %w", release.AssetName, err)
	}
	if err := validateReleaseAsset(ctx, source, release, assetData, config.ChecksumName); err != nil {
		return err
	}

	executableReader, err := selfupdate.DecompressCommand(
		bytes.NewReader(assetData),
		release.AssetName,
		archiveExecutableName(config.CommandName),
		release.OS,
		release.Arch,
	)
	if err != nil {
		return err
	}

	if err := selfupdateApply.Apply(executableReader, selfupdateApply.Options{
		TargetPath: executablePath,
	}); err != nil {
		return err
	}

	return ensureWindowsExecutableExtension(executablePath)
}
