//go:build ignore

package main

// 文件职责：提供只读/纯计算的本地验证桥接；使用：go run scripts/factory/verify-server.go；边界：仅监听回环，不初始化数据库、不写游戏/资源库，不能用于生产。

import (
	"DysonSphereBox/helper/blueprint/build"
	factoryHelper "DysonSphereBox/helper/factory"
	contract "DysonSphereBox/helper/factory/contract"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/service/factory"
	"encoding/json"
	"fmt"
	"net/http"
)

// main 职责：启动隔离验证端点；使用：本地浏览器测试；边界：退出后关闭，不提供路径读写。
func main() {
	s := &factory.Service{}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			return
		}
		if r.Method != "POST" {
			http.Error(w, "POST only", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
		var result interface{}
		switch r.URL.Path {
		case "/ListDatasets":
			result = s.ListDatasets()
		case "/DescribeDataset":
			var key string
			json.NewDecoder(r.Body).Decode(&key)
			result = s.DescribeDataset(key)
		case "/Solve":
			var req quant.Request
			json.NewDecoder(r.Body).Decode(&req)
			result = s.Solve(req)
		case "/Generate":
			var req build.Request
			json.NewDecoder(r.Body).Decode(&req)
			result = s.Generate(req)
		case "/Inspect":
			var req contract.InspectRequest
			json.NewDecoder(r.Body).Decode(&req)
			result = s.Inspect(req)
		case "/ReadBlueprintChunk":
			var req contract.ChunkRequest
			json.NewDecoder(r.Body).Decode(&req)
			result = s.ReadBlueprintChunk(req)
		case "/WorkspaceRecord":
			var req contract.WorkspaceRequest
			json.NewDecoder(r.Body).Decode(&req)
			text, err := factoryHelper.MarshalWorkspace(req)
			result = map[string]interface{}{"code": factoryHelper.Status(err).Code, "data": text}
		default:
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(result)
	})
	fmt.Println("isolated factory verification at 127.0.0.1:5174")
	if err := http.ListenAndServe("127.0.0.1:5174", nil); err != nil {
		panic(err)
	}
}
