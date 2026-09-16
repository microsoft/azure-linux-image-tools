package purl

import (
	"fmt"
	"net/url"
	"strings"
)

func RpmPackageURL(namespace string, name string, version string, release string, arch string, epoch string) string {
	qualifiers := []string{"arch=" + purlEncode(arch)}
	if epoch != "" {
		qualifiers = append(qualifiers, "epoch="+purlEncode(epoch))
	}

	return fmt.Sprintf("pkg:rpm/%s/%s@%s?%s",
		purlEncode(namespace),
		purlEncode(name),
		purlEncode(version+"-"+release),
		strings.Join(qualifiers, "&"))
}

func purlEncode(componentData string) string {
	// QueryEscape encodes spaces as '+', but PURL components require '%20'.
	return strings.ReplaceAll(url.QueryEscape(componentData), "+", "%20")
}
