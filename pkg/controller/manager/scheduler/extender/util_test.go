package extender

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPatch(t *testing.T) {
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cm1",
			Labels: map[string]string{"key": "value"},
		},
	}

	cm2 := cm1.DeepCopy()
	cm2.Labels["key2"] = "value2"
	cm2.Data = map[string]string{"test": "value"}

	var patch = client.MergeFrom(cm1)
	data, err := patch.Data(cm2)
	t.Log(err, string(data))

}
