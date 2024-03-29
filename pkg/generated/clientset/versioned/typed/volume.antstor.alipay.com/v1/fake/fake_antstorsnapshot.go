/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeAntstorSnapshots implements AntstorSnapshotInterface
type FakeAntstorSnapshots struct {
	Fake *FakeVolumeV1
	ns   string
}

var antstorsnapshotsResource = v1.SchemeGroupVersion.WithResource("antstorsnapshots")

var antstorsnapshotsKind = v1.SchemeGroupVersion.WithKind("AntstorSnapshot")

// Get takes name of the antstorSnapshot, and returns the corresponding antstorSnapshot object, and an error if there is any.
func (c *FakeAntstorSnapshots) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.AntstorSnapshot, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(antstorsnapshotsResource, c.ns, name), &v1.AntstorSnapshot{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.AntstorSnapshot), err
}

// List takes label and field selectors, and returns the list of AntstorSnapshots that match those selectors.
func (c *FakeAntstorSnapshots) List(ctx context.Context, opts metav1.ListOptions) (result *v1.AntstorSnapshotList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(antstorsnapshotsResource, antstorsnapshotsKind, c.ns, opts), &v1.AntstorSnapshotList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.AntstorSnapshotList{ListMeta: obj.(*v1.AntstorSnapshotList).ListMeta}
	for _, item := range obj.(*v1.AntstorSnapshotList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested antstorSnapshots.
func (c *FakeAntstorSnapshots) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(antstorsnapshotsResource, c.ns, opts))

}

// Create takes the representation of a antstorSnapshot and creates it.  Returns the server's representation of the antstorSnapshot, and an error, if there is any.
func (c *FakeAntstorSnapshots) Create(ctx context.Context, antstorSnapshot *v1.AntstorSnapshot, opts metav1.CreateOptions) (result *v1.AntstorSnapshot, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(antstorsnapshotsResource, c.ns, antstorSnapshot), &v1.AntstorSnapshot{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.AntstorSnapshot), err
}

// Update takes the representation of a antstorSnapshot and updates it. Returns the server's representation of the antstorSnapshot, and an error, if there is any.
func (c *FakeAntstorSnapshots) Update(ctx context.Context, antstorSnapshot *v1.AntstorSnapshot, opts metav1.UpdateOptions) (result *v1.AntstorSnapshot, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(antstorsnapshotsResource, c.ns, antstorSnapshot), &v1.AntstorSnapshot{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.AntstorSnapshot), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeAntstorSnapshots) UpdateStatus(ctx context.Context, antstorSnapshot *v1.AntstorSnapshot, opts metav1.UpdateOptions) (*v1.AntstorSnapshot, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(antstorsnapshotsResource, "status", c.ns, antstorSnapshot), &v1.AntstorSnapshot{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.AntstorSnapshot), err
}

// Delete takes name of the antstorSnapshot and deletes it. Returns an error if one occurs.
func (c *FakeAntstorSnapshots) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(antstorsnapshotsResource, c.ns, name, opts), &v1.AntstorSnapshot{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAntstorSnapshots) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(antstorsnapshotsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1.AntstorSnapshotList{})
	return err
}

// Patch applies the patch and returns the patched antstorSnapshot.
func (c *FakeAntstorSnapshots) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.AntstorSnapshot, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(antstorsnapshotsResource, c.ns, name, pt, data, subresources...), &v1.AntstorSnapshot{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.AntstorSnapshot), err
}
