package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	mock_hive "github.com/Azure/ARO-RP/pkg/util/mocks/hive"
	mock_metrics "github.com/Azure/ARO-RP/pkg/util/mocks/metrics"
	"github.com/golang/mock/gomock"
	hivev1alpha1 "github.com/openshift/hive/apis/hiveinternal/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	// Register the ClusterSync type with the scheme
	hivev1alpha1.AddToScheme(scheme.Scheme)
}

func TestEmitSyncSetStatus(t *testing.T) {
	for _, tt := range []struct {
		name              string
		clusterSync       *hivev1alpha1.ClusterSync
		getClusterSyncErr error
		expectedError     error
		expectedGauges    []struct {
			name   string
			value  int64
			labels map[string]string
		}
	}{
		{
			name: "SyncSets and SelectorSyncSets have elements",
			clusterSync: &hivev1alpha1.ClusterSync{
				Status: hivev1alpha1.ClusterSyncStatus{
					SyncSets: []hivev1alpha1.SyncStatus{
						{
							Name:               "syncset1",
							Result:             "Success",
							FirstSuccessTime:   &metav1.Time{Time: time.Now()},
							LastTransitionTime: metav1.Time{Time: time.Now()},
							FailureMessage:     "",
						},
					},
					SelectorSyncSets: []hivev1alpha1.SyncStatus{
						{
							Name:               "selectorsyncset1",
							Result:             "Success",
							FirstSuccessTime:   &metav1.Time{Time: time.Now()},
							LastTransitionTime: metav1.Time{Time: time.Now()},
							FailureMessage:     "",
						},
					},
				},
			},
			expectedError: nil,
			expectedGauges: []struct {
				name   string
				value  int64
				labels map[string]string
			}{
				{
					name:  "hive.syncsets",
					value: 1,
					labels: map[string]string{
						"name":               "syncset1",
						"result":             "Success",
						"firstSuccessTime":   time.Now().Format(time.RFC3339),
						"lastTransitionTime": time.Now().Format(time.RFC3339),
						"failureMessage":     "",
					},
				},
				{
					name:  "hive.selectorsyncsets",
					value: 1,
					labels: map[string]string{
						"name":               "selectorsyncset1",
						"result":             "Success",
						"firstSuccessTime":   time.Now().Format(time.RFC3339),
						"lastTransitionTime": time.Now().Format(time.RFC3339),
						"failureMessage":     "",
					},
				},
				{
					name:  "hive.clustersync",
					value: 1,
					labels: map[string]string{
						"name":               "selectorsyncset1",
						"result":             "Success",
						"firstSuccessTime":   time.Now().Format(time.RFC3339),
						"lastTransitionTime": time.Now().Format(time.RFC3339),
						"failureMessage":     "",
					},
				},
			},
		},
		{
			name: "SyncSets and SelectorSyncSets are nil",
			clusterSync: &hivev1alpha1.ClusterSync{
				Status: hivev1alpha1.ClusterSyncStatus{
					SyncSets:         nil,
					SelectorSyncSets: nil,
				},
			},
			expectedError: nil,
			expectedGauges: []struct {
				name   string
				value  int64
				labels map[string]string
			}{
				{
					name:   "hive.syncsets",
					value:  0,
					labels: map[string]string{},
				},
				{
					name:   "hive.selectorsyncsets",
					value:  0,
					labels: map[string]string{},
				},
				{
					name:   "hive.clustersync",
					value:  0,
					labels: map[string]string{},
				},
			},
		},
		{
			name:              "GetClusterSyncforClusterDeployment returns error",
			getClusterSyncErr: errors.New("some error"),
			expectedError:     nil,
			expectedGauges: []struct {
				name   string
				value  int64
				labels map[string]string
			}{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockHiveClusterManager := mock_hive.NewMockClusterManager(ctrl)
			m := mock_metrics.NewMockEmitter(ctrl)
			mockMonitor := &Monitor{
				hiveClusterManager: mockHiveClusterManager,
				m:                  m,
			}

			ctx := context.Background()

			mockHiveClusterManager.EXPECT().GetSyncSetResources(ctx, mockMonitor.doc).Return(tt.clusterSync, tt.getClusterSyncErr).AnyTimes()

			for _, gauge := range tt.expectedGauges {
				m.EXPECT().EmitGauge(gauge.name, gauge.value, gauge.labels).Times(1)
			}

			err := mockMonitor.emitSyncSetStatus(ctx)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}
