package messagix

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
)

// LoggerTransport is useful for debugging purposes of header
type LoggerTransport struct {
	Transport http.RoundTripper
}

// RoundTrip executes a single HTTP transaction and logs the request and response
func (c *LoggerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log the request

	requestDump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		fmt.Println("Error dumping request: ", err)
	} else {
		fmt.Println("Request: ", string(requestDump))
	}

	// Perform the actual request
	resp, err := c.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp, err
}

type ProxyCookieRemoveTransport struct {
	Transport http.RoundTripper
}

func (c *ProxyCookieRemoveTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Modify request cookies to remove 'provided_proxy_url'
	// Directly modify the cookies in the request header
	if cookieHeader := req.Header.Get("Cookie"); cookieHeader != "" {
		// Split the cookie header on the ';' to handle each cookie individually
		regex := regexp.MustCompile(`;\s*`)
		// Split the cookie header using the regex
		cookies := regex.Split(cookieHeader, -1)
		var modifiedCookies []string

		for _, cookie := range cookies {
			if !strings.Contains(cookie, "provided_proxy_url=") {
				modifiedCookies = append(modifiedCookies, cookie)
			}
		}

		// Join the modified cookies and set the cookie header
		req.Header.Set("Cookie", strings.Join(modifiedCookies, "; "))
	}

	// Perform the actual request
	resp, err := c.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
