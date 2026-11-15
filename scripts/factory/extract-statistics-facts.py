"""职责：只读提取量化统计所需能量事实；使用：开发期更新内置目录；边界：不导出游戏实现或美术资源。"""
import argparse
import importlib.util
import json
from pathlib import Path


def main():
    """按物品关联Prefab数值；未知组件布局直接失败，不把解析错误当成零功耗。"""
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--game-dir',required=True,type=Path)
    p.add_argument('--out',required=True,type=Path)
    args=p.parse_args()
    import UnityPy
    spec=importlib.util.spec_from_file_location('facts',Path(__file__).with_name('extract-infrastructure-facts.py'))
    facts=importlib.util.module_from_spec(spec);spec.loader.exec_module(facts)
    m=facts.load_extractor();a=m.Assets(UnityPy,str(args.game_dir/'DSPGAME_Data'))
    _,_,items=m.parse_item_proto_set(a.raw_of('prototypes/itemprotoset'))
    _,_,models=m.parse_model_proto_set(a.raw_of('prototypes/modelprotoset'));models={r['ID']:r for r in models}
    rows=[]
    for item in items:
        row={k:item[k] for k in ('ID','Name','ModelIndex','CanBuild','HeatValue','FuelType','StackSize','IsFluid')}
        row['power']=None
        row['powerKnown']=False
        if item['CanBuild'] and item['ModelIndex']>0:
            prefab=a.game_object(models[item['ModelIndex']]['PrefabPath'])
            if prefab is None:raise ValueError(f'missing prefab {item["ID"]}')
            row['powerKnown']=True
            for node in a.walk(prefab):
                for cr in node['go'].m_Component:
                    c=(cr.component if hasattr(cr,'component') else cr).deref()
                    if c.type!=a.T.MonoBehaviour:continue
                    raw=c.get_raw_data();r=m.Reader(raw);r.p=16
                    sc=a.resolve({'m_FileID':r.i32(),'m_PathID':r.i64()})
                    if sc.read_typetree()['m_Name']!='PowerDesc':continue
                    if row['power'] is not None:raise ValueError('duplicate PowerDesc')
                    r=facts.component_reader(m,raw);v={}
                    v['node']=bool(r.u8_aligned())
                    for _ in range(5):r.f32()
                    for k in ('generator','photovoltaic','wind','gamma','geothermal'):v[k]=bool(r.u8_aligned())
                    for k in ('genEnergyPerTick','useFuelPerTick'):v[k]=r.i64()
                    for k in ('fuelMask','catalystId','catalystMask','productId'):v[k]=r.i32()
                    v['productHeat']=r.i64();v['accumulator']=bool(r.u8_aligned())
                    for k in ('inputEnergyPerTick','outputEnergyPerTick','maxAcuEnergy'):v[k]=r.i64()
                    v['exchanger']=bool(r.u8_aligned());v['exchangeEnergyPerTick']=r.i64()
                    r.i32();r.i32()
                    for k in ('consumer','charger'):v[k]=bool(r.u8_aligned())
                    for k in ('workEnergyPerTick','idleEnergyPerTick'):v[k]=r.i64()
                    r.expect_end('PowerDesc');row['power']=v
        rows.append(row)
    build,_=m.steam_build_id(str(args.game_dir),None)
    result={'schema':'factory-statistics-facts/v1','source':{'gameVersion':m.game_version(str(args.game_dir)),'steamBuildId':build,
        'assemblySha256':m.sha256_of(str(args.game_dir/'DSPGAME_Data/Managed/Assembly-CSharp.dll')),
        'resourcesSha256':m.sha256_of(str(args.game_dir/'DSPGAME_Data/resources.assets'))},'items':rows}
    args.out.parent.mkdir(parents=True,exist_ok=True)
    args.out.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    print('items',len(rows),'powered',sum(r['power'] is not None for r in rows))


if __name__=='__main__':main()
