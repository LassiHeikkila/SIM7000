package module

func maxLength(a, b string) int {
	al := len(a)
	bl := len(b)

	if al >= bl {
		return al
	}
	return bl
}
