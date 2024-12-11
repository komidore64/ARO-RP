package cluster

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apisubnet "github.com/Azure/ARO-RP/pkg/api/util/subnet"
	arov1alpha1 "github.com/Azure/ARO-RP/pkg/operator/apis/aro.openshift.io/v1alpha1"
)

const (
	cwp                  = "clusterWideProxy.status"
	cwpErrorMessage      = "NoProxy entries are incorrect"
	cluster              = "cluster"
	mandatory_no_proxies = "localhost,127.0.0.1,.svc,.cluster.local.,168.63.129.16,169.254.169.254"
	//169.254.169.254 (the IMDS IP)
	//168.63.129.16 (Azure DNS, if no custom DNS exists)
	//localhost, 127.0.0.1, .svc, .cluster.local
)

// Helper function to emit and log the gauge status
func (mon *Monitor) emitAndLogCWPStatus(status bool, message string) {
	mon.emitGauge(cwp, 1, map[string]string{
		"status":  strconv.FormatBool(status),
		"Message": message,
	})

	if mon.hourlyRun {
		mon.log.WithFields(logrus.Fields{
			"metric":  cwp,
			"status":  strconv.FormatBool(status),
			"Message": message,
		}).Print()
	}
}

// Main function to emit CWP status
func (mon *Monitor) emitCWPStatus(ctx context.Context) error {
	mon.hourlyRun = true
	proxyConfig, err := mon.configcli.ConfigV1().Proxies().Get(ctx, cluster, metav1.GetOptions{})
	if err != nil {
		mon.log.Errorf("Error in getting the cluster wide proxy: %v", err)
		return err
	}
	if proxyConfig.Spec.HTTPProxy == "" && proxyConfig.Spec.HTTPSProxy == "" && proxyConfig.Spec.NoProxy == "" {
		mon.emitAndLogCWPStatus(false, "CWP not enabled")
	} else {
		// Create the noProxy map for efficient lookups
		no_proxy_list := strings.Split(proxyConfig.Spec.NoProxy, ",")
		noProxyMap := make(map[string]bool)
		for _, proxy := range no_proxy_list {
			noProxyMap[proxy] = true
		}

		// Check mandatory no_proxy entries
		for _, mandatory_no_proxy := range strings.Split(mandatory_no_proxies, ",") {
			if !noProxyMap[mandatory_no_proxy] {
				mon.emitAndLogCWPStatus(true, "CWP enabled but missing "+mandatory_no_proxy+" in the no_proxy list")
			}
		}

		// Azure credentials and client setup
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			mon.log.Errorf("failed to obtain a credential: %v", err)
			return err
		}

		mastersubnetID, err := azure.ParseResourceID(mon.oc.Properties.MasterProfile.SubnetID)
		if err != nil {
			return err
		}

		// Create client factory
		clientFactory, err := armnetwork.NewClientFactory(mastersubnetID.SubscriptionID, cred, nil)
		if err != nil {
			mon.log.Errorf("failed to create client: %v", err)
			return err
		}

		// Check master subnet
		masterVnetID, _, err := apisubnet.Split(mon.oc.Properties.MasterProfile.SubnetID)
		if err != nil {
			return err
		}
		mastervnetId, err := azure.ParseResourceID(masterVnetID)
		if err != nil {
			return err
		}
		res, err := clientFactory.NewSubnetsClient().Get(ctx, mastersubnetID.ResourceGroup, mastervnetId.ResourceName, mastersubnetID.ResourceName, &armnetwork.SubnetsClientGetOptions{Expand: nil})
		if err != nil {
			mon.log.Errorf("failed to finish the request: %v", err)
			return err
		}
		mastermachineCIDR := *res.Properties.AddressPrefix
		if !noProxyMap[mastermachineCIDR] {
			mon.emitAndLogCWPStatus(true, "CWP enabled but missing master machineCIDR "+mastermachineCIDR+" in the no_proxy list")
		}

		// Check worker profiles
		for _, workerProfile := range mon.oc.Properties.WorkerProfiles {
			workersubnetID, err := azure.ParseResourceID(workerProfile.SubnetID)
			if err != nil {
				return err
			}
			workerVnetID, _, err := apisubnet.Split(workerProfile.SubnetID)
			if err != nil {
				return err
			}
			workervnetId, err := azure.ParseResourceID(workerVnetID)
			if err != nil {
				return err
			}
			workerres, err := clientFactory.NewSubnetsClient().Get(ctx, workersubnetID.ResourceGroup, workervnetId.ResourceName, workersubnetID.ResourceName, &armnetwork.SubnetsClientGetOptions{Expand: nil})
			if err != nil {
				mon.log.Errorf("failed to finish the request: %v", err)
			}
			workermachinesCIDR := *workerres.Properties.AddressPrefix
			if !noProxyMap[workermachinesCIDR] {
				mon.emitAndLogCWPStatus(true, "CWP enabled but missing Worker machine CIDR "+workermachinesCIDR+" in the no_proxy list")
			}
		}

		// Network Configuration Check
		networkConfig, err := mon.configcli.ConfigV1().Networks().Get(ctx, cluster, metav1.GetOptions{})
		if err != nil {
			mon.log.Errorf("Error in getting network info: %v", err)
			return err
		}
		for _, network := range networkConfig.Spec.ClusterNetwork {
			if !noProxyMap[network.CIDR] {
				mon.emitAndLogCWPStatus(true, "CWP enabled but missing PodCIDR "+network.CIDR+" in the no_proxy list")
			}
		}
		for _, network := range networkConfig.Spec.ServiceNetwork {
			if !noProxyMap[network] {
				mon.emitAndLogCWPStatus(true, "CWP enabled but missing ServiceCIDR "+network+" in the no_proxy list")
			}
		}

		// Gateway Domains Check
		clusterdetails, err := mon.arocli.AroV1alpha1().Clusters().Get(ctx, arov1alpha1.SingletonClusterName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, gatewayDomain := range clusterdetails.Spec.GatewayDomains {
			if !noProxyMap[gatewayDomain] {
				mon.emitAndLogCWPStatus(true, "CWP enabled but missing Gateway domain "+gatewayDomain+" in the no_proxy list")
			}
		}

		// Infrastructure Configuration Check
		infraConfig, err := mon.configcli.ConfigV1().Infrastructures().Get(ctx, cluster, metav1.GetOptions{})
		if err != nil {
			mon.log.Errorf("Error in getting network info: %v", err)
			return err
		}

		// APIServerInternal URL Check
		apiServerIntURL, err := url.Parse(infraConfig.Status.APIServerInternalURL)
		if err != nil {
			return err
		}
		apiServerIntdomain := strings.Split(apiServerIntURL.Host, ":")[0]
		if !noProxyMap[apiServerIntdomain] {
			mon.emitAndLogCWPStatus(true, "CWP enabled but missing APIServerInternal URL "+apiServerIntdomain+" in the no_proxy list")
		}

		// APIServerProfile URL Check
		apiServerProfileURL, err := url.Parse(mon.oc.Properties.APIServerProfile.URL)
		if err != nil {
			return err
		}
		apiServerProfiledomain := strings.Split(apiServerProfileURL.Host, ":")[0]
		if !noProxyMap[apiServerProfiledomain] {
			mon.emitAndLogCWPStatus(true, "CWP enabled but missing APiServerProfile URL "+apiServerProfiledomain+" in the no_proxy list")
		}

		// ConsoleProfile URL Check
		consolProfileURL, err := url.Parse(mon.oc.Properties.ConsoleProfile.URL)
		if err != nil {
			return err
		}
		consoleProfiledomain := strings.Split(consolProfileURL.Host, ":")[0]
		if !noProxyMap[consolProfileURL.Host] {
			mon.emitAndLogCWPStatus(true, "CWP enabled but missing Console Profile URL "+consoleProfiledomain+" in the no_proxy list")
		}
	}

	return nil
}
