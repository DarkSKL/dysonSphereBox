package build
import("context";"encoding/json";"os";"testing";"DysonSphereBox/helper/blueprint")
func TestCase21StoredGeometryRules(t *testing.T){
 raw,err:=os.ReadFile("D:/Workspace/DysonSphere/DysonSphereBox/develop/.local/blueprint-core-refactor/after-surface-sharing/case-21.json");if err!=nil{t.Fatal(err)}
 var v struct{Request Request;Result Result};if err=json.Unmarshal(raw,&v);err!=nil{t.Fatal(err)}
 doc,err:=blueprint.Decode(v.Result.Blueprint,true);if err!=nil{t.Fatal(err)}
 enc:=&surfaceEncoding{rootCoordinates:true,areas:doc.Areas,plan:v.Result.SurfacePlan}
 req:=v.Request;req.SurfaceLayout.polarRoot=true
 t.Log("latitude",validateDensityLatitudePlan(req.SurfaceLayout,enc.plan))
 _,err=validateFinalPlanetRules(context.Background(),req,enc,doc.Buildings);t.Log("planet",err)
 t.Log("relocated",validateRelocatedSurface(context.Background(),req,enc,doc.Buildings))
 t.Log("actual-latitude-corners",enc.validateDenseStaticLatitude(context.Background(),req.SurfaceLayout,doc.Buildings))
}
