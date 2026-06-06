package system

import "fmt"

type EnvironmentService struct{}

//func NewEnvironmentService() *EnvironmentService {}

func (model *EnvironmentService) CheckDysonSphereExe(dirPath string) {
	fmt.Println("Checking dyson sphere exe ...")
}
