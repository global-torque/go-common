package httputils

import (
	"net/http"
	"strings"

	"github.com/global-torque/go-common/context/v2/keys"
)

func GetIPAddress(headers http.Header) string {
	ip := "127.0.0.1"
	if xOFF := headers.Get(keys.XOriginalForwardedFor); xOFF != "" {
		i := strings.Index(xOFF, ", ")
		if i == -1 {
			i = len(xOFF)
		}
		ip = xOFF[:i]
	} else if xFF := headers.Get(keys.XForwardedFor); xFF != "" {
		i := strings.Index(xFF, ", ")
		if i == -1 {
			i = len(xFF)
		}
		ip = xFF[:i]
	} else if xrIP := headers.Get(keys.XRealIP); xrIP != "" {
		ip = xrIP
	}
	return ip
}
