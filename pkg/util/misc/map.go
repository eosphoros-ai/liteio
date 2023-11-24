package misc

func CopyLabel(in map[string]string) (out map[string]string) {
	out = make(map[string]string)
	for k, v := range in {
		out[k] = v
	}
	return
}
