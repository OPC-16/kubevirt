/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2017 Red Hat, Inc.
 *
 */

package tests_test

import (
	"flag"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/kubevirt/pkg/api/v1"
	"kubevirt.io/kubevirt/pkg/kubecli"
	"kubevirt.io/kubevirt/tests"
)

var _ = Describe("Storage", func() {

	nodeName := ""
	nodeIp := ""
	flag.Parse()

	virtClient, err := kubecli.GetKubevirtClient()
	tests.PanicOnError(err)

	BeforeEach(func() {
		tests.BeforeTestCleanup()

		nodes, err := virtClient.CoreV1().Nodes().List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(nodes.Items).ToNot(BeEmpty())
		nodeName = nodes.Items[0].Name
		for _, addr := range nodes.Items[0].Status.Addresses {
			if addr.Type == k8sv1.NodeInternalIP {
				nodeIp = addr.Address
				break
			}
		}
		Expect(nodeIp).ToNot(Equal(""))
	})

	getTargetLogs := func(tailLines int64) string {
		pods, err := virtClient.CoreV1().Pods(metav1.NamespaceSystem).List(metav1.ListOptions{LabelSelector: v1.AppLabel + " in (iscsi-demo-target)"})
		Expect(err).ToNot(HaveOccurred())

		//FIXME Sometimes pods hang in terminating state, select the pod which does not have a deletion timestamp
		podName := ""
		for _, pod := range pods.Items {
			if pod.ObjectMeta.DeletionTimestamp == nil {
				if pod.Status.HostIP == nodeIp {
					podName = pod.ObjectMeta.Name
					break
				}
			}
		}
		Expect(podName).ToNot(BeEmpty())

		logsRaw, err := virtClient.CoreV1().
			Pods(metav1.NamespaceSystem).
			GetLogs(podName,
				&k8sv1.PodLogOptions{TailLines: &tailLines}).
			DoRaw()
		Expect(err).To(BeNil())

		return string(logsRaw)
	}

	RunVMAndExpectLaunch := func(vm *v1.VirtualMachine, withAuth bool) {
		obj, err := virtClient.RestClient().Post().Resource("virtualmachines").Namespace(tests.NamespaceTestDefault).Body(vm).Do().Get()
		Expect(err).To(BeNil())
		tests.WaitForSuccessfulVMStart(obj)
	}

	Context("Given a fresh iSCSI target", func() {
		FIt("should be available and ready", func() {
			logs := getTargetLogs(75)
			Expect(logs).To(ContainSubstring("Target 1: iqn.2017-01.io.kubevirt:sn.42"))
			Expect(logs).To(ContainSubstring("Driver: iscsi"))
			Expect(logs).To(ContainSubstring("State: ready"))
		})
	})

	Context("Given a VM and an Alpine PVC", func() {
		FIt("should be successfully started", func(done Done) {
			// Start the VM with the PVC attached
			vm := tests.NewRandomVMWithPVC(tests.DiskAlpineISCSI)
			vm.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": nodeName}
			RunVMAndExpectLaunch(vm, false)
			close(done)
		}, 60)
	})
})
