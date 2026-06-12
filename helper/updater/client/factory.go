package client

// 文件职责：创建应用自更新需要的 go-selfupdate 更新器和 Gitee 更新源。
// 使用：updater 根包在检查 Release 或应用更新前调用 New。
// 边界：只完成对象构造，不访问远端 Release、不下载资产、不读取当前版本。

import (
	giteeSource "DysonSphereBox/helper/updater/gitee/source"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// New 创建 go-selfupdate 更新器和 Gitee 更新源。
// 职责：集中组装自更新所需的 Source 与 checksum Validator。
// 使用：updater.CheckAppUpdate 和 updater.UpdateAppSelf 在执行检测或应用更新前调用。
// 边界：只完成对象构造，不访问远端 Release，也不下载资产。
func New(apiBaseURL string, checksumName string) (*selfupdate.Updater, *giteeSource.Source, error) {
	source, err := giteeSource.NewSource(giteeSource.Config{APIBaseURL: apiBaseURL})
	if err != nil {
		return nil, nil, err
	}
	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source, Validator: &selfupdate.ChecksumValidator{UniqueFilename: checksumName}})
	if err != nil {
		return nil, nil, err
	}
	return updater, source, nil
}
