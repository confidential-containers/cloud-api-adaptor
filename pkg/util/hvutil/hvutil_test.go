package hvutil

import "testing"

func TestCreateInstanceName(t *testing.T) {

	pod := "podname1"
	sid := "d38339f99f605f18b4bcbce983147ad2d270ba479668d80b3bfa69a6b0237aa7"

	if e, a := "podvm-podname1-d38339f9", CreateInstanceName(pod, sid, 0); e != a {
		t.Errorf("expected %s, got %s", e, a)
	}

	if e, a := "podvm-podna-d38339f9", CreateInstanceName(pod, sid, 20); e != a {
		t.Errorf("expected %s, got %s", e, a)
	}
}
