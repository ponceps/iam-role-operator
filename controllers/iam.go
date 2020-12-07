package controllers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/go-logr/logr"
	iamv1alpha1 "github.com/iclinic/iam-role-operator/api/v1alpha1"
)

var svc = iam.New(awsSession)

// DeleteRole deletes AWS IAM Role
func (r *IamRoleReconciler) DeleteRole(log logr.Logger, iamRole *iamv1alpha1.IamRole) error {
	if _, err := svc.DeleteRole(&iam.DeleteRoleInput{RoleName: aws.String(iamRole.ObjectMeta.Name)}); err != nil {
		log.Error(err, "Error deleting role")
		return err
	}

	log.Info("Role deleted successfully")

	return nil
}

// CreateRole creates AWS IAM Role
func (r *IamRoleReconciler) CreateRole(ctx context.Context, log logr.Logger, iamRole *iamv1alpha1.IamRole) error {
	input := &iam.GetRoleInput{
		RoleName: aws.String(iamRole.Name),
	}

	if _, err := svc.GetRole(input); err != nil {
		log.Info("Creating IAM role on AWS")

		assumeRolePolicyDocument := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
			{
				"Sid": "",
				"Effect": "Allow",
				"Principal": {
				"Federated": "arn:aws:iam::%s:oidc-provider/%s"
				},
				"Action": "sts:AssumeRoleWithWebIdentity",
				"Condition": {
				"StringEquals": {
					"%s:sub": "system:serviceaccount:%s:%s"
				}
				}
			}
			]
		}`, awsAccountID, openIDIssuer, openIDIssuer, iamRole.Namespace, iamRole.Spec.ServiceAccount)

		params := &iam.CreateRoleInput{
			AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument),
			RoleName:                 aws.String(iamRole.Name),
		}

		result, err := svc.CreateRole(params)
		if err != nil {
			log.Error(err, "Error creating role on AWS")
			return err
		}

		log.Info("Role was created", "role", iamRole.Name)

		// Update IamRole status
		iamRole.Status.Arn = *result.Role.Arn
		if err := r.Status().Update(ctx, iamRole); err != nil {
			log.Error(err, "Error updating IamRole status")
			return err
		}
	}

	return nil
}
