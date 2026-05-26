package runtime_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseK8sID_valid(t *testing.T) {
	cases := []struct {
		id        string
		wantNS    string
		wantPod   string
		wantCtr   string
	}{
		{"default/mypod/mycontainer", "default", "mypod", "mycontainer"},
		{"kube-system/coredns-abc/coredns", "kube-system", "coredns-abc", "coredns"},
	}
	for _, tc := range cases {
		parts := strings.SplitN(tc.id, "/", 3)
		assert.Len(t, parts, 3)
		assert.Equal(t, tc.wantNS, parts[0])
		assert.Equal(t, tc.wantPod, parts[1])
		assert.Equal(t, tc.wantCtr, parts[2])
	}
}

func TestParseK8sID_invalid(t *testing.T) {
	cases := []string{
		"",
		"noSlash",
		"one/two",
	}
	for _, id := range cases {
		parts := strings.SplitN(id, "/", 3)
		assert.Less(t, len(parts), 3, "should have < 3 parts for id=%q", id)
	}
}
