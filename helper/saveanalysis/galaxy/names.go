package galaxy

// 文件职责：恢复生成时的基础天体名称。
// 使用：按存档命名语言和生成种子运行；边界：玩家自定义名称由存档偏好单独覆盖。
import "fmt"

// starName 职责：生成不重复星名；使用：按恒星顺序执行；边界：保留原有重试上限。
func (s starState) starName(lcid int, used map[string]bool) string {
	r := newRandom(s.nameSeed)
	for i := 0; i < 256; i++ {
		name := candidateName(r.integer(), lcid, s.kind)
		if !used[name] {
			used[name] = true
			return name
		}
	}
	used["XStar"] = true
	return "XStar"
}

// candidateName 职责：选择命名分支；使用：星名重试；边界：固定随机消耗顺序。
func candidateName(seed int32, lcid, kind int) string {
	r := newRandom(seed)
	seed = r.integer()
	roll, giant := r.unit(), r.unit()
	r = newRandom(seed)
	zh := lcid == 2052
	pick := func(en, cn []string) string {
		list := en
		if zh {
			list = cn
		}
		return list[int(r.integer())%len(list)]
	}
	if kind == 3 || kind == 4 {
		sign := "+"
		if r.integer()%2 != 0 {
			sign = "-"
		}
		a, b, c := int(r.unit()*24), int(r.unit()*60), int(r.unit()*60)
		prefix := "NTR"
		if kind == 4 {
			prefix = "DSR"
		}
		return fmt.Sprintf("%s J%02d%02d%s%02d", prefix, a, b, sign, c)
	}
	if kind == 1 {
		if giant < single(.4) {
			return pick(raw_giant_names, raw_giant_names_zhcn)
		}
		if giant < single(.7) {
			n := int(r.integer())
			a, b := int(r.unit()*11)+15, int(r.unit()*26)
			if zh {
				return fmt.Sprintf("%s%c%c", constellations_zhcn[n%len(constellations_zhcn)], a+65, b+65)
			}
			return fmt.Sprintf("%d %s", a+b+130, constellations[n%len(constellations)])
		}
		format := int(r.integer()) % 7
		a, b := int(r.unit()*10000), int(r.unit()*100)
		switch format {
		case 0:
			return fmt.Sprintf("HD %04d%02d", a, b)
		case 1:
			return fmt.Sprintf("HDE %04d%02d", a, b)
		case 2:
			return fmt.Sprintf("HR %04d", a)
		case 3:
			return fmt.Sprintf("HV %04d", a)
		case 4:
			return fmt.Sprintf("LBV %04d-%02d", a, b)
		case 5:
			return fmt.Sprintf("NSV %04d", a)
		default:
			return fmt.Sprintf("YSC %04d-%02d", a, b)
		}
	}
	if roll < single(.6) {
		return pick(raw_star_names, raw_star_names_zhcn)
	}
	n := int(r.integer())
	if roll < single(.93) {
		a := int(r.integer()) % 11
		if zh {
			return constellations_zhcn[int(r.integer())%len(constellations_zhcn)] + alphabeta_letter[a]
		}
		c := constellations[n%len(constellations)]
		alpha := alphabeta[a]
		if len(c) > 10 {
			alpha = alphabeta_letter[a]
		}
		return alpha + " " + c
	}
	number := int(r.unit()*48) + 27
	if zh {
		return fmt.Sprintf("%s %d", constellations_zhcn[int(r.integer())%len(constellations_zhcn)], number)
	}
	return fmt.Sprintf("%d %s", number, constellations[n%len(constellations)])
}
