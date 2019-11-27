// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2019 Datadog, Inc.

// +build windows

package windowsevent

// tailerForIndex returns the tailer which maps to the specified index
func tailerForIndex(id int) (*Tailer, bool) {
	lock.RLock()
	defer lock.RUnlock()
	tailer, exists := eventContextToTailerMap[id]
	return tailer, exists
}
