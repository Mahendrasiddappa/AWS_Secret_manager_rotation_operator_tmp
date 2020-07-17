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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	"encoding/json"
	seceretreloadv1 "example/api/v1"
)

// SQSsecretsReconciler reconciles a SQSsecrets object
type SQSsecretsReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	RequeueAfter time.Duration
}

type messageBody struct {
	EventName         string `json:"eventName"`
	RequestParameters string `json:"requestParameters"`
}

// +kubebuilder:rbac:groups=seceretreload.secretsreload.test,resources=sqssecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=seceretreload.secretsreload.test,resources=sqssecrets/status,verbs=get;update;patch

func (r *SQSsecretsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("sqssecrets", req.NamespacedName, "at", req.Name)

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	svc := sqs.New(sess)

	log.Info("created SQS session")
	result, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: aws.String("casc"),
	})

	log.Info("got SQS queue")
	if err != nil {
		log.Info("SQS error")
		log.WithValues("Error", err)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
	log.Info("got SQS queue with no error")
	fmt.Println("Success", *result.QueueUrl)

	//read message from SQS
	message, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
		AttributeNames: []*string{
			aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		QueueUrl:            result.QueueUrl,
		MaxNumberOfMessages: aws.Int64(1),
		VisibilityTimeout:   aws.Int64(20), // 20 seconds
		WaitTimeSeconds:     aws.Int64(0),
	})

	if err != nil {
		fmt.Println("Error", err)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	//loop through all the messages retrived from SQS
	for _, element := range message.Messages {
		//fmt.Println("SQS message:", *element.Body)
		var result map[string]interface{}

		// Unmarshall SQS message from string JSON to map
		err := json.Unmarshal([]byte(*element.Body), &result)

		if err != nil {
			fmt.Println("Error", err)
		}

		detail := result["detail"].(map[string]interface{})
		requestParameters := detail["requestParameters"].(map[string]interface{})
		eventName := detail["eventName"]
		secretID := requestParameters["secretId"]

		fmt.Println("SQS Event Name", eventName)
		fmt.Println("SQS secret ID", secretID)
		// continue inly if the event type is PutSecretValue
		if eventName == "PutSecretValue" {
			//read the CRD SQSsecrets to get the secert name to deployment mapping
			var SQSSecret seceretreloadv1.SQSsecrets
			if err := r.Get(ctx, req.NamespacedName, &SQSSecret); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			fmt.Println("fetched SQS CRD:", SQSSecret)
			fmt.Println("SQS CRD DeploymentNames:", SQSSecret.Spec.DeploymentNames)
			fmt.Println("SQS CRD secret ID:", SQSSecret.Spec.SecretID)

			//get the deployment specified in the crd SQSsecrets
			var deploy v1.Deployment
			DeploymentNames := SQSSecret.Spec.DeploymentNames
			for _, deployment := range DeploymentNames {
				deployName := client.ObjectKey{Name: deployment, Namespace: req.Namespace}
				if err := r.Get(ctx, deployName, &deploy); err != nil {
					fmt.Println("Get deployment err:", err)
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

				// Patch the deployment with new label containing redeployed timestamp, to force redeploy
				patch := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"labels":{"aws-secrets-controller-redeloyed":"%v"}}}}}`, time.Now().Unix()))
				if err := r.Patch(ctx, &deploy, client.RawPatch(types.StrategicMergePatchType, patch)); err != nil {
					fmt.Println("Patch deployment err:", err)
				}
			}
		}
	}

	return ctrl.Result{RequeueAfter: time.Second * r.RequeueAfter}, nil
}

func (r *SQSsecretsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&seceretreloadv1.SQSsecrets{}).
		Complete(r)
}
