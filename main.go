/*


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

package main

import (
	"context"
	"flag"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	apiv1 "github.com/jeremyary/observability-operator/api/v1"
	"github.com/jeremyary/observability-operator/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apiv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "04220e3f.redhat.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ObservabilityReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Observability"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Observability")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	// TODO: creating Observability CR here for now
	o := &apiv1.Observability{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "observability.redhat.com/v1",
			Kind:       "Observability",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-services-observability",
			Namespace: "openshift-monitoring",
		},
	}
	err = mgr.GetClient().Create(context.Background(), o)
	if err != nil {
		setupLog.Error(err, "Error creating Observablity CR")
		os.Exit(1)
	}

	//TODO: injecting handler to auto-delete our CR when stopping locally running operator for early dev
	//if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
	if err := injectStopHandler(mgr, o, setupLog); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func injectStopHandler(mgr ctrl.Manager, o *apiv1.Observability, setupLog logr.Logger) error {
	defer func() {
		setupLog.Info("SIGINT/KILL received, deleting Observability CR")
		_ = mgr.GetClient().Delete(context.Background(), o)
	}()
	err := mgr.Start(ctrl.SetupSignalHandler())
	return err
}
