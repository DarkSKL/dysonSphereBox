package system

import (
	"fmt"

	"DysonSphereBox/core"
	"DysonSphereBox/utils"
)

type EnvironmentService struct{}

// LoadOrInitDysonSphereExe 加载或者初始化DysonSphereExe 所在路径，如果没有初始会从Steam的环境路径寻找，如果无法找到则返回一个空路径
func (model *EnvironmentService) LoadOrInitDysonSphereExe() (dirPath string, err error) {
	fmt.Println("LoadOrInitDysonSphereExe")
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.GameStartFilePath == nil {
		return "", fmt.Errorf("configure is not initialized")
	}
	fmt.Printf("DIR 1=>:%s\n", core.GLOBAL.Configure.GameStartFilePath)
	configuredPath, err := core.GLOBAL.Configure.GameStartFilePath.Get()
	if err != nil {
		return "", err
	}
	fmt.Printf("DIR 2=>:%s\n", core.GLOBAL.Configure.GameStartFilePath)
	if dirPath := utils.NormalizeDysonSphereDir(configuredPath); dirPath != "" {
		return dirPath, nil
	}
	fmt.Printf("DIR 3=>:%s\n", core.GLOBAL.Configure.GameStartFilePath)
	dirPath, err = utils.FindDysonSphereDirFromSteam()
	if err != nil || dirPath == "" {
		return dirPath, err
	}
	fmt.Printf("DIR 4=>:%s\n", core.GLOBAL.Configure.GameStartFilePath)
	if err := core.GLOBAL.Configure.GameStartFilePath.Set(dirPath); err != nil {
		return "", err
	}
	fmt.Printf("DIR 5=>:%s\n", core.GLOBAL.Configure.GameStartFilePath)
	return dirPath, nil
}

func (model *EnvironmentService) CheckDysonSphereExe(dirPath string) bool {
	return utils.NormalizeDysonSphereDir(dirPath) != ""
}
