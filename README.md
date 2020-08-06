# AWS_Secret_manager_rotation_operator

## Steps to test the controller and CRD 
1. kubectl should be configured to acces EKS cluster on the system where you build the project - https://docs.aws.amazon.com/eks/latest/userguide/install-kubectl.html

2. Install kubebuilder - https://book.kubebuilder.io/quick-start.html#installation

3. Clone the project into go project path - cd go/src && git clone https://github.com/Mahendrasiddappa/AWS_Secret_manager_rotation_operator.git && cd AWS_Secret_manager_rotation_operator

4. Set Environment variables - 
SECRETS_ROTATE_AFTER - Default is 5 seconds, can be configured in seconds
SECRETS_SQS_QUEUE_URL - no default, pass the SQS queue URL not ARN
AWS_DEFAULT_REGION -  Region in which the resources exist

5. Install CRD - make install
Start the controller - make run 

6. 
* Scenario 1:-
Create CRD in default namespace -
kubectl create -f config/samples/seceretreload_v1_sqssecrets.yaml
create deployment named nginx - kubectl run nginx --image=nginx

* Scenario 2:-
CReate CRD in namespace testoperator -
kubectl create ns testoperator && kubectl create -f config/samples/seceretreload_v1_sqssecrets_operator_ns.yaml
there will be no deployment called nginx in this namespace, so controller will try to find deployment name specified in CRD and fails to patch, and moves on

7. Create PutSecretValue event -
aws secretsmanager put-secret-value --secret-id sqssecret --secret-string [{testsqssec:newsecret}]

## Result - 
The nginx pod in default namespace should be restarted
