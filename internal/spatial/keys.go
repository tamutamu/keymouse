package spatial

// GridKeys are the label keys. Their order defines the base-ten labels.
// H/J/K/L stay reserved for fine grid movement.
var GridKeys = []Key{0x41, 0x53, 0x44, 0x46, 0x47, 0x54, 0x52, 0x45, 0x57, 0x51} // A S D F G T R E W Q
var LabelKeys = GridKeys

func IsGridKey(k Key) bool {
	for _, v := range GridKeys {
		if k == v {
			return true
		}
	}
	return false
}
func KeyToChar(k Key) string {
	switch k {
	case 0x52:
		return "R"
	case 0x41:
		return "A"
	case 0x53:
		return "S"
	case 0x44:
		return "D"
	case 0x46:
		return "F"
	case 0x47:
		return "G"
	case 0x54:
		return "T"
	case 0x45:
		return "E"
	case 0x57:
		return "W"
	case 0x51:
		return "Q"
	}
	return ""
}
func GenerateLabel3s(keys []Key) []Label3 {
	r := make([]Label3, 0, len(keys)*len(keys)*len(keys))
	for _, a := range keys {
		for _, b := range keys {
			for _, c := range keys {
				r = append(r, Label3{a, b, c})
			}
		}
	}
	return r
}
func Label3ToStr(l Label3) string {
	return KeyToChar(l[0]) + KeyToChar(l[1]) + KeyToChar(l[2])
}
