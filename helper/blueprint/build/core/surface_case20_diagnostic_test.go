package build
import("context";"encoding/json";"os";"testing";"DysonSphereBox/helper/quant")
func TestSurfaceCase20Diagnostic(t *testing.T){
 raw,e:=os.ReadFile("D:/Workspace/DysonSphere/DysonSphereBox/develop/.local/blueprint-core-refactor/random/seed-20261116-20261116-122055/case-20.json");if e!=nil{t.Fatal(e)}
 var v struct{Request Request};if e=json.Unmarshal(raw,&v);e!=nil{t.Fatal(e)}
 d,e:=quant.LoadDataset("Vanilla");if e!=nil{t.Fatal(e)}
 input:=stationTestInput(t,d,v.Request.Plan,v.Request.Density,v.Request.Mode,false,false)
 input.request=v.Request;input.request.Setting,e=normalizeSetting(v.Request.Setting);if e!=nil{t.Fatal(e)}
 input.request.SurfaceLayout,e=normalizeSurfaceLayout(v.Request.SurfaceLayout);if e!=nil{t.Fatal(e)}
 for _,expand:=range []bool{true,false}{
  options:=DefaultSurfaceSearchOptions();options.ExpandAdjacent=expand
  surfaces,e:=EnumerateSurfaceCandidates(context.Background(),input,options);if e!=nil{t.Fatal(e)}
  for _,surface:=range surfaces {
   if surface.Strategy!="native-window" {continue}
   prepared,e:=PrepareSurfaceCandidate(context.Background(),input,surface);if e!=nil{t.Fatal(e)}
   ctx:=prepared.algorithmContext(context.Background())
   specs,e:=EnumerateStationCandidates(ctx,prepared,StationSearchOptions{Family:"blackbox"});if e!=nil{t.Fatal(e)}
   b,e:=assembleStationCandidate(ctx,prepared,specs[0],nil)
   if b!=nil&&b.polarFrame!=nil{t.Logf("spec=%+v layoutExpand=%t frame=%+v error=%v",surface,prepared.request.SurfaceLayout.ExpandAdjacent,*b.polarFrame,e)}else{t.Logf("spec=%+v layoutExpand=%t error=%v",surface,prepared.request.SurfaceLayout.ExpandAdjacent,e)}
  }
 }
}
