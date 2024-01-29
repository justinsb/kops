/*
Copyright 2019 The Kubernetes Authors.

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

package fi

import "context"

type ProducesDeletions[T SubContext] interface {
	FindDeletions(*Context[T]) ([]Deletion[T], error)
}

type CloudupProducesDeletions = ProducesDeletions[CloudupSubContext]

type Deletion[T SubContext] interface {
	Delete(ctx context.Context, target Target[T]) error
	TaskName() string
	Item() string
	DeferDeletion() bool
}

type CloudupDeletion = Deletion[CloudupSubContext]

type ResourceInfo struct {
	Type string
	Name string
	ID   string

	DeferDeletion bool
}

type DeletionBase[T SubContext] struct {
	Info ResourceInfo
}

type CloudupDeletionBase = DeletionBase[CloudupSubContext]

func (b *DeletionBase[T]) TaskName() string {
	return b.Info.Type
}

func (b *DeletionBase[T]) Item() string {
	return b.Info.Name
}

func (b *DeletionBase[T]) DeferDeletion() bool {
	return b.Info.DeferDeletion
}
