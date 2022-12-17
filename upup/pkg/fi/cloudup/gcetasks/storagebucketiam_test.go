/*
Copyright 2021 The Kubernetes Authors.

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

package gcetasks

import (
	"context"
	"testing"

	"google.golang.org/api/storage/v1"
	gcemock "k8s.io/kops/cloudmock/gce"
	"k8s.io/kops/upup/pkg/fi"
)

func TestStorageBucketIAM(t *testing.T) {
	ctx := context.TODO()

	project := "testproject"
	region := "us-test1"

	cloud := gcemock.InstallMockGCECloud(region, project)

	if _, err := cloud.Storage().Buckets.Insert(project, &storage.Bucket{
		Name: "bucket1",
	}).Context(ctx).Do(); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	// We define a function so we can rebuild the tasks, because we modify in-place when running
	buildTasks := func() map[string]fi.Task {
		binding := &StorageBucketIAM{
			Lifecycle: fi.LifecycleSync,

			Bucket: fi.PtrTo("bucket1"),
			Member: fi.PtrTo("serviceAccount:foo@testproject.iam.gserviceaccount.com"),
			Role:   fi.PtrTo("roles/owner"),
		}

		return map[string]fi.Task{
			"binding": binding,
		}
	}

	{
		allTasks := buildTasks()
		checkHasChanges(t, ctx, cloud, allTasks)
	}

	{
		allTasks := buildTasks()
		runTasks(t, ctx, cloud, allTasks)
	}

	{
		allTasks := buildTasks()
		checkNoChanges(t, ctx, cloud, allTasks)
	}
}
