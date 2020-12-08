package controllers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	iamv1alpha1 "github.com/iclinic/iam-role-operator/api/v1alpha1"
)

var svc = iam.New(awsSession)

// DeleteRole deletes AWS IAM Role
func (r *IamRoleReconciler) DeleteRole(iamRole *iamv1alpha1.IamRole) error {
	log := r.Log.WithValues("role", iamRole.Name)

	// Delete Managed Policies
	if err := deleteAllManagedPolicies(iamRole.Name); err != nil {
		log.Error(err, "Error deleling managed policies")
	}

	// Delete role
	if _, err := svc.DeleteRole(&iam.DeleteRoleInput{RoleName: aws.String(iamRole.ObjectMeta.Name)}); err != nil {
		log.Error(err, "Error deleting role")
		return err
	}

	log.Info("Role deleted successfully")

	return nil
}

// CreateRole creates AWS IAM Role
func (r *IamRoleReconciler) CreateRole(ctx context.Context, iamRole *iamv1alpha1.IamRole) error {
	log := r.Log.WithValues("role", iamRole.Name)

	input := &iam.GetRoleInput{
		RoleName: aws.String(iamRole.Name),
	}

	if _, err := svc.GetRole(input); err != nil {
		log.Info("Creating IAM role")

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
			log.Error(err, "Error creating role")
			return err
		}

		log.Info("Role was created successfully")

		// Update IamRole status
		iamRole.Status.Arn = *result.Role.Arn
		if err := r.Status().Update(ctx, iamRole); err != nil {
			log.Error(err, "Error updating IamRole status")
			return err
		}
	}

	return nil
}

func (r *IamRoleReconciler) updateManagedPolicies(iamRole *iamv1alpha1.IamRole) error {
	log := r.Log.WithValues("role", iamRole.Name).WithValues("managedPolicy", iamRole.Spec.ManagedPolicies)

	log.Info("Updating managed policies")
	// Create or Update policies
	for _, v := range iamRole.Spec.ManagedPolicies {
		attachRolePolicyInput := &iam.AttachRolePolicyInput{
			PolicyArn: aws.String(v),
			RoleName:  aws.String(iamRole.Name),
		}

		if _, err := svc.AttachRolePolicy(attachRolePolicyInput); err != nil {
			log.Error(err, "Error attaching managed policy")
			return err
		}
	}

	log.Info("Deleting unused managed policies")
	// Delete unused policies
	listAttachedRolePoliciesInput := &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(iamRole.Name),
	}
	result, err := svc.ListAttachedRolePolicies(listAttachedRolePoliciesInput)
	if err != nil {
		log.Error(err, "Failed to retrieve the list of managed policies")
		return err
	}

	for _, v := range result.AttachedPolicies {
		if !contains(iamRole.Spec.ManagedPolicies, *v.PolicyArn) {
			detachRolePolicyInput := &iam.DetachRolePolicyInput{
				PolicyArn: aws.String(*v.PolicyArn),
				RoleName:  aws.String(iamRole.Name),
			}

			if _, err = svc.DetachRolePolicy(detachRolePolicyInput); err != nil {
				log.Error(err, "Error deleting an old role policy")
				return err
			}
		}
	}

	return nil
}

func deleteAllManagedPolicies(roleName string) error {
	managedPolicies, err := svc.ListAttachedRolePolicies(&iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return err
	}

	for _, managedPolicy := range managedPolicies.AttachedPolicies {
		_, err := svc.DetachRolePolicy(&iam.DetachRolePolicyInput{
			PolicyArn: managedPolicy.PolicyArn,
			RoleName:  aws.String(roleName),
		})
		if err != nil {
			return err
		}
	}

	return nil
}
