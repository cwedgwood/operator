// Copyright (c) 2019 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package installation

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	operator "github.com/tigera/operator/pkg/apis/operator/v1"
)

var _ = Describe("Installation validation tests", func() {
	var instance *operator.Installation

	BeforeEach(func() {
		instance = &operator.Installation{
			Spec: operator.InstallationSpec{
				CalicoNetwork:  &operator.CalicoNetworkSpec{},
				FlexVolumePath: "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/",
				NodeUpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
				},
				Variant: operator.Calico,
				CNI:     &operator.CNISpec{Type: operator.PluginCalico},
			},
		}
	})

	It("should not allow blocksize to exceed the pool size", func() {
		// Try with an invalid block size.
		var twentySix int32 = 26
		var enabled operator.BGPOption = operator.BGPEnabled
		instance.Spec.CalicoNetwork.BGP = &enabled
		instance.Spec.CalicoNetwork.IPPools = []operator.IPPool{
			{
				CIDR:          "192.168.0.0/27",
				BlockSize:     &twentySix,
				Encapsulation: operator.EncapsulationNone,
				NATOutgoing:   operator.NATOutgoingEnabled,
				NodeSelector:  "all()",
			},
		}
		err := validateCustomResource(instance)
		Expect(err).To(HaveOccurred())

		// Try with a valid block size
		instance.Spec.CalicoNetwork.IPPools[0].CIDR = "192.168.0.0/26"
		err = validateCustomResource(instance)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should prevent IPIP if BGP is disabled", func() {
		disabled := operator.BGPDisabled
		instance.Spec.CalicoNetwork.BGP = &disabled
		instance.Spec.CalicoNetwork.IPPools = []operator.IPPool{
			{
				CIDR:          "192.168.0.0/24",
				Encapsulation: operator.EncapsulationIPIP,
				NATOutgoing:   operator.NATOutgoingEnabled,
				NodeSelector:  "all()",
			},
		}
		err := validateCustomResource(instance)
		Expect(err).To(HaveOccurred())
	})

	It("should prevent IPIP cross-subnet if BGP is disabled", func() {
		disabled := operator.BGPDisabled
		instance.Spec.CalicoNetwork.BGP = &disabled
		instance.Spec.CalicoNetwork.IPPools = []operator.IPPool{
			{
				CIDR:          "192.168.0.0/24",
				Encapsulation: operator.EncapsulationIPIPCrossSubnet,
				NATOutgoing:   operator.NATOutgoingEnabled,
				NodeSelector:  "all()",
			},
		}
		err := validateCustomResource(instance)
		Expect(err).To(HaveOccurred())
	})

	It("should not error if CalicoNetwork is provided on EKS", func() {
		instance := &operator.Installation{}
		instance.Spec.CNI = &operator.CNISpec{Type: operator.PluginCalico}
		instance.Spec.Variant = operator.TigeraSecureEnterprise
		instance.Spec.CalicoNetwork = &operator.CalicoNetworkSpec{}
		instance.Spec.KubernetesProvider = operator.ProviderEKS

		// Fill in defaults and validate the result.
		Expect(fillDefaults(instance)).NotTo(HaveOccurred())
		Expect(validateCustomResource(instance)).NotTo(HaveOccurred())
	})

	It("should not allow out-of-bounds block sizes", func() {
		// Try with an invalid block size.
		var blockSizeTooBig int32 = 33
		var blockSizeTooSmall int32 = 19
		var blockSizeJustRight int32 = 32

		// Start with a valid block size - /32 - just on the border.
		var enabled operator.BGPOption = operator.BGPEnabled
		instance.Spec.CalicoNetwork.BGP = &enabled
		instance.Spec.CalicoNetwork.IPPools = []operator.IPPool{
			{
				CIDR:          "192.0.0.0/8",
				BlockSize:     &blockSizeJustRight,
				Encapsulation: operator.EncapsulationNone,
				NATOutgoing:   operator.NATOutgoingEnabled,
				NodeSelector:  "all()",
			},
		}
		err := validateCustomResource(instance)
		Expect(err).NotTo(HaveOccurred())

		// Try with out-of-bounds sizes now.
		instance.Spec.CalicoNetwork.IPPools[0].BlockSize = &blockSizeTooBig
		err = validateCustomResource(instance)
		Expect(err).To(HaveOccurred())
		instance.Spec.CalicoNetwork.IPPools[0].BlockSize = &blockSizeTooSmall
		err = validateCustomResource(instance)
		Expect(err).To(HaveOccurred())
	})

	It("should not allow a relative path in FlexVolumePath", func() {
		instance.Spec.FlexVolumePath = "foo/bar/baz"
		err := validateCustomResource(instance)
		Expect(err).To(HaveOccurred())
	})

	It("should validate HostPorts", func() {
		instance.Spec.CalicoNetwork.HostPorts = nil
		err := validateCustomResource(instance)
		Expect(err).NotTo(HaveOccurred())

		hp := operator.HostPortsEnabled
		instance.Spec.CalicoNetwork.HostPorts = &hp
		err = validateCustomResource(instance)
		Expect(err).NotTo(HaveOccurred())

		hp = operator.HostPortsDisabled
		instance.Spec.CalicoNetwork.HostPorts = &hp
		err = validateCustomResource(instance)
		Expect(err).NotTo(HaveOccurred())

		hp = "NotValid"
		instance.Spec.CalicoNetwork.HostPorts = &hp
		err = validateCustomResource(instance)
		Expect(err).To(HaveOccurred())
	})

	Describe("CalicoNetwork requires spec.cni.type=Calico", func() {
		DescribeTable("non-calico plugins", func(t operator.CNIPluginType) {
			instance.Spec.CNI.Type = t
			err := validateCustomResource(instance)
			Expect(err).To(HaveOccurred())
		},
			Entry("should disallow GKE", operator.PluginGKE),
			Entry("should disallow AmazonVPC", operator.PluginAmazonVPC),
			Entry("should disallow AzureVNET", operator.PluginAzureVNET),
		)
	})
	Describe("validate non-calico CNI plugin Type", func() {
		BeforeEach(func() {
			instance.Spec.CalicoNetwork = nil
			instance.Spec.CNI = &operator.CNISpec{}
		})
		It("should not allow empty CNI", func() {
			err := validateCustomResource(instance)
			Expect(err).To(HaveOccurred())
		})
		It("should not allow invalid CNI Type", func() {
			instance.Spec.CNI.Type = "bad"
			err := validateCustomResource(instance)
			Expect(err).To(HaveOccurred())
		})
		DescribeTable("test all plugins",
			func(plugin operator.CNIPluginType) {
				instance.Spec.CNI.Type = plugin
				err := validateCustomResource(instance)
				Expect(err).NotTo(HaveOccurred())
			},

			Entry("GKE", operator.PluginGKE),
			Entry("AmazonVPC", operator.PluginAmazonVPC),
			Entry("AzureVNET", operator.PluginAzureVNET),
		)
	})
	Describe("cross validate ExternallyManagedNetwork.Plugin and kubernetesProvider", func() {
		BeforeEach(func() {
			instance.Spec.CalicoNetwork = nil
			instance.Spec.CNI = &operator.CNISpec{}
		})
		DescribeTable("test all plugins",
			func(kubeProvider operator.Provider, plugin operator.CNIPluginType, success bool) {
				instance.Spec.KubernetesProvider = kubeProvider
				instance.Spec.CNI.Type = plugin
				err := validateCustomResource(instance)
				if success {
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},

			Entry("GKE plugin is not allowed on EKS", operator.ProviderEKS, operator.PluginGKE, false),
			Entry("AmazonVPC plugin is allowed on EKS", operator.ProviderEKS, operator.PluginAmazonVPC, true),
			Entry("AzureVNET plugin is not allowed on EKS", operator.ProviderEKS, operator.PluginAzureVNET, false),
		)
	})
})
