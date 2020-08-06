# Introduction
This project helps users to automatically redeploy the pods running on Amazon EKS cluster when the secrets in AWS Secerets Manager is rotated. When the pods are restarted, webhook in this [blog](https://aws.amazon.com/blogs/containers/aws-secrets-controller-poc/) will retrive the latest secret and mount it onto the pods.

## Steps to test the controller and CRD 
1. kubectl should be configured to acces EKS cluster on the system where you build the project - https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html

2. Install kubebuilder - https://book.kubebuilder.io/quick-start.html#installation

3. Create a standard SQS queue

4. Create a AWS Event bridge rule to send PutSecretValue event to SQS queue with following event pattern - 
```
{
  "source": [
    "aws.secretsmanager"
  ],
  "detail-type": [
    "AWS API Call via CloudTrail"
  ],
  "detail": {
    "eventSource": [
      "secretsmanager.amazonaws.com"
    ],
    "eventName": [
      "PutSecretValue"
    ]
  }
}
```

5. Clone the project into go project path -   
```cd go/src && git clone https://github.com/Mahendrasiddappa/AWS_Secret_manager_rotation_operator.git && cd AWS_Secret_manager_rotation_operator```

6. Set Environment variables - 
* SECRETS_ROTATE_AFTER - Default is 5 seconds, can be configured in seconds
* SECRETS_SQS_QUEUE_URL - no default, pass the SQS queue URL not ARN
* AWS_DEFAULT_REGION -  no default, Region in which the resources exist

7. Install CRD -   
```make install```

8. Start the controller -   
```make run ```

9. Create CRD and deployment in multiple namespaces for testing -
* Scenario 1:-
Create CRD in default namespace -
```kubectl create -f config/samples/seceretreload_v1_sqssecrets.yaml```. 
create deployment named nginx -   
```kubectl run nginx --image=nginx```

* Scenario 2:-
CReate CRD in namespace testoperator -  
```kubectl create ns testoperator && kubectl create -f config/samples/seceretreload_v1_sqssecrets_operator_ns.yaml```. 
there will be no deployment called nginx in this namespace, so controller will try to find deployment name specified in CRD and fails to patch, and moves on

10. Create PutSecretValue event -
```aws secretsmanager put-secret-value --secret-id sqssecret --secret-string [{testsqssec:newsecret}]```. 

## Result - 
The nginx pod in default namespace should be restarted
