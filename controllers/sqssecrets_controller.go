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
	QueueUrl     string
	Region       string
}

type messageBody struct {
	EventName         string `json:"eventName"`
	RequestParameters string `json:"requestParameters"`
}

// +kubebuilder:rbac:groups=seceretreload.secretsreload.test,resources=sqssecrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=seceretreload.secretsreload.test,resources=sqssecrets/status,verbs=get;update;patch

func (r *SQSsecretsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	//log := r.Log.WithValues("sqssecrets", req.NamespacedName, "at", req.Name)

	var DeleteMessageBatchList []*sqs.DeleteMessageBatchRequestEntry
	var SQSSecret seceretreloadv1.SQSsecrets
	var result map[string]interface{}

	if err := r.Get(ctx, req.NamespacedName, &SQSSecret); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	fmt.Println("NamespacedName", req.NamespacedName)
	fmt.Println("===========================")
	fmt.Println("===========================")
	fmt.Println("Reconciler started")
	fmt.Println("===========================")
	fmt.Println("===========================")

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(r.Region)},
	)
	svc := sqs.New(sess)

	//log.Info("created SQS session")

	//read message from SQS
	message, err := svc.ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl:            &r.QueueUrl,
		MaxNumberOfMessages: aws.Int64(10),
		VisibilityTimeout:   aws.Int64(20),
		WaitTimeSeconds:     aws.Int64(0),
	})

	if err != nil {
		fmt.Println("Error", err)
		return ctrl.Result{RequeueAfter: time.Second * r.RequeueAfter}, nil
	}

	fmt.Println("SQS messages:", message.Messages)
	//loop through all the messages retrived from SQS
	for _, element := range message.Messages {
		fmt.Println("===========================")
		fmt.Println("===========================")
		fmt.Println("loop started")
		fmt.Println("===========================")
		fmt.Println("===========================")
		err := json.Unmarshal([]byte(*element.Body), &result)
		if err != nil {
			fmt.Println("Error", err)
		}

		detail := result["detail"].(map[string]interface{})
		eventName := detail["eventName"]
		fmt.Println("SQS Event Name", eventName)
		//requestParameters := detail["requestParameters"].(map[string]interface{})
		//requestParameters := detail["additionalEventData"].(map[string]interface{})
		//secretID := requestParameters["secretId"]

		// continue only if the event type is PutSecretValue
		if eventName == "PutSecretValue" {
			requestParameters := detail["requestParameters"].(map[string]interface{})
			secretID := requestParameters["secretId"]
			fmt.Println("Secret ID rotated", secretID)

			//read the CRD SQSsecrets to get the secert name to deployment mapping

			//fmt.Println("fetched SQS CRD:", SQSSecret)
			//fmt.Println("SQS CRD DeploymentNames:", SQSSecret.Spec.DeploymentNames)
			fmt.Println("SQS CRD secret ID:", SQSSecret.Spec.SecretID, secretID)
			//if the secretID in SQS message is not same as the secret in CRD, continue with next message
			if secretID != SQSSecret.Spec.SecretID {
				fmt.Println("continuing to next loop")
				continue
			}

			//get the deployment specified in the crd SQSsecrets
			var deploy v1.Deployment
			DeploymentNames := SQSSecret.Spec.DeploymentNames
			for _, deployment := range DeploymentNames {
				deployName := client.ObjectKey{Name: deployment, Namespace: req.Namespace}
				if err := r.Get(ctx, deployName, &deploy); err != nil {
					fmt.Println("Get deployment err:", err)
					return ctrl.Result{RequeueAfter: time.Second * r.RequeueAfter}, nil
				}

				// Patch the deployment with new label containing redeployed timestamp, to force redeploy
				patch := []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"labels":{"aws-secrets-controller-redeloyed":"%v"}}}}}`, time.Now().Unix()))
				if err := r.Patch(ctx, &deploy, client.RawPatch(types.StrategicMergePatchType, patch)); err != nil {
					fmt.Println("Patch deployment err:", err)
					return ctrl.Result{RequeueAfter: time.Second * r.RequeueAfter}, nil
				}
			}
		}

		// Add SQS message to delete message queue
		//s := string(num)
		//fmt.Println("Iteration number:", num)
		deleteMessage := sqs.DeleteMessageBatchRequestEntry{Id: element.MessageId, ReceiptHandle: element.ReceiptHandle}
		DeleteMessageBatchList = append(DeleteMessageBatchList, &deleteMessage)
	}

	//DeleteMessageBatch
	fmt.Println("DeleteMessageBatchList:", DeleteMessageBatchList)
	DeleteMessageBatchInput := &sqs.DeleteMessageBatchInput{Entries: DeleteMessageBatchList, QueueUrl: &r.QueueUrl}
	DeleteMessageBatchOutput, err := svc.DeleteMessageBatch(DeleteMessageBatchInput)
	if err != nil {
		fmt.Println("DeleteMessageBatchList error:", err)
	}
	fmt.Println("DeleteMessageBatchList output:", DeleteMessageBatchOutput)
	return ctrl.Result{RequeueAfter: time.Second * r.RequeueAfter}, nil
}

func (r *SQSsecretsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&seceretreloadv1.SQSsecrets{}).
		Complete(r)
}
