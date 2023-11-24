package plugin

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestNodeSelector(t *testing.T) {
	s := labels.NewSelector()
	op, err := convertSelectionOp(corev1.NodeSelectorOpExists)
	if err != nil {
		t.Log(err)
	}
	require, err := labels.NewRequirement("sigma.ali/lock-node", op, nil)
	if err != nil {
		t.Log(err)
	}
	s = s.Add(*require)
	matched := s.Matches(labels.Set(map[string]string{
		"test": "test",
	}))
	t.Log(matched)
}
