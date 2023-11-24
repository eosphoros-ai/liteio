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

package v1

import (
	"context"
	"time"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	scheme "code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// StoragePoolsGetter has a method to return a StoragePoolInterface.
// A group's client should implement this interface.
type StoragePoolsGetter interface {
	StoragePools(namespace string) StoragePoolInterface
}

// StoragePoolInterface has methods to work with StoragePool resources.
type StoragePoolInterface interface {
	Create(ctx context.Context, storagePool *v1.StoragePool, opts metav1.CreateOptions) (*v1.StoragePool, error)
	Update(ctx context.Context, storagePool *v1.StoragePool, opts metav1.UpdateOptions) (*v1.StoragePool, error)
	UpdateStatus(ctx context.Context, storagePool *v1.StoragePool, opts metav1.UpdateOptions) (*v1.StoragePool, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.StoragePool, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.StoragePoolList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.StoragePool, err error)
	StoragePoolExpansion
}

// storagePools implements StoragePoolInterface
type storagePools struct {
	client rest.Interface
	ns     string
}

// newStoragePools returns a StoragePools
func newStoragePools(c *VolumeV1Client, namespace string) *storagePools {
	return &storagePools{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the storagePool, and returns the corresponding storagePool object, and an error if there is any.
func (c *storagePools) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.StoragePool, err error) {
	result = &v1.StoragePool{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("storagepools").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of StoragePools that match those selectors.
func (c *storagePools) List(ctx context.Context, opts metav1.ListOptions) (result *v1.StoragePoolList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.StoragePoolList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("storagepools").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested storagePools.
func (c *storagePools) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("storagepools").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a storagePool and creates it.  Returns the server's representation of the storagePool, and an error, if there is any.
func (c *storagePools) Create(ctx context.Context, storagePool *v1.StoragePool, opts metav1.CreateOptions) (result *v1.StoragePool, err error) {
	result = &v1.StoragePool{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("storagepools").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(storagePool).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a storagePool and updates it. Returns the server's representation of the storagePool, and an error, if there is any.
func (c *storagePools) Update(ctx context.Context, storagePool *v1.StoragePool, opts metav1.UpdateOptions) (result *v1.StoragePool, err error) {
	result = &v1.StoragePool{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("storagepools").
		Name(storagePool.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(storagePool).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *storagePools) UpdateStatus(ctx context.Context, storagePool *v1.StoragePool, opts metav1.UpdateOptions) (result *v1.StoragePool, err error) {
	result = &v1.StoragePool{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("storagepools").
		Name(storagePool.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(storagePool).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the storagePool and deletes it. Returns an error if one occurs.
func (c *storagePools) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("storagepools").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *storagePools) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("storagepools").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched storagePool.
func (c *storagePools) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.StoragePool, err error) {
	result = &v1.StoragePool{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("storagepools").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
