package apply

// 文件职责：校验已下载的应用更新资产及其校验链。
// 使用：ApplyRelease 在解压和替换可执行文件前调用 validateReleaseAsset。
// 边界：只验证字节内容可信度，不下载主资产、不应用文件、不决定版本新旧。

import (
	"context"
	"fmt"
	"io"

	giteeSource "DysonSphereBox/helper/updater/gitee/source"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// validateReleaseAsset 校验已下载的 Release 资产内容。
// 职责：按 Release.ValidationChain 逐级下载校验文件，并验证目标资产的 checksum。
// 使用：ApplyRelease 在解压和替换可执行文件前调用，确保资产来自可信 Release 校验链。
// 边界：校验文件缺失、下载失败或 checksum 不匹配都会返回错误；不会自动放行未校验资产。
// 逻辑流：1. 检查 Release 是否提供校验链 => 2. 下载下一层校验文件 => 3. 校验当前资产或校验文件 => 4. 继续校验链直到结束。
func validateReleaseAsset(ctx context.Context, source *giteeSource.Source, release *selfupdate.Release, assetData []byte, checksumName string) error {
	if len(release.ValidationChain) == 0 {
		return fmt.Errorf("validation file not found: %q", checksumName)
	}

	validator := &selfupdate.ChecksumValidator{UniqueFilename: checksumName}
	validationName := release.AssetName
	validationData := assetData
	for _, validationAsset := range release.ValidationChain {
		reader, err := source.DownloadReleaseAsset(ctx, release, validationAsset.ValidationAssetID)
		if err != nil {
			return fmt.Errorf("download validation asset %q: %w", validationAsset.ValidationAssetName, err)
		}

		nextValidationData, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return fmt.Errorf("read validation asset %q: %w", validationAsset.ValidationAssetName, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close validation asset %q: %w", validationAsset.ValidationAssetName, closeErr)
		}

		if err := validator.Validate(validationName, validationData, nextValidationData); err != nil {
			return fmt.Errorf("validate release asset %q: %w", validationName, err)
		}

		validationName = validationAsset.ValidationAssetName
		validationData = nextValidationData
	}

	return nil
}
