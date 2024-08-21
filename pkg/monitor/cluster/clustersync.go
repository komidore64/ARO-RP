package cluster

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"context"
)

func (mon *Monitor) emitClusterSync(ctx context.Context) error {
	clusterSync, err := mon.hiveClusterManager.GetSyncSetResources(ctx, mon.doc)
	if err != nil {
		return err
	}
	if clusterSync != nil {
		if clusterSync.Status.SyncSets != nil {
			for _, s := range clusterSync.Status.SyncSets {
				mon.emitGauge("hive.clustersync", 1, map[string]string{
					"kind":               "SyncSets",
					"name":               s.Name,
					"result":             string(s.Result),
					"firstSuccessTime":   s.FirstSuccessTime.String(),
					"lastTransitionTime": s.LastTransitionTime.String(),
					"failureMessage":     s.FailureMessage,
				})
			}
		}
		if clusterSync.Status.SelectorSyncSets != nil {
			for _, s := range clusterSync.Status.SelectorSyncSets {
				mon.emitGauge("hive.clustersync", 1, map[string]string{
					"kind":               "SelectorSyncSets",
					"name":               s.Name,
					"result":             string(s.Result),
					"firstSuccessTime":   s.FirstSuccessTime.String(),
					"lastTransitionTime": s.LastTransitionTime.String(),
					"failureMessage":     s.FailureMessage,
				})
			}
		}
	}
	return nil
}
