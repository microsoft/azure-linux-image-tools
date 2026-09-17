package purl

import (
	"fmt"
	"net/url"
	"strings"
)

func RpmPackageURL(namespace string, name string, epoch *int, version string, release string, arch string) string {
	qualifiers := []string{"arch=" + purlEncode(arch)}
	if epoch != nil {
		qualifiers = append(qualifiers, fmt.Sprintf("epoch=%d", *epoch))
	}

	return fmt.Sprintf("pkg:rpm/%s/%s@%s?%s",
		purlEncode(namespace),
		purlEncode(name),
		purlEncode(version+"-"+release),
		strings.Join(qualifiers, "&"))
}

func purlEncode(componentData string) string {
	// PUL component data must encode '+' and '@', but PathEscape leaves them unescaped, so it can't be used.
	// QueryEscape encodes both, but uses '+' for spaces and '%3A' for colons,
	// but  PURL needs the reverse encoding: '%20' for spaces and a literal ':' for colons (ECMA-427, section 5.4).
	return strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(componentData), "+", "%20"), "%3A", ":")
}
