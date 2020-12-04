package controllers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	iamv1alpha1 "github.com/iclinic/iam-role-operator/api/v1alpha1"
	"github.com/prometheus/common/log"
)

// DeleteRole deletes AWS IAM Role
func DeleteRole(ctx context.Context, roleName string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION"))},
	))

	svc := iam.New(sess)

	// Delete role
	_, err := svc.DeleteRole(&iam.DeleteRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		log.Error(err, "Error deleting role")
		return err
	}

	log.Info("Role delete successfully")

	return nil
}

// CreateRole creates AWS IAM Role
func CreateRole(ctx context.Context, iamRole *iamv1alpha1.IamRole) error {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION"))},
	))

	svc := iam.New(sess)

	input := &iam.GetRoleInput{
		RoleName: aws.String(iamRole.Name),
	}

	_, err := svc.GetRole(input)
	if err != nil {
		log.Info("Creating AWS Role")

		awsAccountID := os.Getenv("AWS_ACCOUNT_ID")
		openIDIssuer := strings.TrimPrefix(os.Getenv("OPENID_ISSUER_URL"), "https://")

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
		}`, awsAccountID, openIDIssuer, openIDIssuer, iamRole.Namespace, "*")

		params := &iam.CreateRoleInput{
			AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument),
			RoleName:                 aws.String(iamRole.Name),
		}

		_, err = svc.CreateRole(params)
		if err != nil {
			log.Error(err, "Error creating role on AWS")
			return err
		}

		log.Info("Role was created", "role", iamRole.Name)
	}

	return nil
}
