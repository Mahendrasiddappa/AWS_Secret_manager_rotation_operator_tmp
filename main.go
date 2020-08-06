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
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"time"

	seceretreloadv1 "example/api/v1"
	"example/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = seceretreloadv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var RequeueAfter time.Duration

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
		LeaderElectionID:   "8bf23ea1.secretsreload.test",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	//Read the requestAfter vaule from environment variable
	if secret_rotate_after := os.Getenv("SECRETS_ROTATE_AFTER"); secret_rotate_after == "" {
		setupLog.Info("setting default reconciler request after time")
		RequeueAfter = time.Duration(5)
	} else {
		setupLog.Info("setting user specified reconciler request after time")
		temp, _ := strconv.Atoi(secret_rotate_after)
		RequeueAfter = time.Duration(temp)
	}

	//Read the SQS queue name vaule from environment variable
	secret_sqs_queue := os.Getenv("SECRETS_SQS_QUEUE_URL")
	if secret_sqs_queue == "" {
		setupLog.Error(err, "please set the SQS queue URL in environment variable SECRETS_SQS_QUEUE_URL")
		os.Exit(1)
	}

	//Read region vaule from environment variable
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		setupLog.Error(err, "please set region in environment variable AWS_DEFAULT_REGION")
		os.Exit(1)
	}

	fmt.Println("secret_sqs_queue:", secret_sqs_queue)
	if err = (&controllers.SQSsecretsReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("SQSsecrets"),
		Scheme:       mgr.GetScheme(),
		RequeueAfter: RequeueAfter,
		QueueUrl:     secret_sqs_queue,
		Region:       region,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SQSsecrets")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
