package httplimit

import "testing"

func TestClientIPTrustAndNormalization(t *testing.T) {
	cases := []struct{ mode, header, peer, want string }{
		{"direct", "198.51.100.1", "192.0.2.1", "192.0.2.1"},
		{"cloud_run", "spoof, 198.51.100.2", "192.0.2.1", "198.51.100.2"},
		{"cloud_run", "198.51.100.1, invalid", "192.0.2.1", "192.0.2.1"},
		{"cloud_run", "", "192.0.2.1", "192.0.2.1"},
		{"cloud_run", "::ffff:198.51.100.2", "192.0.2.1", "198.51.100.2"},
		{"cloud_run", "2001:db8:0:0::1", "192.0.2.1", "2001:db8::1"},
		{"cloud_run", "fe80::1%eth0", "192.0.2.1", "192.0.2.1"},
	}
	for _, tc := range cases {
		if got := resolveIP(tc.mode, tc.header, tc.peer); got != tc.want {
			t.Errorf("resolveIP(%q, %q) = %q, want %q", tc.mode, tc.header, got, tc.want)
		}
	}
}

func TestClientIPModeRequiresCloudRun(t *testing.T) {
	t.Setenv("K_SERVICE", "")
	if err := ValidateClientIPMode("cloud_run"); err == nil {
		t.Fatal("accepted non-Cloud-Run deployment")
	}
	if err := ValidateClientIPMode("direct"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateClientIPMode("any_proxy"); err == nil {
		t.Fatal("accepted unknown mode")
	}
	t.Setenv("K_SERVICE", "test")
	if err := ValidateClientIPMode("cloud_run"); err != nil {
		t.Fatal(err)
	}
}
