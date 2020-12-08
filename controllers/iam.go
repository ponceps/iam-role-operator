package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	iamv1alpha1 "github.com/iclinic/iam-role-operator/api/v1alpha1"
)

// PolicyDocument for AWS Role
type PolicyDocument struct {
	Version   string           `json:"Version"`
	Statement InlinePolicyList `json:"Statement"`
}

// InlinePolicy struct
type InlinePolicy struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

// InlinePolicyList struct
type InlinePolicyList []InlinePolicy

var svc = iam.New(awsSession)

// DeleteRole deletes AWS IAM Role
func (r *IamRoleReconciler) DeleteRole(iamRole *iamv1alpha1.IamRole) error {
	log := r.Log.WithValues("role", iamRole.Name)

	// Delete Inline Policies
	if err := deleteAllInlinePolicies(iamRole.Name); err != nil {
		log.Error(err, "Error deleling inline policies")
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

func (r *IamRoleReconciler) updateInlinePolicies(iamRole *iamv1alpha1.IamRole) error {
	log := r.Log.WithValues("role", iamRole.Name).WithValues("inlinePolicy", iamRole.Spec.InlinePolicies)

	log.Info("Updating inline policies")
	// Create or Update policies
	for _, policy := range iamRole.Spec.InlinePolicies {
		policyDocument := &PolicyDocument{
			Version: "2012-10-17",
			Statement: InlinePolicyList{
				InlinePolicy{
					Effect:   policy.Effect,
					Action:   policy.Action,
					Resource: policy.Resource,
				},
			},
		}
		var jsonData []byte
		jsonData, err := json.Marshal(policyDocument)
		if err != nil {
			log.Error(err, "Error Marshalling")
		}

		putRolePolicyInput := &iam.PutRolePolicyInput{
			PolicyDocument: aws.String(string(jsonData)),
			PolicyName:     aws.String(policy.Name),
			RoleName:       aws.String(iamRole.Name),
		}

		if _, err := svc.PutRolePolicy(putRolePolicyInput); err != nil {
			log.Error(err, "Error creating inline policy")
			return err
		}
	}

	log.Info("Deleting unused inline policies")
	// Delete unused policies
	listRolePoliciesInput := &iam.ListRolePoliciesInput{
		RoleName: aws.String(iamRole.Name),
	}
	result, err := svc.ListRolePolicies(listRolePoliciesInput)
	if err != nil {
		log.Error(err, "Failed to retrieve the list of inline policies")
		return err
	}

	var listOfPolicyNames []string
	for _, policy := range iamRole.Spec.InlinePolicies {
		listOfPolicyNames = append(listOfPolicyNames, policy.Name)
	}

	for _, policy := range aws.StringValueSlice(result.PolicyNames) {
		if !contains(listOfPolicyNames, policy) {
			deleteRolePolicyInput := &iam.DeleteRolePolicyInput{
				PolicyName: aws.String(policy),
				RoleName:   aws.String(iamRole.Name),
			}

			if _, err = svc.DeleteRolePolicy(deleteRolePolicyInput); err != nil {
				log.Error(err, "Error deleting on old role policy")
				return err
			}
		}
	}

	return nil
}

func deleteAllInlinePolicies(roleName string) error {
	inlinePolicies, err := svc.ListRolePolicies(&iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return err
	}

	for _, inlinePolicy := range aws.StringValueSlice(inlinePolicies.PolicyNames) {
		_, err = svc.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
			PolicyName: aws.String(inlinePolicy),
			RoleName:   aws.String(roleName),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
