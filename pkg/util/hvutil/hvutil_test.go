package hvutil

import "testing"

func TestCreateInstanceName(t *testing.T) {

	namespace := "default"
	pod := "nginx"
	sid := "d38339f99f605f18b4bcbce983147ad2d270ba479668d80b3bfa69a6b0237aa7"
	suffix := namespace + "-" + pod + "-" + sid[:8]

	for node, expected := range map[string]string{
		"worker1":  "podvm-worker1-" + suffix,
		"1.2.3.4":  "podvm-1-2-3-4-" + suffix,
		"worker_1": "podvm-worker-1-" + suffix,
	} {
		actual := CreateInstanceName(node, namespace, pod, sid)
		if actual != expected {
			t.Errorf("expected %s, got %s", expected, actual)
		}
	}
}
