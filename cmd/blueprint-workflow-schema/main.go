package main

// 文件职责：从实际生产工具目录导出工作流JSON Schema。
// 使用：在client根执行go run ./cmd/blueprint-workflow-schema；边界：只输出Schema，不运行蓝图生成、不修改用户配置。

import (
	"flag"
	"fmt"
	"os"

	"DysonSphereBox/helper/blueprint/build/generator"
)

// main 职责：处理显式输出路径或标准输出；使用：配置编辑器及文档生成；边界：目录必须由调用者准备，任何写入失败以非零退出。
func main() {
	output := flag.String("out", "", "输出文件路径；缺省写入标准输出")
	flag.Parse()
	data, err := generator.ConfigurationSchema()
	if err == nil {
		if *output == "" {
			_, err = os.Stdout.Write(data)
		} else {
			err = os.WriteFile(*output, append(data, '\n'), 0644)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
