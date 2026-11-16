package build

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

func TestAssemblyFactsMatchSource(t *testing.T) {
	facts, err := loadAssemblyFacts()
	if err != nil {
		t.Fatal(err)
	}
	var models map[int]int16
	var geometry map[int]geometry
	if err = json.Unmarshal(modelData, &models); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(geometryData, &geometry); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(models, facts.models) || !reflect.DeepEqual(geometry, facts.geometry) {
		t.Fatal("cached facts differ from source")
	}
	var sourceSlots map[int]struct {
		Slots []int8 `json:"soltNum"`
	}
	if err = json.Unmarshal(geometryData, &sourceSlots); err != nil {
		t.Fatal(err)
	}
	for kind, source := range sourceSlots {
		if len(source.Slots) != 6 {
			t.Fatalf("kind=%d slots=%d", kind, len(source.Slots))
		}
	}
}

func TestReferenceTemplateConcurrentEquivalence(t *testing.T) {
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for _, source := range []int{2319, 2318, 2317, 2308} {
				template, err := loadReferenceTemplate(source)
				if err != nil {
					t.Error(err)
					continue
				}
				// 独立还原货物目标和扫描原始记录，确保共享缓存保留精确采样及臂顺序。
				targets, err := referenceCargoTargets(template.unit, template.frame)
				if err != nil {
					t.Error(err)
					continue
				}
				if !reflect.DeepEqual(targets, template.cargoTargets) {
					t.Errorf("source=%d cargo mismatch", source)
				}
				for _, machine := range template.unit.MachineIndices {
					indices := []int32{}
					for _, arm := range template.unit.Records {
						if arm.ItemID >= 2011 && arm.ItemID <= 2014 && (arm.InputObjIdx == machine || arm.OutputObjIdx == machine) {
							indices = append(indices, arm.Index)
						}
					}
					actual := []int32{}
					for _, arm := range template.arms[machine] {
						actual = append(actual, arm.Index)
					}
					if !reflect.DeepEqual(indices, actual) {
						t.Errorf("source=%d machine=%d arm mismatch", source, machine)
					}
				}
			}
		}()
	}
	workers.Wait()
}

func BenchmarkAssemblyFactLoading(b *testing.B) {
	b.Run("reparse-each-candidate", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			var models map[int]int16
			var geometry map[int]geometry
			if err := json.Unmarshal(modelData, &models); err != nil {
				b.Fatal(err)
			}
			if err := json.Unmarshal(geometryData, &geometry); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("shared-immutable", func(b *testing.B) {
		if _, err := loadAssemblyFacts(); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := loadAssemblyFacts(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkReferenceTemplateLoading(b *testing.B) {
	template, err := loadReferenceTemplate(2319)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("resample-each-line", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, err := referenceCargoTargets(template.unit, template.frame); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("shared-immutable", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, err := loadReferenceTemplate(2319); err != nil {
				b.Fatal(err)
			}
		}
	})
}
