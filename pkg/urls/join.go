package urls

import "strings"

func Join(base string, others ...string) string {
	u := base
	for _, o := range others {
		if !strings.HasSuffix(u, "/") {
			u += "/"
		}
		if strings.HasPrefix(o, "/") {
			u += o[1:]
		} else {
			u += o
		}
	}
	return u
}
