// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2019 Datadog, Inc.

// +build kubelet

package collectors

import (
	"strings"
	"time"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/errors"
	"github.com/DataDog/datadog-agent/pkg/util/kubernetes/kubelet"
)

const (
	kubeletCollectorName = "kubelet"
	kubeletExpireFreq    = 5 * time.Minute
)

// KubeletCollector connects to the local kubelet to get kubernetes container
// tags. It is to be supplemented by the cluster agent collector for tags from
// the apiserver.
type KubeletCollector struct {
	watcher           *kubelet.PodWatcher
	infoOut           chan<- []*TagInfo
	lastExpire        time.Time
	expireFreq        time.Duration
	labelsAsTags      map[string]string
	annotationsAsTags map[string]string
}

// Detect tries to connect to the kubelet
func (c *KubeletCollector) Detect(out chan<- []*TagInfo) (CollectionMode, error) {
	watcher, err := kubelet.NewPodWatcher(5*time.Minute, true)
	if err != nil {
		return NoCollection, err
	}
	c.watcher = watcher
	c.infoOut = out
	c.lastExpire = time.Now()
	c.expireFreq = kubeletExpireFreq

	// We lower-case the values collected by viper as well as the ones from inspecting the labels of containers.
	labelsList := config.Datadog.GetStringMapString("kubernetes_pod_labels_as_tags")
	for label, value := range labelsList {
		delete(labelsList, label)
		labelsList[strings.ToLower(label)] = value
	}
	c.labelsAsTags = labelsList

	annotationsList := config.Datadog.GetStringMapString("kubernetes_pod_annotations_as_tags")
	for annotation, value := range annotationsList {
		delete(annotationsList, annotation)
		annotationsList[strings.ToLower(annotation)] = value
	}
	c.annotationsAsTags = annotationsList
	return PullCollection, nil
}

// Pull triggers a podlist refresh and sends new info. It also triggers
// container deletion computation every 'expireFreq'
func (c *KubeletCollector) Pull() error {
	// Compute new/updated pods
	updatedPods, err := c.watcher.PullChanges()
	if err != nil {
		return err
	}

	updates, err := c.parsePods(updatedPods)
	if err != nil {
		return err
	}
	c.infoOut <- updates

	// Throttle deletion computations
	if time.Now().Sub(c.lastExpire) < c.expireFreq {
		return nil
	}

	// Compute deleted pods
	expireList, err := c.watcher.Expire()
	if err != nil {
		return err
	}
	expiries, err := c.parseExpires(expireList)
	if err != nil {
		return err
	}
	c.infoOut <- expiries
	c.lastExpire = time.Now()
	return nil
}

// Fetch fetches tags for a given entity by iterating on the whole podlist
// TODO: optimize if called too often on production
func (c *KubeletCollector) Fetch(entity string) ([]string, []string, []string, error) {
	pod, err := c.watcher.GetPodForEntityID(entity)
	if err != nil {
		return []string{}, []string{}, []string{}, err
	}

	pods := []*kubelet.Pod{pod}
	updates, err := c.parsePods(pods)
	if err != nil {
		return []string{}, []string{}, []string{}, err
	}
	c.infoOut <- updates

	for _, info := range updates {
		if info.Entity == entity {
			return info.LowCardTags, info.OrchestratorCardTags, info.HighCardTags, nil
		}
	}
	// entity not found in updates
	return []string{}, []string{}, []string{}, errors.NewNotFound(entity)
}

// parseExpires transforms event from the PodWatcher to TagInfo objects
func (c *KubeletCollector) parseExpires(idList []string) ([]*TagInfo, error) {
	var output []*TagInfo
	for _, id := range idList {
		info := &TagInfo{
			Source:       kubeletCollectorName,
			Entity:       id,
			DeleteEntity: true,
		}
		output = append(output, info)
	}
	return output, nil
}

func kubeletFactory() Collector {
	return &KubeletCollector{}
}

func init() {
	registerCollector(kubeletCollectorName, kubeletFactory, NodeOrchestrator)
}
