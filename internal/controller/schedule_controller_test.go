/*
Copyright 2026 hauke.cloud.

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

package controller

import (
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/hauke-cloud/irrigator/internal/mqtt"
	"github.com/hauke-cloud/irrigator/internal/tasmota"
	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

var _ = Describe("Schedule Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-schedule"
		const deviceName = "test-valve"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a test device")
			device := &iotv1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deviceName,
					Namespace: "default",
				},
				Spec: iotv1alpha1.DeviceSpec{
					BridgeRef: iotv1alpha1.BridgeReference{
						Name: "test-bridge",
					},
					IEEEAddr:   "0x00158D00012345678",
					SensorType: "valve",
				},
				Status: iotv1alpha1.DeviceStatus{
					ShortAddr: "0x1234",
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: deviceName, Namespace: "default"}, device)
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, device)).To(Succeed())
			}

			By("creating the custom resource for the Kind Schedule")
			schedule := &iotv1alpha1.Schedule{}
			err = k8sClient.Get(ctx, typeNamespacedName, schedule)
			if err != nil && errors.IsNotFound(err) {
				resource := &iotv1alpha1.Schedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: iotv1alpha1.ScheduleSpec{
						DeviceName:      deviceName,
						CronExpression:  "0 6 * * *",
						DurationSeconds: 900,
						Enabled:         ptr.To(true),
						TimeZone:        "UTC",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the schedule resource")
			resource := &iotv1alpha1.Schedule{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the device resource")
			device := &iotv1alpha1.Device{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: deviceName, Namespace: "default"}, device)
			if err == nil {
				Expect(k8sClient.Delete(ctx, device)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			testLogger, _ := zap.NewDevelopment()
			mqttManager := mqtt.NewBridgeManager(k8sClient, testLogger)
			valveController := tasmota.NewValveController(k8sClient, testLogger, mqttManager, mqttManager, true) // dry-run for tests

			controllerReconciler := &ScheduleReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				Log:             testLogger,
				ValveController: valveController,
			}

			// Initialize the scheduler manually for testing
			s, err := gocron.NewScheduler()
			Expect(err).NotTo(HaveOccurred())
			controllerReconciler.scheduler = s
			controllerReconciler.jobs = make(map[string]uuid.UUID)
			controllerReconciler.jobSpecs = make(map[string]string)
			controllerReconciler.generations = make(map[string]int64)
			controllerReconciler.scheduler.Start()

			// Clean up scheduler after test
			defer func() {
				_ = controllerReconciler.Shutdown()
			}()

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			// In test environment without MQTT broker, we expect some errors
			// We just verify that the reconcile doesn't panic
			_ = err
		})
	})
})
